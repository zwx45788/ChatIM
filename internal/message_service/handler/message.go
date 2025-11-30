package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ChatIM/api/proto/message"
	"ChatIM/pkg/auth"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type MessageHandler struct {
	pb.UnimplementedMessageServiceServer
	db  *sql.DB
	rdb *redis.Client
}

func NewMessageHandler(db *sql.DB, rdb *redis.Client) *MessageHandler {
	return &MessageHandler{
		db:  db,
		rdb: rdb,
	}
}

// SendMessage 实现发送消息的接口
func (h *MessageHandler) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	fromUserID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("User %s is sending a message to %s", fromUserID, req.ToUserId)

	msgID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	query := `INSERT INTO messages (id, from_user_id, to_user_id, content, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err = h.db.ExecContext(ctx, query, msgID, fromUserID, req.ToUserId, req.Content, createdAt)
	if err != nil {
		log.Printf("Failed to insert message into database: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to save message")
	}
	log.Printf("Message %s saved successfully", msgID)

	// 👇 4. 【核心】发布消息到 Redis
	notificationPayload := map[string]string{
		"to_user_id": req.ToUserId,
		"msg_id":     msgID,
	}
	payloadBytes, err := json.Marshal(notificationPayload)
	if err != nil {
		log.Printf("Failed to marshal notification payload: %v", err)
		// 不影响主流程，只记录日志
	} else {
		// 发布到 "message_notifications" 频道
		err = h.rdb.Publish(ctx, "message_notifications", payloadBytes).Err()
		if err != nil {
			log.Printf("Warning: failed to publish message notification to Redis: %v", err)
			// 同样，不返回错误，只记录日志
		} else {
			log.Printf("Successfully published notification for message %s to user %s", msgID, req.ToUserId)
		}
	}

	return &pb.SendMessageResponse{
		Code:    0,
		Message: "消息发送成功",
		Msg: &pb.Message{
			Id:         msgID,
			FromUserId: fromUserID,
			ToUserId:   req.ToUserId,
			Content:    req.Content,
			CreatedAt:  time.Now().Unix(),
		},
	}, nil
}

// internal/message_service/handler/message_handler.go

// PullMessages 拉取当前用户的消息列表
func (h *MessageHandler) PullMessages(ctx context.Context, req *pb.PullMessagesRequest) (*pb.PullMessagesResponse, error) {
	// 1. 获取当前用户 ID (复用我们之前写的函数)
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is pulling messages", userID)

	// 2. 准备 SQL 查询
	query := `
		SELECT id, from_user_id, to_user_id, content, created_at
		FROM messages
		WHERE to_user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	// 3. 执行查询
	rows, err := h.db.QueryContext(ctx, query, userID, req.Limit, req.Offset)
	if err != nil {
		log.Printf("Failed to query messages for user %s: %v", userID, err)
		return nil, status.Errorf(codes.Internal, "Failed to query messages")
	}
	defer rows.Close() // 非常重要！确保 rows 最终被关闭

	// 4. 遍历结果集，构建消息列表
	var messages []*pb.Message
	for rows.Next() {
		var msg pb.Message
		var createdAtStr string // 从数据库读出的是字符串，需要转换

		err := rows.Scan(
			&msg.Id,
			&msg.FromUserId,
			&msg.ToUserId,
			&msg.Content,
			&createdAtStr,
		)
		if err != nil {
			log.Printf("Failed to scan message row: %v", err)
			continue // 或者直接返回错误
		}

		// 将时间字符串转换为时间戳
		createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			log.Printf("Failed to parse created_at time: %v", err)
			continue
		}
		msg.CreatedAt = createdAt.Unix()

		messages = append(messages, &msg)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error occurred during rows iteration: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to process messages")
	}

	log.Printf("Successfully pulled %d messages for user %s", len(messages), userID)

	// 5. 返回响应
	return &pb.PullMessagesResponse{
		Code:    0,
		Message: "消息拉取成功",
		Msgs:    messages,
	}, nil
}
