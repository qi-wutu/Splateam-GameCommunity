package service

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ---------- 常量 ----------
const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// ---------- 消息协议 ----------
type WsMessage struct {
	Type     string `json:"type"`               // chat / notification / join_room / leave_room
	PartyID  string `json:"partyId,omitempty"`  // 组队ID
	UserID   string `json:"userId,omitempty"`   // 发送者ID
	UserName string `json:"userName,omitempty"` // 发送者昵称
	Content  string `json:"content,omitempty"`  // 消息内容
	Time     string `json:"time,omitempty"`     // 时间
}

// ---------- Client ----------
type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	UserID string
	Rooms  map[string]bool // 当前在哪些房间 (partyId -> true)
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

		case "leave_room":
			c.Hub.LeaveRoom(c, msg.PartyID)

		case "chat":
			if msg.PartyID == "" || msg.Content == "" {
				continue
			}
			// 查发送者信息
			user, err := GetUserByID(c.UserID)
			userName := c.UserID
			if err == nil {
				userName = user.UserName
			}
			reply := WsMessage{
				Type:     "chat",
				PartyID:  msg.PartyID,
				UserID:   c.UserID,
				UserName: userName,
				Content:  msg.Content,
				Time:     time.Now().Format("15:04"),
			}
			c.Hub.BroadcastToRoom(msg.PartyID, reply)
		}
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
		}
	}
}

// ---------- Hub ----------
type Hub struct {
	Clients    map[string]*Client                 // 所有在线客户端 (userID -> *Client)
	Rooms      map[string]map[string]*Client      // 房间 (partyId -> userID -> *Client)
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
	room, ok := h.Rooms[partyID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
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
