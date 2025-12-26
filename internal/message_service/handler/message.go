package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ChatIM/api/proto/message"
	"ChatIM/pkg/auth"
	"ChatIM/pkg/logger"
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
	logger.Info("Sending private message",
		zap.String("from_user_id", fromUserID),
		zap.String("to_user_id", req.ToUserId))

	msgID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// 1. 立即写入 Redis Stream（快速响应）
	_, err = h.streamOp.AddPrivateMessage(ctx, msgID, fromUserID, req.ToUserId, req.Content)
	if err != nil {
		logger.Error("Failed to add private message to stream", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to save message")
	}

	// 2. 更新双方的会话列表
	conversationID := fmt.Sprintf("private:%s", req.ToUserId)
	h.streamOp.UpdateConversationTime(ctx, fromUserID, fmt.Sprintf("private:%s", fromUserID))
	h.streamOp.UpdateConversationTime(ctx, req.ToUserId, conversationID)

	// 3. 发布消息通知到 Redis（通知 WebSocket 推送，包括发送者自己用于多设备同步）
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// 给接收者发送通知
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
			logger.Warn("Failed to marshal notification", zap.Error(err))
			return
		}

		err = h.rdb.Publish(notificationCtx, "message_notifications", notificationJSON).Err()
		if err != nil {
			logger.Warn("Failed to publish notification", zap.Error(err))
		} else {
			logger.Debug("Notification published",
				zap.String("msg_id", msgID),
				zap.String("to_user_id", req.ToUserId))
		}

		// 也给发送者发送通知（用于多设备同步/消息回显）
		// senderNotification := map[string]interface{}{
		// 	"msg_id":       msgID,
		// 	"to_user_id":   fromUserID,
		// 	"from_user_id": fromUserID,
		// 	"type":         "private",
		// 	"content":      req.Content,
		// 	"created_at":   time.Now().Unix(),
		// 	"is_sender":    true, // 标记这是发送者自己的消息
		// }

		// senderNotificationJSON, err := json.Marshal(senderNotification)
		// if err == nil {
		// 	h.rdb.Publish(notificationCtx, "message_notifications", senderNotificationJSON).Err()
		// }
	}()

	// 4. 异步写入数据库（不阻塞用户）
	go func() {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := `INSERT INTO messages (id, from_user_id, to_user_id, content, created_at) VALUES (?, ?, ?, ?, ?)`
		_, err := h.db.ExecContext(dbCtx, query, msgID, fromUserID, req.ToUserId, req.Content, createdAt)
		if err != nil {
			logger.Warn("Failed to save message to database", zap.Error(err))
		} else {
			logger.Debug("Message saved to database", zap.String("msg_id", msgID))
		}
	}()

	logger.Info("Message sent successfully", zap.String("msg_id", msgID))

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

	logger.Info("Sending group message",
		zap.String("from_user_id", fromUserID),
		zap.String("group_id", req.GroupId))

	msgID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// 1. 查询群成员列表
	memberIDs, err := h.getGroupMembers(ctx, req.GroupId)
	if err != nil {
		logger.Error("Failed to get group members", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to get group members")
	}

	if len(memberIDs) == 0 {
		return nil, status.Errorf(codes.NotFound, "Group has no members")
	}

	// 2. 写入所有成员的 Redis Stream (统一使用 stream:private:{user_id})
	err = h.streamOp.AddGroupMessageToMembers(ctx, msgID, req.GroupId, fromUserID, req.Content, "text", memberIDs)
	if err != nil {
		logger.Error("Failed to add group message to members' streams", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to save group message")
	}

	// 3. 更新所有成员的会话列表
	conversationID := fmt.Sprintf("group:%s", req.GroupId)
	for _, memberID := range memberIDs {
		h.streamOp.UpdateConversationTime(ctx, memberID, conversationID)
	}

	// 4. 发布群消息通知到 Redis（通知所有在线成员，包括发送者用于多设备同步）
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// 给每个成员发送通知
		for _, memberID := range memberIDs {
			if memberID == fromUserID {
				continue
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

			// // 标记发送者自己的消息
			// if memberID == fromUserID {
			// 	notification["is_sender"] = true
			// }

			notificationJSON, err := json.Marshal(notification)
			if err != nil {
				logger.Warn("Failed to marshal notification for member",
					zap.String("member_id", memberID),
					zap.Error(err))
				continue
			}

			err = h.rdb.Publish(notificationCtx, "message_notifications", notificationJSON).Err()
			if err != nil {
				logger.Warn("Failed to publish notification to member",
					zap.String("member_id", memberID),
					zap.Error(err))
			}
		}

		logger.Debug("Notifications published for group message",
			zap.String("msg_id", msgID),
			zap.Int("member_count", len(memberIDs)))
	}()

	// 5. 异步写入数据库
	go func() {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := `INSERT INTO group_messages (id, group_id, from_user_id, content, created_at) VALUES (?, ?, ?, ?, ?)`
		_, err := h.db.ExecContext(dbCtx, query, msgID, req.GroupId, fromUserID, req.Content, createdAt)
		if err != nil {
			logger.Warn("Failed to save group message to database", zap.Error(err))
		} else {
			logger.Debug("Group message saved to database", zap.String("msg_id", msgID))
		}
	}()

	logger.Info("Group message sent",
		zap.String("msg_id", msgID),
		zap.Int("member_count", len(memberIDs)))

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
		logger.Error("Error querying group members", zap.Error(err))
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

// PullMessages 拉取按会话分组的消息（基于游标的增量拉取）
func (h *MessageHandler) PullMessages(ctx context.Context, req *pb.PullMessagesRequest) (*pb.PullMessagesResponse, error) {
	// 1. 获取当前用户 ID
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	logger.Info("Pulling messages",
		zap.String("user_id", userID),
		zap.String("from_stream_id", req.FromStreamId))

	// 设置默认值
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// 2. 确定起始游标
	startCursor := req.FromStreamId
	if startCursor == "" {
		cursor, err := h.streamOp.GetUserCursor(ctx, userID)
		if err != nil {
			logger.Warn("Failed to get user cursor, fallback to beginning", zap.Error(err))
			cursor = "0-0"
		}
		startCursor = cursor
	}

	// 3. 从 Redis Stream 读取消息（增量拉取）
	streamKey := fmt.Sprintf("stream:private:%s", userID)

	// 使用 XRange 进行增量拉取：从 startCursor 之后开始
	messages, err := h.rdb.XRange(ctx, streamKey, "("+startCursor, "+").Result()
	if err != nil {
		logger.Warn("Failed to read from stream", zap.Error(err))
		messages = []redis.XMessage{} // 容错处理
	}

	// 4. 按会话分组消息
	conversationMap := make(map[string]*pb.ConversationMessages)

	for _, msg := range messages {
		msgType, _ := msg.Values["type"].(string)

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
			IsRead:     false, // 新拉取的消息默认未读
			StreamId:   msg.ID,
		}

		conv.Messages = append(conv.Messages, unifiedMsg)
		conv.UnreadCount++

		// 更新最后消息时间
		if unifiedMsg.CreatedAt > conv.LastMessageTime {
			conv.LastMessageTime = unifiedMsg.CreatedAt
		}
	}

	// 5. 转换为数组并按最后消息时间排序
	var conversations []*pb.ConversationMessages
	var totalUnread int32

	for _, conv := range conversationMap {
		// 补充用户/群组信息
		h.enrichConversationInfo(ctx, conv)
		conversations = append(conversations, conv)
		totalUnread += conv.UnreadCount
	}

	// 按最后消息时间降序排序
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].LastMessageTime > conversations[j].LastMessageTime
	})

	logger.Info("Messages pulled",
		zap.String("user_id", userID),
		zap.Int("conversation_count", len(conversations)),
		zap.Int32("total_unread", totalUnread),
		zap.Int("total_messages", len(messages)))

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
			logger.Warn("Failed to enrich private conversation",
				zap.String("peer_id", conv.PeerId),
				zap.Error(err))
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
			logger.Warn("Failed to enrich group conversation",
				zap.String("peer_id", conv.PeerId),
				zap.Error(err))
		}

	default:
		// 未知会话类型，记录日志
		logger.Warn("Unknown conversation type", zap.String("type", conv.Type))
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

