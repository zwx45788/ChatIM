package websocket

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// HandleWebSocket 处理 WebSocket 连接请求
func (h *Hub) HandleWebSocket(c *gin.Context) {
	// 1. 从查询参数中获取 token
	userIDInterface, exists := c.Get("userID")
	if !exists {
		// 如果走到这里，说明 AuthMiddleware 没有成功执行或者没有设置值
		log.Println("Error: userID not found in context after auth middleware")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	userID := userIDInterface.(string)

	// 3. 升级 HTTP 连接为 WebSocket 连接
	conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// 4. 创建客户端并注册到 Hub
	client := &Client{
		Conn:   conn,
		UserID: userID,
		Send:   make(chan []byte, 256), // 带缓冲的通道
	}

	h.register <- client

	// 5. 启动两个 goroutine 来处理读写
	go client.writePump() // 负责发送消息
	go client.readPump(h) // 负责读取消息
}

// readPump 持续从 WebSocket 连接读取消息
func (c *Client) readPump(h *Hub) {
	defer func() {
		h.unregister <- c
		c.Conn.Close()
	}()

	// 设置读取超时和最大消息大小
	c.Conn.SetReadLimit(512)
	// ... (可以设置 pong handler 等)

	for {
		// 我们暂时不处理客户端发来的消息，只负责推送
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// writePump 持续向 WebSocket 连接写入消息
func (c *Client) writePump() {
	defer c.Conn.Close()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// 通道被关闭
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.Conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Failed to write message: %v", err)
				return
			}
		}
	}
}
