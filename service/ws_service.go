package service

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"splatoon-backend/config"

	"github.com/gorilla/websocket"
)

// ---------- 常量 ----------
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096

	// Redis 键前缀
	redisOnlinePrefix = "user:online:"  // + userID → string "online"，带 TTL
	redisMsgPrefix    = "party:msgs:"   // + partyID → List，存最近 100 条消息 JSON
	redisMsgMaxLen    = 100             // 每个房间最多缓存 100 条
	redisOnlineTTL    = pongWait + 10*time.Second // 在线状态 TTL（比 ping 间隔略长）
)

// ---------- 消息协议 ----------
type WsMessage struct {
	Type     string `json:"type"`               // chat / notification / join_room / leave_room / chat_history
	PartyID  string `json:"partyId,omitempty"`  // 组队ID
	UserID   string `json:"userId,omitempty"`   // 发送者ID
	UserName string `json:"userName,omitempty"` // 发送者昵称
	Content  string `json:"content,omitempty"`  // 消息内容（chat_history 时存放 JSON 数组）
	Time     string `json:"time,omitempty"`     // 时间
}

// ---------- Client ----------
type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	UserID   string
	UserName string        // 发送者昵称缓存：连接期间只查一次库
	Rooms    map[string]bool // 当前在哪些房间 (partyId -> true)
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		Hub:    hub,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		UserID: userID,
		Rooms:  make(map[string]bool),
	}
}

// ReadPump: 从 WebSocket 连接读消息
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg WsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "join_room":
			c.Hub.JoinRoom(c, msg.PartyID)
			// 加入房间后从 Redis 拉聊天历史
			c.sendChatHistory(msg.PartyID)

		case "leave_room":
			c.Hub.LeaveRoom(c, msg.PartyID)

		case "chat":
			if msg.PartyID == "" || msg.Content == "" {
				continue
			}
			// 发送者昵称：连接期间缓存，不再每条消息查库
			reply := WsMessage{
				Type:     "chat",
				PartyID:  msg.PartyID,
				UserID:   c.UserID,
				UserName: c.displayName(),
				Content:  msg.Content,
				Time:     time.Now().Format("15:04"),
			}
			c.Hub.BroadcastToRoom(msg.PartyID, reply)

			// ----- 缓存到 Redis（异步，不阻塞读路径） -----
			cacheMessageAsync(msg.PartyID, reply)
		}
	}
}

// sendChatHistory: 从 Redis 拉取最近消息发给当前客户端
func (c *Client) sendChatHistory(partyID string) {
	if config.RedisClient == nil {
		return
	}

	msgs, err := config.RedisClient.LRange(config.RedisCtx, redisMsgPrefix+partyID, 0, -1).Result()
	if err != nil || len(msgs) == 0 {
		return
	}

	// Redis 中存的是逐条 JSON，拼成 JSON 数组
	historyJSON := "[" + strings.Join(msgs, ",") + "]"
	reply := WsMessage{
		Type:    "chat_history",
		PartyID: partyID,
		Content: historyJSON,
	}
	data, _ := json.Marshal(reply)
	c.Send <- data
}

// displayName: 发送者昵称，连接期间只查一次库并缓存；查不到则退化为 UserID。
func (c *Client) displayName() string {
	if c.UserName != "" {
		return c.UserName
	}
	if user, err := GetUserByID(c.UserID); err == nil {
		c.UserName = user.UserName
	} else {
		c.UserName = c.UserID
	}
	return c.UserName
}

// cacheMessageToRedis: 同步写 Redis List 并裁剪长度（只保留最近 N 条）。
// 供后台 worker 与基准测试调用；热路径请用 cacheMessageAsync。
func cacheMessageToRedis(partyID string, msg WsMessage) {
	if config.RedisClient == nil {
		return
	}

	data, _ := json.Marshal(msg)
	key := redisMsgPrefix + partyID

	config.RedisClient.RPush(config.RedisCtx, key, data)
	config.RedisClient.LTrim(config.RedisCtx, key, int64(-redisMsgMaxLen), -1)
}

// ---------- 异步 Redis 缓存（聊天历史）----------
// 聊天消息热路径只入队、不阻塞；由 cacheWorkers 个后台协程真正写 Redis。
// 队列满则丢弃本次历史缓存——与 BroadcastToRoom 的"缓冲满丢消息"策略一致，
// 避免 Redis 变慢或抖动拖死 ReadPump。

type redisCacheJob struct {
	partyID string
	msg     WsMessage
}

const (
	cacheJobQueue = 4096 // 未落盘缓存任务上限
	cacheWorkers  = 2    // 写 Redis 的后台协程数
)

var (
	cacheJobs = make(chan redisCacheJob, cacheJobQueue)
	cacheOnce sync.Once
)

func cacheWorker() {
	for j := range cacheJobs {
		cacheMessageToRedis(j.partyID, j.msg)
	}
}

// ensureCacheWorkers 惰性启动后台协程（首个任务到来时）。
func ensureCacheWorkers() {
	cacheOnce.Do(func() {
		for i := 0; i < cacheWorkers; i++ {
			go cacheWorker()
		}
	})
}