// GetUnreadCount 获取用户的未读消息数 [DEPRECATED]
func (h *MessageHandler) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	logger.Debug("GetUnreadCount called but deprecated")

	return &pb.GetUnreadCountResponse{
		Code:        0,
		Message:     "此接口已弃用，请使用基于游标的拉取方式",
		UnreadCount: 0,
	}, nil
}

// UpdateLastSeenCursor 更新会话的已读游标
func (h *MessageHandler) UpdateLastSeenCursor(ctx context.Context, req *pb.UpdateLastSeenCursorRequest) (*pb.UpdateLastSeenCursorResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	if req.LastSeenStreamId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "last_seen_stream_id is required")
	}

	logger.Info("Updating last seen cursor",
		zap.String("user_id", userID),
		zap.String("type", req.ConversationType),
		zap.String("peer_id", req.PeerId),
		zap.String("cursor", req.LastSeenStreamId))

	if err := h.streamOp.SetUserCursor(ctx, userID, req.LastSeenStreamId); err != nil {
		logger.Error("Failed to set user cursor", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "Failed to update cursor")
	}

	// 兼容群聊的数据库已读状态同步
	switch req.ConversationType {
	case "group":
		if req.PeerId == "" {
			return nil, status.Errorf(codes.InvalidArgument, "peer_id (group_id) is required for group conversation")
		}

		go func() {
			dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			h.db.ExecContext(dbCtx, `
				INSERT INTO group_read_states (group_id, user_id, last_read_msg_id, last_read_at)
				VALUES (?, ?, ?, NOW())
				ON DUPLICATE KEY UPDATE
					last_read_msg_id = VALUES(last_read_msg_id),
					last_read_at = NOW()
			`, req.PeerId, userID, req.LastSeenStreamId)
		}()

	case "", "private":
		// 私聊或未指定类型无需额外处理
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid conversation_type: %s", req.ConversationType)
	}

	cursor, _ := h.streamOp.GetUserCursor(ctx, userID)

	logger.Info("Cursor updated successfully",
		zap.String("user_id", userID),
		zap.String("type", req.ConversationType),
		zap.String("cursor", cursor))

	return &pb.UpdateLastSeenCursorResponse{
		Code:    0,
		Message: "游标更新成功",
		Cursor:  cursor,
	}, nil
}

