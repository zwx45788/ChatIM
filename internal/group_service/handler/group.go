package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	pb "ChatIM/api/proto/group"
	"ChatIM/pkg/auth"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GroupHandler struct {
	pb.UnimplementedGroupServiceServer
	db  *sql.DB
	rdb *redis.Client
}

func NewGroupHandler(db *sql.DB, rdb *redis.Client) *GroupHandler {
	return &GroupHandler{
		db:  db,
		rdb: rdb,
	}
}

// CreateGroup 创建群组
func (h *GroupHandler) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	creatorID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is creating a group: %s", creatorID, req.Name)

	groupID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")

	// 1. 创建群组
	query := `INSERT INTO groups (id, name, description, creator_id, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err = h.db.ExecContext(ctx, query, groupID, req.Name, req.Description, creatorID, createdAt)
	if err != nil {
		log.Printf("Failed to create group: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to create group")
	}

	// 2. 添加创建者为管理员
	_, err = h.db.ExecContext(ctx,
		"INSERT INTO group_members (group_id, user_id, role, joined_at) VALUES (?, ?, 'admin', NOW())",
		groupID, creatorID)
	if err != nil {
		log.Printf("Failed to add creator to group: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to add creator to group")
	}

	// 3. 添加其他成员
	if len(req.MemberIds) > 0 {
		for _, memberID := range req.MemberIds {
			_, err = h.db.ExecContext(ctx,
				"INSERT INTO group_members (group_id, user_id, role, joined_at) VALUES (?, ?, 'member', NOW())",
				groupID, memberID)
			if err != nil {
				log.Printf("Warning: failed to add member %s to group: %v", memberID, err)
			}
		}
	}

	log.Printf("Group %s created successfully", groupID)

	return &pb.CreateGroupResponse{
		Code:    0,
		Message: "群组创建成功",
		GroupId: groupID,
	}, nil
}

// GetGroupInfo 获取群组信息
func (h *GroupHandler) GetGroupInfo(ctx context.Context, req *pb.GetGroupInfoRequest) (*pb.GetGroupInfoResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	// 检查用户是否在群中
	var isMember int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
		req.GroupId, userID).Scan(&isMember)
	if err != nil || isMember == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "User not in group")
	}

	// 查询群组信息
	var group pb.GroupInfo
	var createdAtStr string
	var memberCount int32

	query := `
		SELECT g.id, g.name, g.description, g.creator_id, g.created_at, COUNT(gm.user_id)
		FROM groups g
		LEFT JOIN group_members gm ON g.id = gm.group_id
		WHERE g.id = ?
		GROUP BY g.id`

	err = h.db.QueryRowContext(ctx, query, req.GroupId).Scan(
		&group.Id, &group.Name, &group.Description, &group.CreatorId, &createdAtStr, &memberCount)
	if err != nil {
		log.Printf("Failed to query group info: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query group info")
	}

	createdAt, _ := time.Parse("2006-01-02 15:04:05", createdAtStr)
	group.CreatedAt = createdAt.Unix()
	group.MemberCount = memberCount

	return &pb.GetGroupInfoResponse{
		Code:    0,
		Message: "查询成功",
		Group:   &group,
	}, nil
}

// SendGroupMessage 发送群聊消息
func (h *GroupHandler) SendGroupMessage(ctx context.Context, req *pb.SendGroupMessageRequest) (*pb.SendGroupMessageResponse, error) {
	fromUserID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is sending a message to group %s", fromUserID, req.GroupId)

	// 1. 验证用户是否在群中
	var isMember int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
		req.GroupId, fromUserID).Scan(&isMember)
	if err != nil || isMember == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "User not in group")
	}

	// 2. 保存群消息（msg_index自动递增）
	msgID := uuid.New().String()
	createdAt := time.Now().Format("2006-01-02 15:04:05")
	var msgIndex int64

	query := `INSERT INTO group_messages (id, group_id, from_user_id, content, msg_type, created_at) 
	          VALUES (?, ?, ?, ?, ?, ?)`
	_, err = h.db.ExecContext(ctx, query, msgID, req.GroupId, fromUserID, req.Content, req.MsgType, createdAt)
	if err != nil {
		log.Printf("Failed to insert group message: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to save message")
	}

	// 获取msg_index
	err = h.db.QueryRowContext(ctx,
		"SELECT msg_index FROM group_messages WHERE id = ?", msgID).Scan(&msgIndex)
	if err != nil {
		log.Printf("Failed to get msg_index: %v", err)
	}

	log.Printf("Group message %s saved successfully with index %d", msgID, msgIndex)

	// 3. 发送者自动标记为已读
	h.db.ExecContext(ctx, `
		INSERT INTO group_read_states (group_id, user_id, last_read_msg_index, last_read_msg_id, last_read_at, unread_count, updated_at)
		VALUES (?, ?, ?, ?, NOW(), 0, NOW())
		ON DUPLICATE KEY UPDATE
			last_read_msg_index = VALUES(last_read_msg_index),
			last_read_msg_id = VALUES(last_read_msg_id),
			last_read_at = NOW(),
			unread_count = 0,
			updated_at = NOW()
	`, req.GroupId, fromUserID, msgIndex, msgID)

	// 4. 发布到Pub/Sub
	notificationPayload := map[string]interface{}{
		"group_id":  req.GroupId,
		"msg_id":    msgID,
		"msg_index": msgIndex,
	}
	payloadBytes, err := json.Marshal(notificationPayload)
	if err == nil {
		err = h.rdb.Publish(ctx, "group:"+req.GroupId, payloadBytes).Err()
		if err != nil {
			log.Printf("Warning: failed to publish group message notification: %v", err)
		}
	}

	return &pb.SendGroupMessageResponse{
		Code:    0,
		Message: "群聊消息发送成功",
		Msg: &pb.GroupMessage{
			Id:         msgID,
			GroupId:    req.GroupId,
			FromUserId: fromUserID,
			Content:    req.Content,
			MsgType:    req.MsgType,
			CreatedAt:  time.Now().Unix(),
		},
	}, nil
}

// PullGroupMessages 拉取群聊消息（支持翻页）
func (h *GroupHandler) PullGroupMessages(ctx context.Context, req *pb.PullGroupMessagesRequest) (*pb.PullGroupMessagesResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	// 验证用户是否在群中
	var isMember int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
		req.GroupId, userID).Scan(&isMember)
	if err != nil || isMember == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "User not in group")
	}

	// 查询消息
	query := `
		SELECT id, group_id, from_user_id, content, msg_type, created_at
		FROM group_messages
		WHERE group_id = ?`

	var args []interface{}
	args = append(args, req.GroupId)

	// 支持before_msg_id用于翻页
	if req.BeforeMsgId != "" {
		query += " AND id < (SELECT id FROM group_messages WHERE id = ?)"
		args = append(args, req.BeforeMsgId)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, req.Limit)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("Failed to query group messages: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query messages")
	}
	defer rows.Close()

	var messages []*pb.GroupMessage
	for rows.Next() {
		var msg pb.GroupMessage
		var createdAtStr string

		err := rows.Scan(
			&msg.Id,
			&msg.GroupId,
			&msg.FromUserId,
			&msg.Content,
			&msg.MsgType,
			&createdAtStr,
		)
		if err != nil {
			log.Printf("Failed to scan message row: %v", err)
			continue
		}

		createdAt, _ := time.Parse("2006-01-02 15:04:05", createdAtStr)
		msg.CreatedAt = createdAt.Unix()
		messages = append(messages, &msg)
	}

	return &pb.PullGroupMessagesResponse{
		Code:    0,
		Message: "群聊消息拉取成功",
		Msgs:    messages,
	}, nil
}

// PullGroupUnreadMessages 拉取群聊未读消息
func (h *GroupHandler) PullGroupUnreadMessages(ctx context.Context, req *pb.PullGroupUnreadMessagesRequest) (*pb.PullGroupUnreadMessagesResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	// 验证用户是否在群中
	var isMember int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?",
		req.GroupId, userID).Scan(&isMember)
	if err != nil || isMember == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "User not in group")
	}

	// 查询用户在该群的已读状态
	var lastReadMsgIndex sql.NullInt64
	var lastReadMsgID sql.NullString
	err = h.db.QueryRowContext(ctx,
		"SELECT COALESCE(last_read_msg_index, 0), last_read_msg_id FROM group_read_states WHERE group_id = ? AND user_id = ?",
		req.GroupId, userID).Scan(&lastReadMsgIndex, &lastReadMsgID)

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to query read state: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query read state")
	}

	// 查询未读消息
	query := `
		SELECT id, group_id, from_user_id, content, msg_type, created_at
		FROM group_messages
		WHERE group_id = ?`

	var args []interface{}
	args = append(args, req.GroupId)

	if lastReadMsgIndex.Valid && lastReadMsgIndex.Int64 > 0 {
		query += " AND msg_index > ?"
		args = append(args, lastReadMsgIndex.Int64)
	}

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 100
	}

	query += " ORDER BY created_at ASC LIMIT ?"
	args = append(args, req.Limit)

	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("Failed to query unread messages: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query messages")
	}
	defer rows.Close()

	var messages []*pb.GroupMessage
	var maxMsgIndex int64

	for rows.Next() {
		var msg pb.GroupMessage
		var createdAtStr string
		var msgIndex int64

		err := rows.Scan(
			&msg.Id,
			&msg.GroupId,
			&msg.FromUserId,
			&msg.Content,
			&msg.MsgType,
			&createdAtStr,
		)
		if err != nil {
			log.Printf("Failed to scan message row: %v", err)
			continue
		}

		createdAt, _ := time.Parse("2006-01-02 15:04:05", createdAtStr)
		msg.CreatedAt = createdAt.Unix()
		messages = append(messages, &msg)

		// 记录最大msg_index用于更新已读状态
		err = h.db.QueryRow("SELECT msg_index FROM group_messages WHERE id = ?", msg.Id).Scan(&msgIndex)
		if err == nil && msgIndex > maxMsgIndex {
			maxMsgIndex = msgIndex
		}
	}

	// 更新已读状态
	if len(messages) > 0 && maxMsgIndex > 0 {
		latestMsg := messages[len(messages)-1]
		_, err := h.db.ExecContext(ctx, `
			INSERT INTO group_read_states (group_id, user_id, last_read_msg_index, last_read_msg_id, last_read_at, unread_count, updated_at)
			VALUES (?, ?, ?, ?, NOW(), 0, NOW())
			ON DUPLICATE KEY UPDATE
				last_read_msg_index = VALUES(last_read_msg_index),
				last_read_msg_id = VALUES(last_read_msg_id),
				last_read_at = NOW(),
				unread_count = 0,
				updated_at = NOW()
		`, req.GroupId, userID, maxMsgIndex, latestMsg.Id)

		if err != nil {
			log.Printf("Warning: failed to update read state: %v", err)
		}
	}

	log.Printf("User %s pulled %d unread messages from group %s", userID, len(messages), req.GroupId)

	return &pb.PullGroupUnreadMessagesResponse{
		Code:        0,
		Message:     "成功拉取未读消息",
		Msgs:        messages,
		TotalUnread: int32(len(messages)),
	}, nil
}

// GetGroupUnreadCount 获取群聊未读数
func (h *GroupHandler) GetGroupUnreadCount(ctx context.Context, req *pb.GetGroupUnreadCountRequest) (*pb.GetGroupUnreadCountResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	var unreadCount int32
	query := `SELECT COALESCE(unread_count, 0) FROM group_read_states WHERE group_id = ? AND user_id = ?`
	err = h.db.QueryRowContext(ctx, query, req.GroupId, userID).Scan(&unreadCount)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Failed to query unread count: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query unread count")
	}

	return &pb.GetGroupUnreadCountResponse{
		Code:        0,
		Message:     "查询成功",
		UnreadCount: unreadCount,
	}, nil
}

// AddGroupMember 添加群成员
func (h *GroupHandler) AddGroupMember(ctx context.Context, req *pb.AddGroupMemberRequest) (*pb.AddGroupMemberResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	// 检查操作者是否是群主
	var isAdmin int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND role = 'admin'",
		req.GroupId, userID).Scan(&isAdmin)
	if err != nil || isAdmin == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "Only admin can add members")
	}

	addedCount := 0
	for _, memberID := range req.UserIds {
		_, err = h.db.ExecContext(ctx,
			"INSERT IGNORE INTO group_members (group_id, user_id, role, joined_at) VALUES (?, ?, 'member', NOW())",
			req.GroupId, memberID)
		if err == nil {
			addedCount++
		}
	}

	return &pb.AddGroupMemberResponse{
		Code:       0,
		Message:    "添加成员成功",
		AddedCount: int32(addedCount),
	}, nil
}

// RemoveGroupMember 移除群成员
func (h *GroupHandler) RemoveGroupMember(ctx context.Context, req *pb.RemoveGroupMemberRequest) (*pb.RemoveGroupMemberResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	// 检查操作者是否是群主
	var isAdmin int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND role = 'admin'",
		req.GroupId, userID).Scan(&isAdmin)
	if err != nil || isAdmin == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "Only admin can remove members")
	}

	removedCount := 0
	for _, memberID := range req.UserIds {
		result, err := h.db.ExecContext(ctx,
			"DELETE FROM group_members WHERE group_id = ? AND user_id = ?",
			req.GroupId, memberID)
		if err == nil {
			affected, _ := result.RowsAffected()
			if affected > 0 {
				removedCount++
			}
		}
	}

	return &pb.RemoveGroupMemberResponse{
		Code:         0,
		Message:      "移除成员成功",
		RemovedCount: int32(removedCount),
	}, nil
}

// LeaveGroup 离开群组
func (h *GroupHandler) LeaveGroup(ctx context.Context, req *pb.LeaveGroupRequest) (*pb.LeaveGroupResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	result, err := h.db.ExecContext(ctx,
		"DELETE FROM group_members WHERE group_id = ? AND user_id = ?",
		req.GroupId, userID)
	if err != nil {
		log.Printf("Failed to leave group: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to leave group")
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, status.Errorf(codes.NotFound, "User not in group")
	}

	return &pb.LeaveGroupResponse{
		Code:    0,
		Message: "离开群组成功",
	}, nil
}

// ListGroups 列出用户的所有群组
func (h *GroupHandler) ListGroups(ctx context.Context, req *pb.ListGroupsRequest) (*pb.ListGroupsResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 100
	}

	// 查询用户所在的群组
	query := `
		SELECT g.id, g.name, g.description, g.creator_id, g.created_at, COUNT(gm.user_id)
		FROM groups g
		INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE g.id IN (SELECT group_id FROM group_members WHERE user_id = ?)
		GROUP BY g.id
		ORDER BY g.created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query, userID, req.Limit, req.Offset)
	if err != nil {
		log.Printf("Failed to query groups: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query groups")
	}
	defer rows.Close()

	var groups []*pb.GroupInfo
	for rows.Next() {
		var group pb.GroupInfo
		var createdAtStr string
		var memberCount int32

		err := rows.Scan(
			&group.Id,
			&group.Name,
			&group.Description,
			&group.CreatorId,
			&createdAtStr,
			&memberCount,
		)
		if err != nil {
			log.Printf("Failed to scan group row: %v", err)
			continue
		}

		createdAt, _ := time.Parse("2006-01-02 15:04:05", createdAtStr)
		group.CreatedAt = createdAt.Unix()
		group.MemberCount = memberCount
		groups = append(groups, &group)
	}

	// 获取总数
	var total int32
	h.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT g.id) FROM groups g INNER JOIN group_members gm ON g.id = gm.group_id WHERE gm.user_id = ?",
		userID).Scan(&total)

	return &pb.ListGroupsResponse{
		Code:    0,
		Message: "查询成功",
		Groups:  groups,
		Total:   total,
	}, nil
}

