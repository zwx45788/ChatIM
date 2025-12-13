package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ChatIM/api/proto/message"
	"ChatIM/pkg/auth"
	"ChatIM/pkg/stream"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type MessageHandler struct {
	pb.UnimplementedMessageServiceServer
	db       *sql.DB
	rdb      *redis.Client
	streamOp *stream.StreamOperator
}

func NewMessageHandler(db *sql.DB, rdb *redis.Client) *MessageHandler {
	return &MessageHandler{
		db:       db,
		rdb:      rdb,
		streamOp: stream.NewStreamOperator(rdb),
	}
}

// SendMessage 实现发送消息的接口（使用 Redis Stream）
func (h *MessageHandler) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	fromUserID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}
	log.Printf("User %s is sending a message to %s", fromUserID, req.ToUserId)

	msgID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// 1. 立即写入 Redis Stream（快速响应）
	_, err = h.streamOp.AddPrivateMessage(ctx, msgID, fromUserID, req.ToUserId, req.Content)
	if err != nil {
		log.Printf("Failed to add private message to stream: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to save message")
	}

	// 2. 更新双方的会话列表
	conversationID := fmt.Sprintf("private:%s", req.ToUserId)
	h.streamOp.UpdateConversationTime(ctx, fromUserID, fmt.Sprintf("private:%s", fromUserID))
	h.streamOp.UpdateConversationTime(ctx, req.ToUserId, conversationID)

	// 2. 异步写入数据库（不阻塞用户）
	go func() {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := `INSERT INTO messages (id, from_user_id, to_user_id, content, created_at) VALUES (?, ?, ?, ?, ?)`
		_, err := h.db.ExecContext(dbCtx, query, msgID, fromUserID, req.ToUserId, req.Content, createdAt)
		if err != nil {
			log.Printf("Warning: failed to save message to database: %v", err)
		} else {
			log.Printf("Message %s saved to database successfully", msgID)
		}
	}()

	log.Printf("Message %s sent successfully", msgID)

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

// SendGroupMessage 发送群聊消息（写入每个成员的 Stream 并异步持久化到数据库）
func (h *MessageHandler) SendGroupMessage(ctx context.Context, req *pb.SendGroupMessageRequest) (*pb.SendGroupMessageResponse, error) {
	fromUserID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GroupId) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "group_id is required")
	}

	log.Printf("User %s is sending a group message to group %s", fromUserID, req.GroupId)

	msgID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// 1. 查询群成员列表
	memberIDs, err := h.getGroupMembers(ctx, req.GroupId)
	if err != nil {
		log.Printf("Failed to get group members: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to get group members")
	}

	if len(memberIDs) == 0 {
		return nil, status.Errorf(codes.NotFound, "Group has no members")
	}

	// 2. 写入所有成员的 Redis Stream (统一使用 stream:private:{user_id})
	err = h.streamOp.AddGroupMessageToMembers(ctx, msgID, req.GroupId, fromUserID, req.Content, "text", memberIDs)
	if err != nil {
		log.Printf("Failed to add group message to members' streams: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to save group message")
	}

	// 3. 更新所有成员的会话列表
	conversationID := fmt.Sprintf("group:%s", req.GroupId)
	for _, memberID := range memberIDs {
		h.streamOp.UpdateConversationTime(ctx, memberID, conversationID)
	}

	// 3. 异步写入数据库
	go func() {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := `INSERT INTO group_messages (id, group_id, from_user_id, content, created_at) VALUES (?, ?, ?, ?, ?)`
		_, err := h.db.ExecContext(dbCtx, query, msgID, req.GroupId, fromUserID, req.Content, createdAt)
		if err != nil {
			log.Printf("Warning: failed to save group message to database: %v", err)
		} else {
			log.Printf("Group message %s saved to database successfully", msgID)
		}
	}()

	log.Printf("✅ Group message %s sent to %d members via their personal streams", msgID, len(memberIDs))

	return &pb.SendGroupMessageResponse{
		Code:    0,
		Message: "群聊消息发送成功",
		Msg: &pb.GroupMessage{
			Id:         msgID,
			GroupId:    req.GroupId,
			FromUserId: fromUserID,
			Content:    req.Content,
			CreatedAt:  time.Now().Unix(),
		},
	}, nil
}

