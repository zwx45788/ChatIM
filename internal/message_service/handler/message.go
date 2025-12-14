package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
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

	// 3. 发布消息通知到 Redis（通知 WebSocket 推送）
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		notification := map[string]interface{}{
			"msg_id":       msgID,
			"to_user_id":   req.ToUserId,
			"from_user_id": fromUserID,
			"type":         "private",
			"content":      req.Content,
			"created_at":   time.Now().Unix(),
		}

		notificationJSON, err := json.Marshal(notification)
		if err != nil {
			log.Printf("Warning: failed to marshal notification: %v", err)
			return
		}

		err = h.rdb.Publish(notificationCtx, "message_notifications", notificationJSON).Err()
		if err != nil {
			log.Printf("Warning: failed to publish notification: %v", err)
		} else {
			log.Printf("✅ Notification published for message %s to user %s", msgID, req.ToUserId)
		}
	}()

	// 4. 异步写入数据库（不阻塞用户）
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

	// 4. 发布群消息通知到 Redis（通知所有在线成员）
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// 给每个成员（除了发送者）发送通知
		for _, memberID := range memberIDs {
			if memberID == fromUserID {
				continue // 跳过发送者本人
			}

			notification := map[string]interface{}{
				"msg_id":       msgID,
				"to_user_id":   memberID,
				"from_user_id": fromUserID,
				"group_id":     req.GroupId,
				"type":         "group",
				"content":      req.Content,
				"created_at":   time.Now().Unix(),
			}

			notificationJSON, err := json.Marshal(notification)
			if err != nil {
				log.Printf("Warning: failed to marshal notification for member %s: %v", memberID, err)
				continue
			}

			err = h.rdb.Publish(notificationCtx, "message_notifications", notificationJSON).Err()
			if err != nil {
				log.Printf("Warning: failed to publish notification to member %s: %v", memberID, err)
			}
		}

		log.Printf("✅ Notifications published for group message %s to %d members", msgID, len(memberIDs)-1)
	}()

	// 5. 异步写入数据库
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

// PullMessages 拉取按会话分组的消息（优先从 Redis Stream 读取，支持私聊和群聊）
func (h *MessageHandler) PullMessages(ctx context.Context, req *pb.PullMessagesRequest) (*pb.PullMessagesResponse, error) {
	// 1. 获取当前用户 ID
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is pulling messages (grouped by conversation)", userID)

	// 设置默认值
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 2. 从 Redis Stream 读取消息（优先读取最新消息）
	streamKey := fmt.Sprintf("stream:private:%s", userID)
	messages, err := h.rdb.XRevRangeN(ctx, streamKey, "+", "-", 500).Result()
	if err != nil {
		log.Printf("Warning: failed to read from stream: %v", err)
	}

	// 3. 按会话分组消息
	conversationMap := make(map[string]*pb.ConversationMessages)
	var totalUnread int32

	for _, msg := range messages {
		msgType, _ := msg.Values["type"].(string)
		isRead := h.isMessageRead(msg.Values)

		// 根据 include_read 参数过滤
		if !req.IncludeRead && isRead {
			continue
		}

		var conversationID string
		var peerID string
		var convType string

		switch msgType {
		case "private":
			fromUserID := getString(msg.Values["from_user_id"])
			toUserID := getString(msg.Values["to_user_id"])

			// 确定对方ID
			if fromUserID == userID {
				peerID = toUserID
			} else {
				peerID = fromUserID
			}

			conversationID = fmt.Sprintf("private:%s", peerID)
			convType = "private"

		case "group":
			groupID := getString(msg.Values["group_id"])
			conversationID = fmt.Sprintf("group:%s", groupID)
			peerID = groupID
			convType = "group"

		default:
			// 未知消息类型，跳过处理
			continue
		}

		// 初始化会话
		if _, exists := conversationMap[conversationID]; !exists {
			conversationMap[conversationID] = &pb.ConversationMessages{
				ConversationId: conversationID,
				Type:           convType,
				PeerId:         peerID,
				Messages:       []*pb.UnifiedMessage{},
			}
		}

		conv := conversationMap[conversationID]

		// 限制每个会话的消息数量
		if int64(len(conv.Messages)) >= limit {
			continue
		}

		// 添加消息
		unifiedMsg := &pb.UnifiedMessage{
			Id:         getString(msg.Values["msg_id"]),
			Type:       msgType,
			FromUserId: getString(msg.Values["from_user_id"]),
			Content:    getString(msg.Values["content"]),
			CreatedAt:  getInt64(msg.Values["created_at"]),
			IsRead:     isRead,
			StreamId:   msg.ID,
		}

		conv.Messages = append(conv.Messages, unifiedMsg)

		// 更新未读计数
		if !isRead {
			conv.UnreadCount++
			totalUnread++
		}

		// 更新最后消息时间
		if unifiedMsg.CreatedAt > conv.LastMessageTime {
			conv.LastMessageTime = unifiedMsg.CreatedAt
		}
	}

	// 4. 转换为数组并按最后消息时间排序
	var conversations []*pb.ConversationMessages
	for _, conv := range conversationMap {
		// 补充用户/群组信息
		h.enrichConversationInfo(ctx, conv)
		conversations = append(conversations, conv)
	}

	// 按最后消息时间降序排序
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].LastMessageTime > conversations[j].LastMessageTime
	})

	// 注意：移除了自动标记已读逻辑，前端需要在成功接收消息后主动调用标记已读接口

	log.Printf("✅ User %s pulled %d conversations with %d total unread messages", userID, len(conversations), totalUnread)

	return &pb.PullMessagesResponse{
		Code:              0,
		Message:           "消息拉取成功",
		Conversations:     conversations,
		TotalUnread:       totalUnread,
		ConversationCount: int32(len(conversations)),
	}, nil
}

