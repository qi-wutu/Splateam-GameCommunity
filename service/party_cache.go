package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"splatoon-backend/config"
	"splatoon-backend/models"

	"github.com/redis/go-redis/v9"
)

// ---------- 组队列表：Redis 索引缓存 + keyset 分页 ----------
//
// 设计要点：列表页是最「高频 + 重复」的读操作，但直接 ORDER BY created_at DESC 会越来越大。
// 这里用 Redis 的一个 ZSET 缓存「所有 party id 按创建时间排序」的索引：
//   - member = party ID（字符串）
//   - score  = created_at 的 Unix 毫秒时间戳
//
// 分页用 ZREVRANGEBYSCORE（score 即时间，天然就是游标），再对命中的 id 用 WHERE id IN (...) 批量回表，
// 这样每次请求只做 O(logN) 的区间扫描 + N 行回表，不再全表排序。
//
// ⚠️ 只缓存「id + 时间」这个稳定排序键，不缓存整行数据。playernum 等易变字段每次回表取最新，
// 所以 join / leave 不需要失效索引（只有 create 需要 ZADD、delete 需要 ZREM）。
// 索引带随机化短 TTL，过期后由下一次读取重建（带进程内锁防穿透）。

const (
	partyListIndexKey    = "party:list:ids" // ZSET：id -> created_at 排序
	partyListIndexTTL    = 45 * time.Second // 基础 TTL
	partyListIndexTTLJ   = 15 * time.Second // TTL 随机抖动
	partyListIndexSlack  = 8                // 游标页多取的候选数（用于同毫秒 tie + 清理已删除 id）
	PartyListPageSize    = 10               // 默认每页
	PartyListMaxPageSize = 50               // 每页上限
)

// PartyListPage 是 /api/party 的分页返回体。
type PartyListPage struct {
	Items      []models.Party `json:"items"`      // 当前页
	NextCursor string         `json:"nextCursor"` // 下一页游标（空串表示已到底）
	HasMore    bool           `json:"hasMore"`    // 是否还有更多
}

// partyCursor 游标：上一页最后一条的 (created_at, id)，用于 keyset 分页。
// id==0 表示「没有游标即第一页」。
type partyCursor struct {
	CreatedAt time.Time
	ID        uint
}

func (c *partyCursor) isEmpty() bool { return c == nil || c.ID == 0 }

// partyListMu 进程内互斥锁：缓存穿透（索引已过期且并发打到）时，只有一个 goroutine 重建。
var partyListMu sync.Mutex

// partyListIndexReady 返回索引是否存在。不存在且 Redis 可用时，在锁内重建一次。
func partyListIndexReady() bool {
	if config.RedisClient == nil {
		return false
	}
	exists, err := config.RedisClient.Exists(config.RedisCtx, partyListIndexKey).Result()
	if err != nil || exists == 1 {
		return exists == 1
	}

	// 双检锁重建，避免并发击穿
	partyListMu.Lock()
	defer partyListMu.Unlock()
	if exists, err := config.RedisClient.Exists(config.RedisCtx, partyListIndexKey).Result(); err == nil && exists == 1 {
		return true
	}
	if err := rebuildPartyListIndex(); err != nil {
		return false
	}
	return true
}

// rebuildPartyListIndex 从 DB 全量重建索引：SELECT id, created_at → 写进 ZSET → 设随机 TTL。
func rebuildPartyListIndex() error {
	var rows []struct {
		ID        uint
		CreatedAt time.Time
	}
	if err := config.Db.Model(&models.Party{}).Select("id, created_at").Scan(&rows).Error; err != nil {
		return err
	}

	pipe := config.RedisClient.Pipeline()
	for _, r := range rows {
		pipe.ZAdd(config.RedisCtx, partyListIndexKey, redis.Z{
			Score:  float64(r.CreatedAt.UnixMilli()),
			Member: strconv.FormatUint(uint64(r.ID), 10),
		})
	}
	if _, err := pipe.Exec(config.RedisCtx); err != nil {
		return err
	}

	// 随机化 TTL，把「一批请求同时过期」摊开（防雪崩）；下次过期后重建也带上新数据
	ttl := partyListIndexTTL + time.Duration(rand.Int63n(int64(partyListIndexTTLJ)))
	return config.RedisClient.Expire(config.RedisCtx, partyListIndexKey, ttl).Err()
}

