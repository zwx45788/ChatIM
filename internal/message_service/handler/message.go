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
		SELECT id, from_user_id, to_user_id, content, is_read, read_at, created_at
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
		var readAtStr sql.NullString

		err := rows.Scan(
			&msg.Id,
			&msg.FromUserId,
			&msg.ToUserId,
			&msg.Content,
			&msg.IsRead,
			&readAtStr,
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

		// 处理已读时间（可能为NULL）
		if readAtStr.Valid {
			readAt, err := time.Parse("2006-01-02 15:04:05", readAtStr.String)
			if err != nil {
				log.Printf("Failed to parse read_at time: %v", err)
			} else {
				msg.ReadAt = readAt.Unix()
			}
		}

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

// MarkMessagesAsRead 标记消息为已读
func (h *MessageHandler) MarkMessagesAsRead(ctx context.Context, req *pb.MarkMessagesAsReadRequest) (*pb.MarkMessagesAsReadResponse, error) {
	// 1. 验证用户身份
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is marking messages as read", userID)

	if len(req.MessageIds) == 0 {
		return &pb.MarkMessagesAsReadResponse{
			Code:        0,
			Message:     "没有需要标记的消息",
			MarkedCount: 0,
		}, nil
	}

	// 2. 构建批量更新 SQL（只更新接收者是当前用户的消息）
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// 使用 IN 子句进行批量更新
	placeholders := ""
	args := make([]interface{}, len(req.MessageIds)+2)
	args[0] = userID
	args[1] = currentTime
	for i, msgID := range req.MessageIds {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i+2] = msgID
	}

	query := `UPDATE messages SET is_read = TRUE, read_at = ? 
	          WHERE to_user_id = ? AND id IN (` + placeholders + `)`

	result, err := h.db.ExecContext(ctx, query, append([]interface{}{currentTime, userID}, args[2:]...)...)
	if err != nil {
		log.Printf("Failed to mark messages as read: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to mark messages as read")
	}

	// 3. 获取受影响的行数
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to get affected rows: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to get affected rows")
	}

	log.Printf("Successfully marked %d messages as read for user %s", rowsAffected, userID)

	return &pb.MarkMessagesAsReadResponse{
		Code:        0,
		Message:     "消息已标记为已读",
		MarkedCount: int32(rowsAffected),
	}, nil
}

// GetUnreadCount 获取用户的未读消息数
func (h *MessageHandler) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	// 1. 验证用户身份
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is checking unread count", userID)

	// 2. 查询未读消息数
	query := `SELECT COUNT(*) FROM messages WHERE to_user_id = ? AND is_read = FALSE`
	var unreadCount int32
	err = h.db.QueryRowContext(ctx, query, userID).Scan(&unreadCount)
	if err != nil {
		log.Printf("Failed to query unread count for user %s: %v", userID, err)
		return nil, status.Errorf(codes.Internal, "Failed to query unread count")
	}

	log.Printf("User %s has %d unread messages", userID, unreadCount)

	return &pb.GetUnreadCountResponse{
		Code:        0,
		Message:     "查询成功",
		UnreadCount: unreadCount,
	}, nil
}

// PullUnreadMessages 拉取所有未读消息（自动标记为已读）
func (h *MessageHandler) PullUnreadMessages(ctx context.Context, req *pb.PullUnreadMessagesRequest) (*pb.PullUnreadMessagesResponse, error) {
	// 1. 验证用户身份
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is pulling unread messages", userID)

	// 设置默认 limit（最多 100 条）
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	// 2. 先查询总未读数
	var totalUnread int32
	countQuery := `SELECT COUNT(*) FROM messages WHERE to_user_id = ? AND is_read = FALSE`
	err = h.db.QueryRowContext(ctx, countQuery, userID).Scan(&totalUnread)
	if err != nil {
		log.Printf("Failed to query total unread count: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query unread count")
	}

	// 3. 查询未读消息列表
	query := `
		SELECT id, from_user_id, to_user_id, content, is_read, read_at, created_at
		FROM messages
		WHERE to_user_id = ? AND is_read = FALSE
		ORDER BY created_at DESC
		LIMIT ?`

	rows, err := h.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		log.Printf("Failed to query unread messages for user %s: %v", userID, err)
		return nil, status.Errorf(codes.Internal, "Failed to query unread messages")
	}
	defer rows.Close()

	// 4. 遍历结果集，构建消息列表和 ID 列表
	var messages []*pb.Message
	var messageIDs []string
	for rows.Next() {
		var msg pb.Message
		var createdAtStr string
		var readAtStr sql.NullString

		err := rows.Scan(
			&msg.Id,
			&msg.FromUserId,
			&msg.ToUserId,
			&msg.Content,
			&msg.IsRead,
			&readAtStr,
			&createdAtStr,
		)
		if err != nil {
			log.Printf("Failed to scan message row: %v", err)
			continue
		}

		// 将时间字符串转换为时间戳
		createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
		if err != nil {
			log.Printf("Failed to parse created_at time: %v", err)
			continue
		}
		msg.CreatedAt = createdAt.Unix()

		// 处理已读时间（可能为NULL）
		if readAtStr.Valid {
			readAt, err := time.Parse("2006-01-02 15:04:05", readAtStr.String)
			if err != nil {
				log.Printf("Failed to parse read_at time: %v", err)
			} else {
				msg.ReadAt = readAt.Unix()
			}
		}

		messages = append(messages, &msg)
		messageIDs = append(messageIDs, msg.Id)
	}

	// 检查遍历过程中是否有错误
	if err = rows.Err(); err != nil {
		log.Printf("Error occurred during rows iteration: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to process messages")
	}

	// 5. 如果启用自动标记，将这些消息标记为已读
	if req.AutoMark && len(messageIDs) > 0 {
		markReq := &pb.MarkMessagesAsReadRequest{
			MessageIds: messageIDs,
		}
		_, err := h.MarkMessagesAsRead(ctx, markReq)
		if err != nil {
			// 记录日志但不影响返回消息
			log.Printf("Warning: failed to auto-mark messages as read: %v", err)
		} else {
			log.Printf("Successfully auto-marked %d messages as read for user %s", len(messageIDs), userID)
		}
	}

	// 6. 判断是否还有更多未读消息
	hasMore := totalUnread > int32(limit)

	log.Printf("Successfully pulled %d unread messages for user %s (total: %d)", len(messages), userID, totalUnread)

	// 7. 返回响应
	return &pb.PullUnreadMessagesResponse{
		Code:        0,
		Message:     "成功拉取未读消息",
		Msgs:        messages,
		TotalUnread: totalUnread,
		HasMore:     hasMore,
	}, nil
}
