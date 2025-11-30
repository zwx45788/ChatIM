package websocket

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Upgrader 用于将 HTTP 连接升级为 WebSocket 连接
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 在生产环境中，这里应该检查 r.Header.Get("Origin")
		// 开发阶段，我们先允许所有来源
		return true
	},
}

// Client 代表一个 WebSocket 客户端
type Client struct {
	Conn   *websocket.Conn
	UserID string
	Send   chan []byte // 发送消息的通道
}

// Hub 管理所有的客户端连接
type Hub struct {
	// 注册的客户端，key 是 UserID
	clients map[string]*Client

	// 从客户端接收的消息
	broadcast chan []byte

	// 注册请求
	register chan *Client

	// 注销请求
	unregister chan *Client

	// 读写锁，保护 clients map
	mu sync.RWMutex
}

// NewHub 创建一个新的 Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 启动 Hub 的主循环
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserID] = client
			h.mu.Unlock()
			log.Printf("Client %s connected", client.UserID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.UserID]; ok {
				delete(h.clients, client.UserID)
				close(client.Send)
				log.Printf("Client %s disconnected", client.UserID)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// 这个 broadcast 通道我们暂时用不到，先留在这里
			// 后续如果需要群发，可以用它
			log.Printf("Broadcasting message: %s", string(message))
		}
	}
}

// SendMessageToUser 向指定用户发送消息
func (h *Hub) SendMessageToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if client, ok := h.clients[userID]; ok {
		select {
		case client.Send <- message:
			log.Printf("Message sent to user %s", userID)
		default:
			// 通道已满或已关闭，认为客户端已断开
			log.Printf("Failed to send message to user %s, channel blocked", userID)
			close(client.Send)
			delete(h.clients, userID)
		}
	} else {
		log.Printf("User %s is not connected", userID)
	}
}

func (h *Hub) NotifyUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if client, ok := h.clients[userID]; ok {
		select {
		case client.Send <- message:
			log.Printf("Message successfully queued for user %s", userID)
		default:
			// 通道已满或已关闭，认为客户端已断开
			log.Printf("User %s is connected but send channel is blocked, disconnecting.", userID)
			close(client.Send)
			delete(h.clients, userID)
		}
	} else {
		log.Printf("User %s is not online, skipping WebSocket push.", userID)
	}
}