// getGroupMembers 获取群组的所有成员ID
func (h *MessageHandler) getGroupMembers(ctx context.Context, groupID string) ([]string, error) {
	// 先尝试从缓存读取
	cachedMembers, hit, _ := h.streamOp.GetCachedGroupMembers(ctx, groupID)
	if hit {
		return cachedMembers, nil
	}

	// 从数据库读取
	rows, err := h.db.QueryContext(ctx,
		"SELECT user_id FROM group_members WHERE group_id = ? AND is_deleted = 0",
		groupID)
	if err != nil {
		log.Printf("Error querying group members: %v", err)
		return nil, err
	}
	defer rows.Close()

	var members []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			continue
		}
		members = append(members, userID)
	}

	// 保存到缓存
	h.streamOp.CacheGroupMembers(ctx, groupID, members)

	return members, nil
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

	// 对分页参数做保护，避免无限制扫描
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// 2. 准备 SQL 查询
	query := `
		SELECT id, from_user_id, to_user_id, content, is_read, read_at, created_at
		FROM messages
		WHERE to_user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	// 3. 执行查询
	rows, err := h.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		log.Printf("Failed to query messages for user %s: %v", userID, err)
		return nil, status.Errorf(codes.Internal, "Failed to query messages")
	}
	defer rows.Close() // 非常重要！确保 rows 最终被关闭

	// 4. 遍历结果集，构建消息列表
	var messages []*pb.Message
	for rows.Next() {
		var (
			id           string
			fromUserID   string
			toUserID     string
			content      string
			isRead       bool
			readAtStr    sql.NullString
			createdAtStr string
		)

		if err := rows.Scan(&id, &fromUserID, &toUserID, &content, &isRead, &readAtStr, &createdAtStr); err != nil {
			log.Printf("Failed to scan message row: %v", err)
			continue
		}

		msg, err := convertDBRowToMessage(id, fromUserID, toUserID, content, isRead, readAtStr, createdAtStr)
		if err != nil {
			log.Printf("Failed to convert db row to message: %v", err)
			continue
		}

		messages = append(messages, msg)
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

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(req.MessageIds)), ",")
	args := make([]interface{}, 0, len(req.MessageIds)+2)
	args = append(args, currentTime, userID)
	for _, msgID := range req.MessageIds {
		args = append(args, msgID)
	}

	query := `UPDATE messages SET is_read = TRUE, read_at = ? 
	          WHERE to_user_id = ? AND id IN (` + placeholders + `)`

	result, err := h.db.ExecContext(ctx, query, args...)
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

	// 设置默认 limit，并限制最大值
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	// 2. 查询未读消息列表，使用窗口函数一次查询 total
	query := `
		SELECT id, from_user_id, to_user_id, content, is_read, read_at, created_at,
		       COUNT(*) OVER() AS total_count
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
	var (
		messages    []*pb.Message
		messageIDs  []string
		totalUnread int64
		totalSeen   bool
	)

	for rows.Next() {
		var (
			id           string
			fromUserID   string
			toUserID     string
			content      string
			isRead       bool
			readAtStr    sql.NullString
			createdAtStr string
			rowTotal     sql.NullInt64
		)

		if err := rows.Scan(&id, &fromUserID, &toUserID, &content, &isRead, &readAtStr, &createdAtStr, &rowTotal); err != nil {
			log.Printf("Failed to scan message row: %v", err)
			continue
		}

		msg, err := convertDBRowToMessage(id, fromUserID, toUserID, content, isRead, readAtStr, createdAtStr)
		if err != nil {
			log.Printf("Failed to convert db row to message: %v", err)
			continue
		}

		messages = append(messages, msg)
		messageIDs = append(messageIDs, msg.Id)

		if rowTotal.Valid && !totalSeen {
			totalUnread = rowTotal.Int64
			totalSeen = true
		}
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
	hasMore := totalUnread > int64(len(messages))

	log.Printf("Successfully pulled %d unread messages for user %s (total: %d)", len(messages), userID, totalUnread)

	// 7. 返回响应
	return &pb.PullUnreadMessagesResponse{
		Code:        0,
		Message:     "成功拉取未读消息",
		Msgs:        messages,
		TotalUnread: int32(totalUnread),
		HasMore:     hasMore,
	}, nil
}

