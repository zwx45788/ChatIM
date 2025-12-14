package websocket

import (
	"context"
	"encoding/json"
	"log"

	"ChatIM/pkg/config"

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
	go subscribePrivateMessages(hub, rdb)

	log.Println("✅ Subscriber started - unified stream architecture (private + group)")
}

// subscribePrivateMessages 订阅消息通知（私聊 + 群聊统一）
// 现在私聊和群聊消息都写入用户的 stream:private:{user_id}
// 通过 "type" 字段区分消息类型："private" 或 "group"
func subscribePrivateMessages(hub *Hub, rdb *redis.Client) {
	pubsub := rdb.Subscribe(context.Background(), "message_notifications")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("✅ Subscribed to Redis channel 'message_notifications' (unified)")

	for msg := range ch {
		log.Printf("📨 Message notification: %s", msg.Payload)

		var notification map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Payload), &notification); err != nil {
			log.Printf("Failed to unmarshal notification: %v", err)
			continue
		}

		toUserID, ok := notification["to_user_id"].(string)
		if !ok {
			log.Printf("Invalid to_user_id in notification")
			continue
		}

		msgType, _ := notification["type"].(string)

		// 构建推送消息（直接使用通知中的数据，无需查询数据库）
		var pushMessage map[string]interface{}

		if msgType == "group" {
			// 群聊消息
			pushMessage = map[string]interface{}{
				"type":         "group",
				"id":           notification["msg_id"],
				"group_id":     notification["group_id"],
				"from_user_id": notification["from_user_id"],
				"content":      notification["content"],
				"created_at":   notification["created_at"],
			}
		} else {
			// 私聊消息（默认）
			pushMessage = map[string]interface{}{
				"type":         "private",
				"id":           notification["msg_id"],
				"from_user_id": notification["from_user_id"],
				"to_user_id":   notification["to_user_id"],
				"content":      notification["content"],
				"created_at":   notification["created_at"],
			}
		}

		messageJSON, err := json.Marshal(pushMessage)
		if err != nil {
			log.Printf("Failed to marshal push message: %v", err)
			continue
		}

		// 推送给目标用户
		hub.SendMessageToUser(toUserID, messageJSON)
		log.Printf("✅ Message pushed to user %s via WebSocket", toUserID)
	}
}

// 已移除 fetchMessageFromDB 和 fetchGroupMessageFromDB 函数
// 现在直接使用 Redis 通知中的消息内容，无需再查询数据库，提升性能