// partyListIndexZAdd 新建 party 时增量入索引。仅当索引处于「已就绪」状态才更新，
// 否则交给读取时的重建兜底（避免在过期/从未建过的 key 上造出只含一个成员的残缺索引）。
func partyListIndexZAdd(id uint, createdAt time.Time) {
	if config.RedisClient == nil {
		return
	}
	exists, err := config.RedisClient.Exists(config.RedisCtx, partyListIndexKey).Result()
	if err != nil || exists == 0 {
		return
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	config.RedisClient.ZAdd(config.RedisCtx, partyListIndexKey, redis.Z{
		Score:  float64(createdAt.UnixMilli()),
		Member: strconv.FormatUint(uint64(id), 10),
	})
}

// partyListIndexZRem 删除 party 时从索引移除。ZADD/ZREM 都是幂等增量，比整键失效便宜得多。
func partyListIndexZRem(id uint) {
	if config.RedisClient == nil {
		return
	}
	config.RedisClient.ZRem(config.RedisCtx, partyListIndexKey, strconv.FormatUint(uint64(id), 10))
}

// seekPartyIDs 在索引里取「严格早于游标」的最多 n 个 party ID（按 createdAt 新→旧）。
// 同一毫秒的 tie 用 id 打破（created_at 大的在前；相同时 id 大的在前）。
func seekPartyIDs(cur *partyCursor, n int) []uint {
	if config.RedisClient == nil {
		return nil
	}
	ids := make([]uint, 0, n)

	max := "+inf"
	if !cur.isEmpty() {
		max = strconv.FormatInt(cur.CreatedAt.UnixMilli(), 10)
	}
	// request 稍多个候选，供「同毫秒 tie 筛选 + 清理已删除 id」后仍够一页
	zs, err := config.RedisClient.ZRevRangeByScoreWithScores(config.RedisCtx, partyListIndexKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    max,
		Offset: 0,
		Count:  int64(n + partyListIndexSlack),
	}).Result()
	if err != nil {
		return ids
	}

	curMs := int64(0)
	if !cur.isEmpty() {
		curMs = cur.CreatedAt.UnixMilli()
	}
	for _, z := range zs {
		id, err := strconv.ParseUint(z.Member.(string), 10, 64)
		if err != nil {
			continue
		}
		// kept: score < curMs，或在同毫秒内 id 更小（严格更旧）
		kept := cur.isEmpty() || int64(z.Score) < curMs || (int64(z.Score) == curMs && id < uint64(cur.ID))
		if !kept {
			continue
		}
		ids = append(ids, uint(id))
		if len(ids) >= n {
			break
		}
	}
	return ids
}

// fetchPartyRows 用 id 批量回表，返回 id->Party。查不到（已删）的 id 不返回。
func fetchPartyRows(ids []uint) (map[uint]models.Party, error) {
	res := make(map[uint]models.Party, len(ids))
	if len(ids) == 0 {
		return res, nil
	}
	var rows []models.Party
	if err := config.Db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		res[r.ID] = r
	}
	return res, nil
}

// encodePartyCursor 生成不透明游标串（URL 安全 base64），前端无需解析，原样回传即可。
func encodePartyCursor(createdAt time.Time, id uint) string {
	payload := fmt.Sprintf("%s|%d", createdAt.Format(time.RFC3339Nano), id)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// parsePartyCursor 解析游标串；空串表示第一页（返回 id==0 的空游标）。
func parsePartyCursor(cursor string) (*partyCursor, error) {
	if cursor == "" {
		return &partyCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, errors.New("invalid cursor")
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, errors.New("invalid cursor")
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id == 0 {
		return nil, errors.New("invalid cursor")
	}
	return &partyCursor{CreatedAt: createdAt, ID: uint(id)}, nil
}

// orderPartyRows 按 (created_at desc, id desc) 排序，保证同一毫秒内也稳定有序。
func orderPartyRows(rows map[uint]models.Party, ids []uint) []models.Party {
	// 先只保留确实能取到的行
	got := make([]models.Party, 0, len(ids))
	for _, id := range ids {
		if p, ok := rows[id]; ok {
			got = append(got, p)
		}
	}
	sort.SliceStable(got, func(i, j int) bool {
		if got[i].CreatedAt.Equal(got[j].CreatedAt) {
			return got[i].ID > got[j].ID
		}
		return got[i].CreatedAt.After(got[j].CreatedAt)
	})
	return got
}