// cacheMessageAsync: 聊天消息热路径用——入队即返回，不触碰 Redis，零阻塞。
func cacheMessageAsync(partyID string, msg WsMessage) {
	if config.RedisClient == nil {
		return
	}
	ensureCacheWorkers()
	select {
	case cacheJobs <- redisCacheJob{partyID: partyID, msg: msg}:
	default:
		// 队列满，丢弃，保证读路径不被 Redis 拖死
	}
}

// WritePump: 往 WebSocket 连接写消息
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 把队列里积压的消息一起发出去
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			// ----- 心跳续期 Redis 在线状态 -----
			if config.RedisClient != nil {
				config.RedisClient.Expire(config.RedisCtx, redisOnlinePrefix+c.UserID, redisOnlineTTL)
			}
		}
	}
}

// ---------- Hub ----------
type Hub struct {
	Clients    map[string]*Client            // 所有在线客户端 (userID -> *Client)
	Rooms      map[string]map[string]*Client // 房间 (partyId -> userID -> *Client)
	Register   chan *Client
	Unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Rooms:      make(map[string]map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			// 如果已有同 ID 的连接，关掉旧的
			if old, ok := h.Clients[client.UserID]; ok {
				close(old.Send)
				old.Conn.Close()
			}
			h.Clients[client.UserID] = client
			h.mu.Unlock()

			// Redis: 标记在线（断连时 TTL 到期自动过期，Unregister 也会主动删）
			if config.RedisClient != nil {
				config.RedisClient.Set(config.RedisCtx, redisOnlinePrefix+client.UserID, "online", redisOnlineTTL)
			}
			log.Printf("[WS] 连接: user=%s, 在线: %d", client.UserID, len(h.Clients))

		case client := <-h.Unregister:
			h.mu.Lock()
			// 从所有房间移除
			for partyID := range client.Rooms {
				if room, ok := h.Rooms[partyID]; ok {
					delete(room, client.UserID)
					if len(room) == 0 {
						delete(h.Rooms, partyID)
					}
				}
			}
			// 从在线列表移除
			if _, ok := h.Clients[client.UserID]; ok {
				delete(h.Clients, client.UserID)
				close(client.Send)
			}
			h.mu.Unlock()

			// Redis: 移除在线标记
			if config.RedisClient != nil {
				config.RedisClient.Del(config.RedisCtx, redisOnlinePrefix+client.UserID)
			}
			log.Printf("[WS] 断开: user=%s, 在线: %d", client.UserID, len(h.Clients))
		}
	}
}

// JoinRoom: 客户端加入房间
func (h *Hub) JoinRoom(client *Client, partyID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.Rooms[partyID]; !ok {
		h.Rooms[partyID] = make(map[string]*Client)
	}
	h.Rooms[partyID][client.UserID] = client
	client.Rooms[partyID] = true
}

// LeaveRoom: 客户端离开房间
func (h *Hub) LeaveRoom(client *Client, partyID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.Rooms[partyID]; ok {
		delete(room, client.UserID)
		if len(room) == 0 {
			delete(h.Rooms, partyID)
		}
	}
	delete(client.Rooms, partyID)
}

// BroadcastToRoom: 给房间所有成员发消息
func (h *Hub) BroadcastToRoom(partyID string, msg WsMessage) {
	data, _ := json.Marshal(msg)

	h.mu.RLock()
	defer h.mu.RUnlock()

	room, ok := h.Rooms[partyID]
	if !ok {
		return
	}
	for _, client := range room {
		select {
		case client.Send <- data:
		default:
			// 缓冲区满，跳过
		}
	}
}

// SendToUser: 给指定用户发消息
func (h *Hub) SendToUser(userID string, msg WsMessage) {
	data, _ := json.Marshal(msg)

	h.mu.RLock()
	client, ok := h.Clients[userID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	select {
	case client.Send <- data:
	default:
	}
}

// ---------- Redis 辅助查询（给 HTTP handler 用）----------

// CheckUserOnline 检查用户是否在线（Redis）
func CheckUserOnline(userID string) bool {
	if config.RedisClient == nil {
		return false
	}
	val, err := config.RedisClient.Get(config.RedisCtx, redisOnlinePrefix+userID).Result()
	return err == nil && val == "online"
}

// ---------- 全局 Hub 实例（供 HTTP handler 调用推送）----------
var GlobalHub = NewHub()

// NotifyPartyMembers: 通知组队所有成员（HTTP handler 调这个）
func NotifyPartyMembers(partyID string, content string) {
	GlobalHub.BroadcastToRoom(partyID, WsMessage{
		Type:    "notification",
		PartyID: partyID,
		Content: content,
		Time:    time.Now().Format("15:04"),
	})
}

// NotifyUser: 通知单个用户
func NotifyUser(userID string, content string) {
	GlobalHub.SendToUser(userID, WsMessage{
		Type:    "notification",
		Content: content,
		Time:    time.Now().Format("15:04"),
	})
}
