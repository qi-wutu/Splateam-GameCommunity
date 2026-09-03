package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"splatoon-backend/config"

	"github.com/redis/go-redis/v9"
)

// ---------- 只测进程内的 Hub，用来定位瓶颈 ----------
// 用 -cpu=1,2,4,8,16 看扩展性：若多核吞吐几乎线性上升，说明全局锁没拖住（RWMutex 读锁可共享）；
// 若持平，则锁是瓶颈。

// hubWithRoom 构造一个 Hub，并在指定房间塞入 clients 个客户端（无需真 websocket.Conn，广播只用到 Send chan）。
func hubWithRoom(roomID string, clients int) *Hub {
	h := NewHub()
	h.Rooms[roomID] = make(map[string]*Client)
	for i := 0; i < clients; i++ {
		uid := itoa(i)
		h.Rooms[roomID][uid] = &Client{
			Hub:    h,
			UserID: uid,
			Send:   make(chan []byte, 256),
			Rooms:  map[string]bool{roomID: true},
		}
	}
	return h
}

// BenchmarkHubBroadcast_Room200 单房间 200 客户端，单 goroutine 广播：量"一条消息送达 200 人"的纯 Hub 成本。
func BenchmarkHubBroadcast_Room200(b *testing.B) {
	h := hubWithRoom("room1", 200)
	msg := WsMessage{Type: "chat", PartyID: "room1", UserID: "u", Content: "hello"}
	b.ReportAllocs()
	for b.Loop() {
		h.BroadcastToRoom("room1", msg)
	}
}

// BenchmarkHubBroadcast_Room1 单房间 1 客户端：量"广播本身的固定开销"（锁+序列化），与房间大小解耦。
func BenchmarkHubBroadcast_Room1(b *testing.B) {
	h := hubWithRoom("room1", 1)
	msg := WsMessage{Type: "chat", PartyID: "room1", UserID: "u", Content: "hello"}
	b.ReportAllocs()
	for b.Loop() {
		h.BroadcastToRoom("room1", msg)
	}
}

// BenchmarkHubBroadcast_ParallelRooms 多 goroutine 同时广播到不同房间：
// 用 -cpu 看扩展性，判断全局锁是否是并发瓶颈。
func BenchmarkHubBroadcast_ParallelRooms(b *testing.B) {
	const rooms = 16
	h := NewHub()
	for r := 0; r < rooms; r++ {
		rid := "rm" + itoa(r)
		h.Rooms[rid] = make(map[string]*Client)
		for i := 0; i < 20; i++ {
			uid := itoa(i) + "_" + itoa(r)
			h.Rooms[rid][uid] = &Client{Hub: h, UserID: uid, Send: make(chan []byte, 256)}
		}
	}
	msg := WsMessage{Type: "chat", Content: "hi"}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			h.BroadcastToRoom("rm"+itoa(i%rooms), msg)
			i++
		}
	})
}

// setupBenchRedis 尝试连本地 Redis；连不上就跳过（纯 Hub 基准不含 Redis，二者对比即 Redis 写成本）。
func setupBenchRedis(b *testing.B) *redis.Client {
	b.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		b.Skip("本地 Redis 未运行，跳过含 Redis 的基准")
	}
	config.RedisClient = client
	config.RedisCtx = context.Background()
	b.Cleanup(func() { _ = client.Close() })
	return client
}

// BenchmarkCacheMessageRedis 每次聊天消息的"同步写 Redis"(RPush+LTrim) 成本——改造成本参照。
func BenchmarkCacheMessageRedis(b *testing.B) {
	setupBenchRedis(b)
	msg := WsMessage{Type: "chat", PartyID: "room1", UserID: "u", Content: "hello"}
	b.ReportAllocs()
	for b.Loop() {
		cacheMessageToRedis("room1", msg)
	}
}

// BenchmarkCacheMessageAsync 异步"入队"成本（热路径实际行为：只入队，不等 Redis）。
// 用一个连不上的 RedisClient 只是为了不走 nil 早退，测出真实入队机器成本；worker 在后台超时重连，
// 不影响热路径。对照 BenchmarkCacheMessageRedis 的同步代价（需真实 Redis）。
func BenchmarkCacheMessageAsync(b *testing.B) {
	// 故意指向一个不存在的端口：入队路径不触达 Redis，触达的是后台 worker（off 热路径）。
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6390"})
	config.RedisClient = client
	config.RedisCtx = context.Background()
	b.Cleanup(func() { _ = client.Close() })

	msg := WsMessage{Type: "chat", PartyID: "room1", UserID: "u", Content: "hello"}
	b.ReportAllocs()
	for b.Loop() {
		cacheMessageAsync("room1", msg)
	}
}

// BenchmarkJWTMarshal 对照：一次 JSON 序列化的成本（确认序列化是否值得优化）。
func BenchmarkJWTMarshal(b *testing.B) {
	msg := WsMessage{Type: "chat", PartyID: "room1", UserID: "u", Content: strings.Repeat("x", 50)}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(msg)
	}
}
