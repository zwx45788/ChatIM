package websocket

import (
	"context"
	"encoding/json"
	"log"

	"ChatIM/pkg/config"
	"ChatIM/pkg/database"

	"github.com/redis/go-redis/v9"
)

// MessagePayload 用于解析从数据库查询出的私聊消息
type MessagePayload struct {
	ID         string `json:"id"`
	FromUserID string `json:"from_user_id"`
	ToUserID   string `json:"to_user_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
	Type       string `json:"type"` // "private"
}

// GroupMessagePayload 群聊消息结构
type GroupMessagePayload struct {
	ID         string `json:"id"`
	GroupID    string `json:"group_id"`
	FromUserID string `json:"from_user_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
	Type       string `json:"type"` // "group"
}

// GroupMessageNotification 群聊消息通知结构
type GroupMessageNotification struct {
	MsgID      string `json:"msg_id"`
	GroupID    string `json:"group_id"`
	FromUserID string `json:"from_user_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

// StartSubscriber 启动 Redis 订阅者（统一使用 Stream 架构）
// 私聊和群聊消息都写入用户的 stream:private:{user_id}，统一处理
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

	// 启动消息通知订阅（私聊和群聊统一通知）
	go subscribePrivateMessages(hub, rdb, cfg)

	log.Println("✅ Subscriber started - unified stream architecture (private + group)")
}

// subscribePrivateMessages 订阅消息通知（私聊 + 群聊统一）
// 现在私聊和群聊消息都写入用户的 stream:private:{user_id}
// 通过 "type" 字段区分消息类型："private" 或 "group"
func subscribePrivateMessages(hub *Hub, rdb *redis.Client, cfg *config.Config) {
	pubsub := rdb.Subscribe(context.Background(), "message_notifications")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("✅ Subscribed to Redis channel 'message_notifications' (unified)")

	for msg := range ch {
		log.Printf("📨 Message notification: %s", msg.Payload)

		var notification map[string]string
		if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
			log.Printf("Failed to unmarshal notification: %v", err)
			continue
		}

		toUserID := notification["to_user_id"]
		msgID := notification["msg_id"]
		msgType := notification["type"] // "private" 或 "group"

		// 从数据库查询完整消息
		var messageJSON []byte
		var err error

		if msgType == "group" {
			// 群聊消息
			groupMsg, err := fetchGroupMessageFromDB(msgID, cfg)
			if err != nil {
				log.Printf("Failed to fetch group message %s from DB: %v", msgID, err)
				continue
			}
			messageJSON, err = json.Marshal(groupMsg)
		} else {
			// 私聊消息（默认）
			privateMsg, err := fetchMessageFromDB(msgID, cfg)
			if err != nil {
				log.Printf("Failed to fetch message %s from DB: %v", msgID, err)
				continue
			}
			messageJSON, err = json.Marshal(privateMsg)
		}

		if err != nil {
			log.Printf("Failed to marshal message: %v", err)
			continue
		}

		hub.NotifyUser(toUserID, messageJSON)
	}
}

// fetchMessageFromDB 从数据库查询私聊消息的辅助函数
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

	message.Type = "private"
	return &message, nil
}

// fetchGroupMessageFromDB 从数据库查询群聊消息
func fetchGroupMessageFromDB(msgID string, cfg *config.Config) (*GroupMessagePayload, error) {
	db, err := database.InitDB(cfg.Database.MySQL.DSN)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var message GroupMessagePayload
	query := `SELECT id, group_id, from_user_id, content, created_at FROM group_messages WHERE id = ?`
	err = db.QueryRow(query, msgID).Scan(&message.ID, &message.GroupID, &message.FromUserID, &message.Content, &message.CreatedAt)
	if err != nil {
		return nil, err
	}

	message.Type = "group"
	return &message, nil
}
