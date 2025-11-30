package websocket

import (
	"context"
	"encoding/json"
	"log"

	"ChatIM/pkg/config"
	"ChatIM/pkg/database"

	"github.com/redis/go-redis/v9"
)

// MessagePayload 用于解析从数据库查询出的消息
type MessagePayload struct {
	ID         string `json:"id"`
	FromUserID string `json:"from_user_id"`
	ToUserID   string `json:"to_user_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

// StartSubscriber 启动 Redis 订阅者
// 它接收一个 Hub 实例，用于在收到消息时通知 Hub
func StartSubscriber(hub *Hub) {
	// 加载配置以连接 Redis
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config for redis subscriber: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Redis.Addr,
		Password: cfg.Database.Redis.Password,
		DB:       cfg.Database.Redis.DB,
	})

	// 创建订阅者
	pubsub := rdb.Subscribe(context.Background(), "message_notifications")
	defer pubsub.Close()

	// 获取消息通道
	ch := pubsub.Channel()

	log.Println("Successfully subscribed to Redis channel 'message_notifications'")

	// 持续监听消息
	for msg := range ch {
		log.Printf("Received notification from Redis: %s", msg.Payload)

		// 解析通知载荷
		var notification map[string]string
		if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
			log.Printf("Failed to unmarshal notification: %v", err)
			continue
		}

		toUserID := notification["to_user_id"]
		msgID := notification["msg_id"]

		// 从数据库查询完整的消息内容
		fullMessage, err := fetchMessageFromDB(msgID, cfg)
		if err != nil {
			log.Printf("Failed to fetch message %s from DB: %v", msgID, err)
			continue
		}

		// 将完整消息序列化为 JSON
		messageJSON, err := json.Marshal(fullMessage)
		if err != nil {
			log.Printf("Failed to marshal full message: %v", err)
			continue
		}

		// 通过 Hub 发送给目标用户
		hub.NotifyUser(toUserID, messageJSON)
	}
}

// fetchMessageFromDB 从数据库查询消息的辅助函数
func fetchMessageFromDB(msgID string, cfg *config.Config) (*MessagePayload, error) {
	db, err := database.InitDB(cfg.Database.MySQL.DSN)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var message MessagePayload
	query := `SELECT id, from_user_id, to_user_id, content, created_at FROM messages WHERE id = ?`
	err = db.QueryRow(query, msgID).Scan(&message.ID, &message.FromUserID, &message.ToUserID, &message.Content, &message.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &message, nil
}
