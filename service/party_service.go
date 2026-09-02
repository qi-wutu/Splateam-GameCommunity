package service

import (
	"errors"
	"splatoon-backend/config"
	"splatoon-backend/models"
	"splatoon-backend/utils"
	"strconv"

	"gorm.io/gorm"
)

type CreatePartyReq struct {
	Title        string `json:"title"`
	Game         string `json:"game"`
	Introduction string `json:"introduction"`
	Playernum    int    `json:"playernum"`
	MaxNum       int    `json:"maxNum"`
}

type MemberInfo struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	Gender   string `json:"gender"`
	Status   string `json:"status"`
}

type PartyDetail struct {
	models.Party
	Members []MemberInfo `json:"members"`
}

func CreateParty(userID string, req *CreatePartyReq) (*models.Party, error) {
	// 查 DB 拿用户名
	var user models.User
	if err := config.Db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}

	party := &models.Party{
		Title:        req.Title,
		Game:         req.Game,
		Introduction: req.Introduction,
		Playernum:    1,
		MaxNum:       req.MaxNum,
		OwnerID:      userID,
		OwnerName:    user.UserName,
	}

	if err := config.Db.Create(party).Error; err != nil {
		return nil, err
	}

	// 新建 party 后把 id 增量写入列表索引（索引未就绪时由读取端重建兜底）
	partyListIndexZAdd(party.ID, party.CreatedAt)

	// 创建者自动加入成员表
	member := &models.PartyMember{
		PartyID: party.ID,
		UserID:  userID,
		Status:  "JOINED",
	}
	if err := config.Db.Create(member).Error; err != nil {
		return nil, err
	}

	return party, nil
}

// GetPartyList 分页返回组队列表（created_at desc, id desc）。
// 游标 + Redis ZSET 索引：先取「严格早于游标」的一页 id，再回表取整行，避免全表 ORDER BY。
// cursor 为空表示第一页；limit 缺省用 PartyListPageSize，上限 PartyListMaxPageSize。
func GetPartyList(cursor string, limit int) (*PartyListPage, error) {
	if limit <= 0 {
		limit = PartyListPageSize
	}
	if limit > PartyListMaxPageSize {
		limit = PartyListMaxPageSize
	}
	cur, err := parsePartyCursor(cursor)
	if err != nil {
		return nil, err
	}

	// 索引未就绪（Redis 不可用 / 尚未建立）时降级：直接按 DB 全量分页，逻辑仍正确。
	if !partyListIndexReady() {
		return getPartyListFromDB(cur, limit)
	}

	// 多取 1 个用于判断 isMore，再裁成 limit 个
	cand := seekPartyIDs(cur, limit+1)
	hasMore := len(cand) > limit
	if hasMore {
		cand = cand[:limit]
	}

	rows, err := fetchPartyRows(cand)
	if err != nil {
		return nil, err
	}
	// 索引里可能残留已删除（软删）的 id：回表查不到 → 顺手清理索引
	for _, id := range cand {
		if _, ok := rows[id]; !ok {
			partyListIndexZRem(id)
		}
	}
	parties := orderPartyRows(rows, cand)

	next := ""
	if len(parties) > 0 {
		last := parties[len(parties)-1]
		next = encodePartyCursor(last.CreatedAt, last.ID)
	}
	return &PartyListPage{Items: parties, NextCursor: next, HasMore: hasMore}, nil
}

// getPartyListFromDB 降级路径：Redis 不可用 / 索引缺失时直接查 DB。
// 用 join 条件实现 keyset（created_at, id 双条件），语义与 Redis 路径保持一致。
func getPartyListFromDB(cur *partyCursor, limit int) (*PartyListPage, error) {
	q := config.Db.Model(&models.Party{}).Order("created_at desc, id desc")
	if !cur.isEmpty() {
		q = q.Where("(created_at < ?) OR (created_at = ? AND id < ?)",
			cur.CreatedAt, cur.CreatedAt, cur.ID)
	}
	var parties []models.Party
	if err := q.Limit(limit + 1).Find(&parties).Error; err != nil {
		return nil, err
	}
	hasMore := len(parties) > limit
	if hasMore {
		parties = parties[:limit]
	}
	next := ""
	if len(parties) > 0 {
		last := parties[len(parties)-1]
		next = encodePartyCursor(last.CreatedAt, last.ID)
	}
	return &PartyListPage{Items: parties, NextCursor: next, HasMore: hasMore}, nil
}