// PullAllGroupsUnreadMessages 拉取用户所有群的未读消息（用于上线同步）
func (h *GroupHandler) PullAllGroupsUnreadMessages(ctx context.Context, req *pb.PullAllGroupsUnreadMessagesRequest) (*pb.PullAllGroupsUnreadMessagesResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("User %s is pulling all groups unread messages", userID)

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20 // 默认每个群拉取20条未读消息
	}

	// 1. 查询用户所有的群
	groupRows, err := h.db.QueryContext(ctx, `
		SELECT DISTINCT g.id, g.name FROM groups g
		INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = ? AND g.is_deleted = false
	`, userID)
	if err != nil {
		log.Printf("Failed to query user's groups: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to query groups")
	}
	defer groupRows.Close()

	var groupUnreads []*pb.GroupUnreadInfo
	var totalUnreadCount int32

	// 2. 遍历每个群，查询未读消息
	for groupRows.Next() {
		var groupID, groupName string
		if err := groupRows.Scan(&groupID, &groupName); err != nil {
			log.Printf("Failed to scan group row: %v", err)
			continue
		}

		// 查询用户在该群的已读状态
		var lastReadMsgIndex sql.NullInt64
		err := h.db.QueryRowContext(ctx,
			"SELECT COALESCE(last_read_msg_index, 0) FROM group_read_states WHERE group_id = ? AND user_id = ?",
			groupID, userID).Scan(&lastReadMsgIndex)

		if err != nil && err != sql.ErrNoRows {
			log.Printf("Failed to query read state for group %s: %v", groupID, err)
			continue
		}

		// 查询未读消息
		query := `
			SELECT id, group_id, from_user_id, content, msg_type, created_at
			FROM group_messages
			WHERE group_id = ?`

		var args []interface{}
		args = append(args, groupID)

		if lastReadMsgIndex.Valid && lastReadMsgIndex.Int64 > 0 {
			query += " AND msg_index > ?"
			args = append(args, lastReadMsgIndex.Int64)
		}

		query += " ORDER BY created_at ASC LIMIT ?"
		args = append(args, req.Limit)

		rows, err := h.db.QueryContext(ctx, query, args...)
		if err != nil {
			log.Printf("Failed to query unread messages for group %s: %v", groupID, err)
			continue
		}

		var messages []*pb.GroupMessage
		var unreadCount int32 = 0
		var maxMsgIndex int64

		for rows.Next() {
			var msg pb.GroupMessage
			var createdAtStr string

			err := rows.Scan(
				&msg.Id,
				&msg.GroupId,
				&msg.FromUserId,
				&msg.Content,
				&msg.MsgType,
				&createdAtStr,
			)
			if err != nil {
				log.Printf("Failed to scan message row: %v", err)
				continue
			}

			createdAt, _ := time.Parse("2006-01-02 15:04:05", createdAtStr)
			msg.CreatedAt = createdAt.Unix()
			messages = append(messages, &msg)
			unreadCount++

			// 记录最大msg_index用于更新已读状态
			var msgIndex int64
			h.db.QueryRow("SELECT msg_index FROM group_messages WHERE id = ?", msg.Id).Scan(&msgIndex)
			if msgIndex > maxMsgIndex {
				maxMsgIndex = msgIndex
			}
		}
		rows.Close()

		// 更新该群的已读状态
		if unreadCount > 0 && maxMsgIndex > 0 {
			latestMsg := messages[len(messages)-1]
			_, err := h.db.ExecContext(ctx, `
				INSERT INTO group_read_states (group_id, user_id, last_read_msg_index, last_read_msg_id, last_read_at, unread_count, updated_at)
				VALUES (?, ?, ?, ?, NOW(), 0, NOW())
				ON DUPLICATE KEY UPDATE
					last_read_msg_index = VALUES(last_read_msg_index),
					last_read_msg_id = VALUES(last_read_msg_id),
					last_read_at = NOW(),
					unread_count = 0,
					updated_at = NOW()
			`, groupID, userID, maxMsgIndex, latestMsg.Id)

			if err != nil {
				log.Printf("Warning: failed to update read state for group %s: %v", groupID, err)
			}
		}

		// 如果有未读消息，添加到响应中
		if unreadCount > 0 {
			groupUnreads = append(groupUnreads, &pb.GroupUnreadInfo{
				GroupId:        groupID,
				GroupName:      groupName,
				UnreadCount:    unreadCount,
				LatestMessages: messages,
			})
			totalUnreadCount += unreadCount
		}
	}

	log.Printf("User %s pulled unread messages from %d groups, total unread: %d", userID, len(groupUnreads), totalUnreadCount)

	return &pb.PullAllGroupsUnreadMessagesResponse{
		Code:             0,
		Message:          "成功拉取所有群未读消息",
		GroupUnreads:     groupUnreads,
		TotalUnreadCount: totalUnreadCount,
	}, nil
}