// isMessageRead 判断消息是否已读
func (h *MessageHandler) isMessageRead(values map[string]interface{}) bool {
	if isReadStr, ok := values["is_read"].(string); ok {
		return isReadStr == "true" || isReadStr == "1"
	}
	return false
}

// enrichConversationInfo 补充会话信息（用户昵称、头像等）
func (h *MessageHandler) enrichConversationInfo(ctx context.Context, conv *pb.ConversationMessages) {
	switch conv.Type {
	case "private":
		// 查询用户信息
		var name, avatar string
		query := `SELECT username, avatar FROM users WHERE id = ?`
		err := h.db.QueryRowContext(ctx, query, conv.PeerId).Scan(&name, &avatar)
		if err == nil {
			conv.PeerName = name
			conv.PeerAvatar = avatar
		} else {
			// 可以考虑记录日志，方便排查问题
			log.Printf("Warning: failed to enrich private conversation %s: %v", conv.PeerId, err)
		}

	case "group":
		// 查询群组信息
		var name, avatar string
		query := `SELECT name, avatar FROM groups WHERE id = ?`
		err := h.db.QueryRowContext(ctx, query, conv.PeerId).Scan(&name, &avatar)
		if err == nil {
			conv.PeerName = name
			conv.PeerAvatar = avatar
		} else {
			log.Printf("Warning: failed to enrich group conversation %s: %v", conv.PeerId, err)
		}

	default:
		// 未知会话类型，记录日志
		log.Printf("Warning: unknown conversation type: %s", conv.Type)
	}

	// 补充每条消息的发送者昵称
	for _, msg := range conv.Messages {
		var senderName string
		query := `SELECT username FROM users WHERE id = ?`
		err := h.db.QueryRowContext(ctx, query, msg.FromUserId).Scan(&senderName)
		if err == nil {
			msg.FromUserName = senderName
		}
	}
}

// getString 辅助函数：从 interface{} 提取字符串
func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// getInt64 辅助函数：从 interface{} 提取 int64
func getInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case float64:
		return int64(val)
	}
	return 0
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

// PullUnreadMessages 拉取所有未读消息
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
		messages []*pb.Message

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

	// 注意：移除了自动标记已读逻辑，前端需要在成功接收消息后主动调用标记已读接口

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
// 注意：移除了自动标记已读逻辑，前端需要在成功接收消息后主动调用标记已读接口
func (h *MessageHandler) PullAllUnreadOnLogin(ctx context.Context, req *pb.PullAllUnreadOnLoginRequest) (*pb.PullAllUnreadOnLoginResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is pulling all unread messages on login", userID)

	// 1. 记录用户上线时间
	h.streamOp.RecordUserOnlineTime(ctx, userID)

	// 2. 从用户的 Redis Stream 读取所有未读消息（私聊和群聊都在 stream:private:{user_id} 中）
	streamKey := fmt.Sprintf("stream:private:%s", userID)
	messages, err := h.rdb.XRevRangeN(ctx, streamKey, "+", "-", 1000).Result()
	if err != nil {
		log.Printf("Warning: failed to read from stream: %v", err)
	}

	// 3. 按消息类型分组
	var privateMessages []*pb.Message
	groupMessages := make(map[string]*pb.GroupUnreadInfo)

	for _, msg := range messages {
		msgType, _ := msg.Values["type"].(string)
		isRead := h.isMessageRead(msg.Values)

		// 只处理未读消息
		if isRead {
			continue
		}

		switch msgType {
		case "private":
			// 私聊消息
			pbMsg := &pb.Message{
				Id:         getString(msg.Values["msg_id"]),
				FromUserId: getString(msg.Values["from_user_id"]),
				ToUserId:   getString(msg.Values["to_user_id"]),
				Content:    getString(msg.Values["content"]),
				IsRead:     false,
				CreatedAt:  getInt64(msg.Values["created_at"]),
			}
			privateMessages = append(privateMessages, pbMsg)

		case "group":
			// 群聊消息
			groupID := getString(msg.Values["group_id"])
			pbMsg := &pb.Message{
				Id:         getString(msg.Values["msg_id"]),
				FromUserId: getString(msg.Values["from_user_id"]),
				Content:    getString(msg.Values["content"]),
				IsRead:     false,
				CreatedAt:  getInt64(msg.Values["created_at"]),
			}

			if _, exists := groupMessages[groupID]; !exists {
				groupMessages[groupID] = &pb.GroupUnreadInfo{
					GroupId:     groupID,
					UnreadCount: 0,
					Messages:    []*pb.Message{},
				}
			}
			groupMessages[groupID].Messages = append(groupMessages[groupID].Messages, pbMsg)
			groupMessages[groupID].UnreadCount++
		}
	}

	// 4. 计算总数
	totalCount := int32(len(privateMessages))
	for _, detail := range groupMessages {
		totalCount += detail.UnreadCount
	}

	log.Printf("User %s pulled %d private unread and %d group conversations with total %d unread messages",
		userID, len(privateMessages), len(groupMessages), totalCount)

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

// convertDBRowToMessage 辅助函数：将数据库行转换为 Message 对象
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