// PullUnreadMessages 拉取所有未读消息

// PullAllUnreadOnLogin 登录时拉取所有未读消息（私聊 + 群聊）

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

// MarkPrivateMessageAsRead 标记私聊消息为已读（仅更新数据库）
// 注意：此方法不更新游标，游标应通过 UpdateLastSeenCursor 方法统一管理
func (h *MessageHandler) MarkPrivateMessageAsRead(ctx context.Context, req *pb.MarkPrivateMessageAsReadRequest) (*pb.MarkPrivateMessageAsReadResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	msgID := req.MessageId
	if msgID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "message_id is required")
	}

	logger.Debug("Marking private message as read",
		zap.String("msg_id", msgID),
		zap.String("user_id", userID))

	// 异步更新数据库
	go func() {
		dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		h.db.ExecContext(dbCtx,
			"UPDATE messages SET is_read = true, read_at = NOW() WHERE id = ? AND to_user_id = ?",
			msgID, userID)
	}()

	logger.Debug("Private message marked as read",
		zap.String("msg_id", msgID),
		zap.String("user_id", userID))

	return &pb.MarkPrivateMessageAsReadResponse{
		Code:    0,
		Message: "消息已标记为已读",
	}, nil
}

// MarkGroupMessageAsRead 标记群聊消息为已读（仅更新数据库）
// 注意：此方法不更新游标，游标应通过 UpdateLastSeenCursor 方法统一管理
func (h *MessageHandler) MarkGroupMessageAsRead(ctx context.Context, req *pb.MarkGroupMessageAsReadRequest) (*pb.MarkGroupMessageAsReadResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	groupID := req.GroupId
	lastReadMsgID := req.LastReadMessageId

	if groupID == "" || lastReadMsgID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "group_id and last_read_message_id are required")
	}

	logger.Debug("Marking group messages as read",
		zap.String("group_id", groupID),
		zap.String("last_msg_id", lastReadMsgID),
		zap.String("user_id", userID))

	// 异步更新数据库
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

	logger.Debug("Group messages marked as read",
		zap.String("group_id", groupID),
		zap.String("user_id", userID))

	return &pb.MarkGroupMessageAsReadResponse{
		Code:    0,
		Message: "群聊消息已标记为已读",
	}, nil
}