// PullAllUnreadOnLogin 登录时拉取所有未读消息（私聊 + 群聊）
func (h *MessageHandler) PullAllUnreadOnLogin(ctx context.Context, req *pb.PullAllUnreadOnLoginRequest) (*pb.PullAllUnreadOnLoginResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is pulling all unread messages on login", userID)

	// 1. 记录用户上线时间
	h.streamOp.RecordUserOnlineTime(ctx, userID)

	// 2. 并发拉取私聊和群聊未读
	var (
		privateMessages []*pb.Message
		groupMessages   map[string]*pb.GroupUnreadInfo
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		privateMessages = h.pullPrivateUnread(ctx, userID)
	}()

	go func() {
		defer wg.Done()
		groupMessages = h.pullGroupUnread(ctx, userID)
	}()

	wg.Wait()

	// 5. 计算总数
	totalCount := int32(len(privateMessages))
	for _, detail := range groupMessages {
		totalCount += detail.UnreadCount
	}

	log.Printf("User %s pulled %d private unread and %d group unread messages",
		userID, len(privateMessages), len(groupMessages))

	response := &pb.PullAllUnreadOnLoginResponse{
		Code:               0,
		Message:            "成功拉取未读消息",
		PrivateMessages:    privateMessages,
		PrivateUnreadCount: int32(len(privateMessages)),
		GroupMessages:      groupMessages,
		GroupUnreadCount:   int32(len(groupMessages)),
		TotalUnreadCount:   totalCount,
		PulledAt:           time.Now().Format(time.RFC3339),
	}

	return response, nil
}

// pullPrivateUnread 拉取私聊未读消息
func (h *MessageHandler) pullPrivateUnread(ctx context.Context, userID string) []*pb.Message {
	streamKey := fmt.Sprintf("stream:private:%s", userID)

	// 从 Stream 读取所有消息，设置超时时间避免长时间阻塞
	streamCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	messages, err := h.streamOp.ReadMessages(streamCtx, streamKey, "-", 1000)
	if err != nil {
		log.Printf("Error reading private messages from stream: %v", err)
		return []*pb.Message{}
	}

	var result []*pb.Message
	for _, msg := range messages {
		pbMsg, ok := convertStreamEntryToMessage(msg, userID, true)
		if !ok {
			continue
		}
		result = append(result, pbMsg)
	}

	log.Printf("User %s pulled %d private unread messages", userID, len(result))
	return result
}

// pullGroupUnread 拉取群聊未读消息
func (h *MessageHandler) pullGroupUnread(ctx context.Context, userID string) map[string]*pb.GroupUnreadInfo {
	result := make(map[string]*pb.GroupUnreadInfo)

	// 获取用户所在的所有群
	groups := h.getUserGroups(ctx, userID)

	for _, groupID := range groups {
		streamKey := fmt.Sprintf("stream:group:%s", groupID)

		// 从 Stream 读取所有消息，设置超时时间避免长时间阻塞
		streamCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		messages, err := h.streamOp.ReadMessages(streamCtx, streamKey, "-", 50)
		cancel()
		if err != nil {
			log.Printf("Error reading group %s messages: %v", groupID, err)
			continue
		}

		var pbMessages []*pb.Message
		for _, msg := range messages {
			pbMsg, ok := convertStreamEntryToMessage(msg, "", false)
			if !ok {
				continue
			}
			pbMessages = append(pbMessages, pbMsg)
		}

		if len(pbMessages) > 0 {
			result[groupID] = &pb.GroupUnreadInfo{
				GroupId:     groupID,
				UnreadCount: int32(len(pbMessages)),
				Messages:    pbMessages,
			}
		}

		log.Printf("Group %s: pulled %d unread messages", groupID, len(pbMessages))
	}

	return result
}

