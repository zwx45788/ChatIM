package handler

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ChatIM/api/proto/message"
	"ChatIM/pkg/auth" // 假设你的 JWT 工具函数在这里

	"github.com/google/uuid"
)

type MessageHandler struct {
	pb.UnimplementedMessageServiceServer
	db *sql.DB
}

func NewMessageHandler(db *sql.DB) *MessageHandler {
	return &MessageHandler{
		db: db,
	}
}

// SendMessage 实现发送消息的接口
func (h *MessageHandler) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	// 1. 从上下文中获取 user_id (发送者)
	// md, ok := metadata.FromIncomingContext(ctx)
	// if !ok {
	// 	return nil, status.Errorf(codes.Unauthenticated, "Missing metadata")
	// }
	// authHeaders := md["authorization"]
	// if len(authHeaders) == 0 {
	// 	return nil, status.Errorf(codes.Unauthenticated, "Missing authorization token")
	// }

	// // 👇 修改点 1: 清理 Token，去除 "Bearer " 前缀
	// tokenString := authHeaders[0]
	// tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// claims, err := auth.ParseToken(tokenString)
	// if err != nil {
	// 	return nil, status.Errorf(codes.Unauthenticated, "Invalid token: %v", err)
	// }
	fromUserID, err := auth.GetUserID(ctx) //检验token并getuserid
	if err != nil {
		return nil, err
	}
	log.Printf("User %s is sending a message to %s", fromUserID, req.ToUserId)

	// 2. 生成消息 ID 和时间戳
	msgID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// 3. 将消息插入数据库
	query := `INSERT INTO messages (id, from_user_id, to_user_id, content, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err = h.db.ExecContext(ctx, query, msgID, fromUserID, req.ToUserId, req.Content, createdAt)
	if err != nil {
		log.Printf("Failed to insert message into database: %v", err)

		// 👇 修改点 2: 增加更精确的错误判断
		// 检查是否是外键约束错误，即 to_user_id 不存在
		if errors.Is(err, sql.ErrNoRows) {
			// 注意：MySQL 的外键错误通常不是 sql.ErrNoRows，而是更具体的错误码
			// 这里用 sql.ErrNoRows 作为概念示例，实际可能需要检查错误字符串
			// 例如: strings.Contains(err.Error(), "Cannot add or update a child row")
			return nil, status.Errorf(codes.NotFound, "Receiver user not found")
		}

		return nil, status.Errorf(codes.Internal, "Failed to save message")
	}

	log.Printf("Message %s saved successfully", msgID)

	// 4. 返回成功响应
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