func GetPartyByID(partyID string) (*models.Party, error) {
	var party models.Party
	if err := config.Db.Where("id = ?", partyID).First(&party).Error; err != nil {
		return nil, errors.New("party not found")
	}
	return &party, nil
}

func GetPartyDetail(partyID string) (*PartyDetail, error) {
	party, err := GetPartyByID(partyID)
	if err != nil {
		return nil, err
	}

	var partyMembers []models.PartyMember
	config.Db.Where("party_id = ?", party.ID).Find(&partyMembers)

	members := []MemberInfo{}
	for _, pm := range partyMembers {
		var user models.User
		if err := config.Db.Where("id = ?", pm.UserID).First(&user).Error; err != nil {
			continue
		}
		members = append(members, MemberInfo{
			UserID:   pm.UserID,
			UserName: user.UserName,
			Gender:   user.Gender,
			Status:   pm.Status,
		})
	}

	return &PartyDetail{
		Party:   *party,
		Members: members,
	}, nil
}

func DeleteParty(partyid string, userid string) (*models.Party, error) {
	var exist models.Party
	if err := config.Db.Where("id=?", partyid).First(&exist).Error; err != nil {
		return nil, err
	}
	owner := exist.OwnerID
	if owner == userid {
		// 软删该 party 下所有成员记录
		if err := config.Db.Where("party_id = ?", exist.ID).Delete(&models.PartyMember{}).Error; err != nil {
			return nil, err
		}
		if err := config.Db.Where("id=?", partyid).Delete(&exist).Error; err != nil {
			return nil, err
		}
		// 删除后同步从列表索引移除（软删行不会被列表查询返回，此处为主动清理）
		partyListIndexZRem(exist.ID)
	} else {
		return nil, errors.New("only owner have right")
	}
	return &exist, nil
}

func JoinParty(userID string, partyID string) (*models.PartyMember, error) {
	if utils.RuserinP(userID, partyID) {
		return nil, errors.New("You are already in the party")
	}

	// 取 party 并校验人数上限（人满不可再进）。
	// 注：单实例部署下"先查后插"可接受；若要多实例/高并发，需改用原子条件更新或 SELECT ... FOR UPDATE。
	party, err := GetPartyByID(partyID)
	if err != nil {
		return nil, err
	}
	if party.MaxNum <= 0 || party.Playernum >= party.MaxNum {
		return nil, errors.New("party is full")
	}

	//格式转换string->uint
	id, err := strconv.ParseUint(partyID, 10, 64)
	if err != nil {
		return nil, errors.New("invalid party id")
	}
	member := models.PartyMember{
		PartyID: uint(id),
		UserID:  userID,
		Status:  "JOINED",
	}
	if err := config.Db.Create(&member).Error; err != nil {
		return nil, errors.New("Create partymember failed")
	}

	// 加入后当前人数 +1
	if err := config.Db.Model(&models.Party{}).Where("id = ?", id).
		Update("playernum", gorm.Expr("playernum + 1")).Error; err != nil {
		return nil, errors.New("Update playernum failed")
	}
	return &member, nil
}

func LeaveParty(userID string, partyID string) (*models.PartyMember, error) {
	if !utils.RuserinP(userID, partyID) {
		return nil, errors.New("You are not in the party")
	}
	id, err := strconv.ParseUint(partyID, 10, 64)
	if err != nil {
		return nil, errors.New("invalid party id")
	}
	// 硬删（退队记录留着没意义，避免唯一索引冲突无法再加）
	if err := config.Db.Unscoped().Where("party_id = ? AND user_id = ?", id, userID).Delete(&models.PartyMember{}).Error; err != nil {
		return nil, errors.New("Leave party failed")
	}
	config.Db.Model(&models.Party{}).Where("id = ?", id).Update("playernum", gorm.Expr("playernum - 1"))
	return nil, nil
}