// getUserGroups 获取用户所在的所有群
func (h *MessageHandler) getUserGroups(ctx context.Context, userID string) []string {
	// 先尝试从缓存读取
	cachedGroups, hit, _ := h.streamOp.GetCachedUserGroups(ctx, userID)
	if hit {
		return cachedGroups
	}

	// 从数据库读取
	rows, err := h.db.QueryContext(ctx,
		"SELECT group_id FROM group_members WHERE user_id = ? AND is_deleted = 0",
		userID)
	if err != nil {
		log.Printf("Error querying user groups: %v", err)
		return []string{}
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var groupID string
		if err := rows.Scan(&groupID); err != nil {
			continue
		}
		groups = append(groups, groupID)
	}

	// 保存到缓存（包含空结果，避免缓存穿透）
	h.streamOp.CacheUserGroups(ctx, userID, groups)

	return groups
}

func convertDBRowToMessage(id, fromUserID, toUserID, content string, isRead bool, readAt sql.NullString, createdAtStr string) (*pb.Message, error) {
	createdAt, err := time.Parse("2006-01-02 15:04:05", createdAtStr)
	if err != nil {
		return nil, err
	}

	msg := &pb.Message{
		Id:         id,
		FromUserId: fromUserID,
		ToUserId:   toUserID,
		Content:    content,
		IsRead:     isRead,
		CreatedAt:  createdAt.Unix(),
	}

	if readAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", readAt.String); err == nil {
			msg.ReadAt = t.Unix()
		} else {
			return nil, err
		}
	}

	return msg, nil
}

func convertStreamEntryToMessage(entry map[string]string, toUserID string, setTo bool) (*pb.Message, bool) {
	if entry["is_read"] == "true" {
		return nil, false
	}

	var createdAt int64
	if ts, ok := entry["created_at"]; ok {
		if t, err := strconv.ParseInt(ts, 10, 64); err == nil {
			createdAt = t
		}
	}

	msg := &pb.Message{
		Id:         entry["id"],
		FromUserId: entry["from_user_id"],
		Content:    entry["content"],
		IsRead:     false,
		CreatedAt:  createdAt,
	}

	if setTo {
		msg.ToUserId = toUserID
	}

	return msg, true
}

// MarkPrivateMessageAsRead 标记私聊消息为已读
func (h *MessageHandler) MarkPrivateMessageAsRead(ctx context.Context, req *pb.MarkPrivateMessageAsReadRequest) (*pb.MarkPrivateMessageAsReadResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	msgID := req.MessageId

	// 1. 在 Redis Stream 中更新已读状态
	err = h.streamOp.UpdatePrivateMessageAsRead(ctx, userID, msgID)
	if err != nil {
		log.Printf("Warning: failed to update message read status in stream: %v", err)
	}

	// 2. 异步更新数据库
	go func() {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		h.db.ExecContext(dbCtx,
			"UPDATE messages SET is_read = true, read_at = NOW() WHERE id = ? AND to_user_id = ?",
			msgID, userID)
	}()

	log.Printf("Private message %s marked as read by user %s", msgID, userID)

	return &pb.MarkPrivateMessageAsReadResponse{
		Code:    0,
		Message: "消息已标记为已读",
	}, nil
}

// MarkGroupMessageAsRead 标记群聊消息为已读
func (h *MessageHandler) MarkGroupMessageAsRead(ctx context.Context, req *pb.MarkGroupMessageAsReadRequest) (*pb.MarkGroupMessageAsReadResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	groupID := req.GroupId
	lastReadMsgID := req.LastReadMessageId

	// 1. 在 Redis Stream 中更新已读状态
	err = h.streamOp.UpdateGroupMessageAsRead(ctx, groupID, lastReadMsgID)
	if err != nil {
		log.Printf("Warning: failed to update message read status in stream: %v", err)
	}

	// 2. 异步更新数据库
	go func() {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		h.db.ExecContext(dbCtx, `
			INSERT INTO group_read_states (group_id, user_id, last_read_msg_id, last_read_at)
			VALUES (?, ?, ?, NOW())
			ON DUPLICATE KEY UPDATE
				last_read_msg_id = VALUES(last_read_msg_id),
				last_read_at = NOW()
		`, groupID, userID, lastReadMsgID)
	}()

	log.Printf("Group %s messages marked as read by user %s", groupID, userID)

	return &pb.MarkGroupMessageAsReadResponse{
		Code:    0,
		Message: "群聊消息已标记为已读",
	}, nil
}
