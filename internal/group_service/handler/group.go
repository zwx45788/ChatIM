package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	pb "ChatIM/api/proto/group"
	"ChatIM/pkg/auth"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GroupHandler struct {
	pb.UnimplementedGroupServiceServer
	db *sql.DB
}

func NewGroupHandler(db *sql.DB) *GroupHandler {
	return &GroupHandler{
		db: db,
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
	query := "INSERT INTO `groups` (id, name, description, creator_id, created_at) VALUES (?, ?, ?, ?, ?)"
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
		FROM ` + "`groups`" + ` g
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
		FROM ` + "`groups`" + ` g
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
		"SELECT COUNT(DISTINCT g.id) FROM `groups` g INNER JOIN group_members gm ON g.id = gm.group_id WHERE gm.user_id = ?",
		userID).Scan(&total)

	return &pb.ListGroupsResponse{
		Code:    0,
		Message: "查询成功",
		Groups:  groups,
		Total:   total,
	}, nil
}

// ==================== 群加入请求处理 ====================

// SendGroupJoinRequest 发送群加入请求
func (h *GroupHandler) SendGroupJoinRequest(ctx context.Context, req *pb.SendGroupJoinRequestRequest) (*pb.SendGroupJoinRequestResponse, error) {
	fromUserID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s sending group join request to group %s", fromUserID, req.GroupId)

	// 1. 验证群组是否存在
	var groupExists int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM `groups` WHERE id = ? AND is_deleted = 0",
		req.GroupId).Scan(&groupExists)
	if err != nil || groupExists == 0 {
		return nil, status.Errorf(codes.NotFound, "群组不存在")
	}

	// 2. 检查是否已经是群成员
	var isMember int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		req.GroupId, fromUserID).Scan(&isMember)
	if err != nil {
		log.Printf("Error checking membership: %v", err)
		return nil, status.Errorf(codes.Internal, "检查群成员失败")
	}
	if isMember > 0 {
		return nil, status.Errorf(codes.AlreadyExists, "已经是群成员")
	}

	// 3. 检查是否已有申请记录（无论状态如何）
	var existingReqID string
	var existingStatus sql.NullString
	err = h.db.QueryRowContext(ctx,
		"SELECT id, status FROM group_join_requests WHERE group_id = ? AND from_user_id = ?",
		req.GroupId, fromUserID).Scan(&existingReqID, &existingStatus)

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking existing requests: %v", err)
		return nil, status.Errorf(codes.Internal, "检查申请状态失败: %v", err)
	}

	if err == nil {
		statusStr := ""
		if existingStatus.Valid {
			statusStr = existingStatus.String
		}
		// 存在历史记录
		if statusStr == "pending" {
			return nil, status.Errorf(codes.AlreadyExists, "已发送过申请，请等待处理")
		}

		// 如果是其他状态（rejected, accepted, cancelled），则更新为 pending 并更新消息和时间
		// 注意：这里我们复用旧的 ID，或者也可以选择删除旧的插入新的。复用旧 ID 比较简单。
		// 更新 created_at 以便在按时间排序时能排在前面
		_, err = h.db.ExecContext(ctx,
			"UPDATE group_join_requests SET message = ?, status = 'pending', created_at = NOW() WHERE id = ?",
			req.Message, existingReqID)
		if err != nil {
			log.Printf("Failed to update group join request: %v", err)
			return nil, status.Errorf(codes.Internal, "更新申请失败: %v", err)
		}

		log.Printf("Group join request %s updated successfully (re-applied)", existingReqID)

		return &pb.SendGroupJoinRequestResponse{
			Code:      0,
			Message:   "加群申请已发送",
			RequestId: existingReqID,
		}, nil
	}

	// 4. 创建新加群申请
	requestID := uuid.New().String()
	query := `INSERT INTO group_join_requests (id, group_id, from_user_id, message, status, created_at) 
	          VALUES (?, ?, ?, ?, 'pending', NOW())`
	_, err = h.db.ExecContext(ctx, query, requestID, req.GroupId, fromUserID, req.Message)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, status.Errorf(codes.AlreadyExists, "已发送过申请")
		}
		log.Printf("Failed to create group join request: %v", err)
		return nil, status.Errorf(codes.Internal, "创建申请失败: %v", err)
	}

	log.Printf("Group join request %s created successfully", requestID)

	return &pb.SendGroupJoinRequestResponse{
		Code:      0,
		Message:   "加群申请已发送",
		RequestId: requestID,
	}, nil
}

// HandleGroupJoinRequest 处理群加入请求（接受/拒绝）
func (h *GroupHandler) HandleGroupJoinRequest(ctx context.Context, req *pb.HandleGroupJoinRequestRequest) (*pb.HandleGroupJoinRequestResponse, error) {
	reviewerID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s handling group join request %s with action %d", reviewerID, req.RequestId, req.Action)

	// 1. 查询申请信息
	var groupID, fromUserID string
	var currentStatus sql.NullString
	err = h.db.QueryRowContext(ctx,
		"SELECT group_id, from_user_id, status FROM group_join_requests WHERE id = ?",
		req.RequestId).Scan(&groupID, &fromUserID, &currentStatus)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "申请不存在")
	}
	if err != nil {
		log.Printf("Error querying request: %v", err)
		return nil, status.Errorf(codes.Internal, "查询申请失败")
	}

	statusStr := ""
	if currentStatus.Valid {
		statusStr = currentStatus.String
	}

	// 2. 检查申请状态
	if statusStr != "pending" {
		return nil, status.Errorf(codes.FailedPrecondition, "申请已被处理")
	}

	// 3. 检查处理者是否是群管理员或群主
	var role string
	err = h.db.QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		groupID, reviewerID).Scan(&role)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.PermissionDenied, "您不是群成员")
	}
	if err != nil {
		log.Printf("Error checking reviewer role: %v", err)
		return nil, status.Errorf(codes.Internal, "检查权限失败")
	}
	if role != "admin" {
		return nil, status.Errorf(codes.PermissionDenied, "只有管理员才能处理申请")
	}

	// 4. 更新申请状态
	var newStatus string
	if req.Action == 1 {
		newStatus = "accepted"
	} else if req.Action == 2 {
		newStatus = "rejected"
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "无效的操作")
	}

	query := `UPDATE group_join_requests 
	          SET status = ?, reviewed_by = ?, processed_at = NOW() 
	          WHERE id = ?`
	_, err = h.db.ExecContext(ctx, query, newStatus, reviewerID, req.RequestId)
	if err != nil {
		log.Printf("Failed to update request status: %v", err)
		return nil, status.Errorf(codes.Internal, "更新申请状态失败")
	}

	// 5. 如果接受，添加用户到群组
	if req.Action == 1 {
		// 使用 ON DUPLICATE KEY UPDATE 处理重新加入的情况（之前可能软删除了）
		query := `INSERT INTO group_members (group_id, user_id, role, joined_at, is_deleted) 
		          VALUES (?, ?, 'member', NOW(), 0)
		          ON DUPLICATE KEY UPDATE role = 'member', joined_at = NOW(), is_deleted = 0`
		_, err = h.db.ExecContext(ctx, query, groupID, fromUserID)
		if err != nil {
			log.Printf("Failed to add member to group: %v", err)
			return nil, status.Errorf(codes.Internal, "添加成员失败: %v", err)
		}
		log.Printf("User %s added to group %s", fromUserID, groupID)
	}

	message := "申请已拒绝"
	if req.Action == 1 {
		message = "申请已接受"
	}

	log.Printf("Group join request %s %s by %s", req.RequestId, newStatus, reviewerID)

	return &pb.HandleGroupJoinRequestResponse{
		Code:    0,
		Message: message,
	}, nil
}

// GetGroupJoinRequests 获取群的加入申请列表（管理员查看）
func (h *GroupHandler) GetGroupJoinRequests(ctx context.Context, req *pb.GetGroupJoinRequestsRequest) (*pb.GetGroupJoinRequestsResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s getting group join requests for group %s", userID, req.GroupId)

	// 1. 检查用户是否是群管理员
	var role string
	err = h.db.QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		req.GroupId, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.PermissionDenied, "您不是群成员")
	}
	if err != nil {
		log.Printf("Error checking user role: %v", err)
		return nil, status.Errorf(codes.Internal, "检查权限失败")
	}
	if role != "admin" {
		return nil, status.Errorf(codes.PermissionDenied, "只有管理员才能查看申请")
	}

	// 2. 设置默认值
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// 3. 构建查询条件
	var whereClause string
	if req.Status == 1 {
		whereClause = "AND gjr.status = 'pending'"
	} else if req.Status == 2 {
		whereClause = "AND gjr.status = 'accepted'"
	} else if req.Status == 3 {
		whereClause = "AND gjr.status = 'rejected'"
	}

	// 4. 查询申请列表
	query := `
		SELECT gjr.id, gjr.group_id, gjr.from_user_id, u.username, gjr.message, 
		       gjr.status, gjr.reviewed_by, gjr.created_at, gjr.processed_at
		FROM group_join_requests gjr
		LEFT JOIN users u ON gjr.from_user_id = u.id
		WHERE gjr.group_id = ? ` + whereClause + `
		ORDER BY gjr.created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query, req.GroupId, req.Limit, req.Offset)
	if err != nil {
		log.Printf("Failed to query join requests: %v", err)
		return nil, status.Errorf(codes.Internal, "查询申请失败")
	}
	defer rows.Close()

	var requests []*pb.GroupJoinRequest
	for rows.Next() {
		var request pb.GroupJoinRequest
		var createdAtStr string
		var processedAtStr sql.NullString
		var reviewedBy sql.NullString

		err := rows.Scan(
			&request.Id,
			&request.GroupId,
			&request.FromUserId,
			&request.FromUsername,
			&request.Message,
			&request.Status,
			&reviewedBy,
			&createdAtStr,
			&processedAtStr,
		)
		if err != nil {
			log.Printf("Failed to scan request row: %v", err)
			continue
		}

		if reviewedBy.Valid {
			request.ReviewedBy = reviewedBy.String
		}

		createdAt, _ := time.Parse("2006-01-02 15:04:05", createdAtStr)
		request.CreatedAt = createdAt.Unix()

		if processedAtStr.Valid {
			processedAt, _ := time.Parse("2006-01-02 15:04:05", processedAtStr.String)
			request.ProcessedAt = processedAt.Unix()
		}

		requests = append(requests, &request)
	}

	// 5. 查询总数
	var total int32
	countQuery := `SELECT COUNT(*) FROM group_join_requests WHERE group_id = ? ` + whereClause
	h.db.QueryRowContext(ctx, countQuery, req.GroupId).Scan(&total)

	return &pb.GetGroupJoinRequestsResponse{
		Code:     0,
		Message:  "查询成功",
		Requests: requests,
		Total:    total,
	}, nil
}

// GetMyGroupJoinRequests 获取我的加入申请列表
func (h *GroupHandler) GetMyGroupJoinRequests(ctx context.Context, req *pb.GetMyGroupJoinRequestsRequest) (*pb.GetMyGroupJoinRequestsResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s getting own group join requests", userID)

	// 1. 设置默认值
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// 2. 构建查询条件
	var whereClause string
	if req.Status == 1 {
		whereClause = "AND gjr.status = 'pending'"
	} else if req.Status == 2 {
		whereClause = "AND gjr.status = 'accepted'"
	} else if req.Status == 3 {
		whereClause = "AND gjr.status = 'rejected'"
	}

	// 3. 查询申请列表
	query := `
		SELECT gjr.id, gjr.group_id, g.name, gjr.from_user_id, gjr.message, 
		       gjr.status, gjr.reviewed_by, gjr.created_at, gjr.processed_at
		FROM group_join_requests gjr
		LEFT JOIN ` + "`groups`" + ` g ON gjr.group_id = g.id
		WHERE gjr.from_user_id = ? ` + whereClause + `
		ORDER BY gjr.created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query, userID, req.Limit, req.Offset)
	if err != nil {
		log.Printf("Failed to query join requests: %v", err)
		return nil, status.Errorf(codes.Internal, "查询申请失败")
	}
	defer rows.Close()

	var requests []*pb.GroupJoinRequest
	for rows.Next() {
		var request pb.GroupJoinRequest
		var groupName string
		var createdAtStr string
		var processedAtStr sql.NullString
		var reviewedBy sql.NullString

		err := rows.Scan(
			&request.Id,
			&request.GroupId,
			&groupName,
			&request.FromUserId,
			&request.Message,
			&request.Status,
			&reviewedBy,
			&createdAtStr,
			&processedAtStr,
		)
		if err != nil {
			log.Printf("Failed to scan request row: %v", err)
			continue
		}

		if reviewedBy.Valid {
			request.ReviewedBy = reviewedBy.String
		}

		// 使用 FromUsername 字段存储群名称（用于显示）
		request.FromUsername = groupName

		createdAt, _ := time.Parse("2006-01-02 15:04:05", createdAtStr)
		request.CreatedAt = createdAt.Unix()

		if processedAtStr.Valid {
			processedAt, _ := time.Parse("2006-01-02 15:04:05", processedAtStr.String)
			request.ProcessedAt = processedAt.Unix()
		}

		requests = append(requests, &request)
	}

	// 4. 查询总数
	var total int32
	countQuery := `SELECT COUNT(*) FROM group_join_requests WHERE from_user_id = ? ` + whereClause
	h.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)

	return &pb.GetMyGroupJoinRequestsResponse{
		Code:     0,
		Message:  "查询成功",
		Requests: requests,
		Total:    total,
	}, nil
}

// ==================== 群组管理功能 ====================

// UpdateGroupInfo 修改群组信息
func (h *GroupHandler) UpdateGroupInfo(ctx context.Context, req *pb.UpdateGroupInfoRequest) (*pb.UpdateGroupInfoResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s updating group %s info", userID, req.GroupId)

	// 1. 检查用户是否是群主或管理员
	var role string
	err = h.db.QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		req.GroupId, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.PermissionDenied, "您不是群成员")
	}
	if err != nil {
		log.Printf("Error checking user role: %v", err)
		return nil, status.Errorf(codes.Internal, "检查权限失败")
	}
	if role != "admin" {
		return nil, status.Errorf(codes.PermissionDenied, "只有管理员才能修改群信息")
	}

	// 2. 构建更新语句
	updates := []string{}
	args := []interface{}{}

	if req.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, req.Name)
	}
	if req.Description != "" {
		updates = append(updates, "description = ?")
		args = append(args, req.Description)
	}
	if req.Avatar != "" {
		updates = append(updates, "avatar = ?")
		args = append(args, req.Avatar)
	}

	if len(updates) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "没有需要更新的字段")
	}

	// 3. 执行更新
	args = append(args, req.GroupId)
	query := fmt.Sprintf("UPDATE `groups` SET %s WHERE id = ?", strings.Join(updates, ", "))
	_, err = h.db.ExecContext(ctx, query, args...)
	if err != nil {
		log.Printf("Failed to update group info: %v", err)
		return nil, status.Errorf(codes.Internal, "更新群信息失败")
	}

	log.Printf("Group %s info updated successfully by %s", req.GroupId, userID)

	return &pb.UpdateGroupInfoResponse{
		Code:    0,
		Message: "群信息更新成功",
	}, nil
}

// TransferOwner 转让群主
func (h *GroupHandler) TransferOwner(ctx context.Context, req *pb.TransferOwnerRequest) (*pb.TransferOwnerResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s transferring group %s ownership to %s", userID, req.GroupId, req.NewOwnerId)

	// 1. 检查当前用户是否是群主
	var currentRole string
	err = h.db.QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		req.GroupId, userID).Scan(&currentRole)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.PermissionDenied, "您不是群成员")
	}
	if err != nil {
		log.Printf("Error checking current user role: %v", err)
		return nil, status.Errorf(codes.Internal, "检查权限失败")
	}
	if currentRole != "admin" {
		return nil, status.Errorf(codes.PermissionDenied, "只有群主才能转让群")
	}

	// 2. 检查新群主是否是群成员
	var newOwnerRole string
	err = h.db.QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		req.GroupId, req.NewOwnerId).Scan(&newOwnerRole)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "新群主不是群成员")
	}
	if err != nil {
		log.Printf("Error checking new owner membership: %v", err)
		return nil, status.Errorf(codes.Internal, "检查成员状态失败")
	}

	// 3. 开始事务
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "开始事务失败")
	}
	defer tx.Rollback()

	// 4. 更新 groups 表的 creator_id
	_, err = tx.ExecContext(ctx,
		"UPDATE `groups` SET creator_id = ? WHERE id = ?",
		req.NewOwnerId, req.GroupId)
	if err != nil {
		log.Printf("Failed to update group creator: %v", err)
		return nil, status.Errorf(codes.Internal, "更新群主失败")
	}

	// 5. 将原群主降为普通成员（如果新群主原来是管理员，则保持）
	_, err = tx.ExecContext(ctx,
		"UPDATE group_members SET role = 'member' WHERE group_id = ? AND user_id = ?",
		req.GroupId, userID)
	if err != nil {
		log.Printf("Failed to demote old owner: %v", err)
		return nil, status.Errorf(codes.Internal, "更新权限失败")
	}

	// 6. 将新群主设置为管理员
	_, err = tx.ExecContext(ctx,
		"UPDATE group_members SET role = 'admin' WHERE group_id = ? AND user_id = ?",
		req.GroupId, req.NewOwnerId)
	if err != nil {
		log.Printf("Failed to promote new owner: %v", err)
		return nil, status.Errorf(codes.Internal, "更新权限失败")
	}

	// 7. 提交事务
	if err = tx.Commit(); err != nil {
		return nil, status.Errorf(codes.Internal, "提交事务失败")
	}

	log.Printf("Group %s ownership transferred from %s to %s", req.GroupId, userID, req.NewOwnerId)

	return &pb.TransferOwnerResponse{
		Code:    0,
		Message: "群主转让成功",
	}, nil
}

// DismissGroup 解散群组
func (h *GroupHandler) DismissGroup(ctx context.Context, req *pb.DismissGroupRequest) (*pb.DismissGroupResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s dismissing group %s", userID, req.GroupId)

	// 1. 检查群组是否存在
	var creatorID string
	err = h.db.QueryRowContext(ctx,
		"SELECT creator_id FROM `groups` WHERE id = ? AND is_deleted = 0",
		req.GroupId).Scan(&creatorID)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "群组不存在")
	}
	if err != nil {
		log.Printf("Error checking group: %v", err)
		return nil, status.Errorf(codes.Internal, "查询群组失败")
	}

	// 2. 检查用户是否是群主
	if creatorID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "只有群主才能解散群")
	}

	// 3. 开始事务
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "开始事务失败")
	}
	defer tx.Rollback()

	// 4. 软删除群组
	_, err = tx.ExecContext(ctx,
		"UPDATE `groups` SET is_deleted = 1 WHERE id = ?",
		req.GroupId)
	if err != nil {
		log.Printf("Failed to delete group: %v", err)
		return nil, status.Errorf(codes.Internal, "解散群组失败")
	}

	// 5. 软删除所有群成员
	_, err = tx.ExecContext(ctx,
		"UPDATE group_members SET is_deleted = 1 WHERE group_id = ?",
		req.GroupId)
	if err != nil {
		log.Printf("Failed to remove group members: %v", err)
		return nil, status.Errorf(codes.Internal, "移除成员失败")
	}

	// 6. 提交事务
	if err = tx.Commit(); err != nil {
		return nil, status.Errorf(codes.Internal, "提交事务失败")
	}

	log.Printf("Group %s dismissed by %s", req.GroupId, userID)

	return &pb.DismissGroupResponse{
		Code:    0,
		Message: "群组已解散",
	}, nil
}

// SetAdmin 设置/取消管理员
func (h *GroupHandler) SetAdmin(ctx context.Context, req *pb.SetAdminRequest) (*pb.SetAdminResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s setting admin status for %s in group %s: %v", userID, req.UserId, req.GroupId, req.IsAdmin)

	// 1. 检查操作者是否是群主
	var creatorID string
	err = h.db.QueryRowContext(ctx,
		"SELECT creator_id FROM `groups` WHERE id = ? AND is_deleted = 0",
		req.GroupId).Scan(&creatorID)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "群组不存在")
	}
	if err != nil {
		log.Printf("Error checking group: %v", err)
		return nil, status.Errorf(codes.Internal, "查询群组失败")
	}

	if creatorID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "只有群主才能设置管理员")
	}

	// 2. 检查目标用户是否是群成员
	var targetRole string
	err = h.db.QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		req.GroupId, req.UserId).Scan(&targetRole)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "用户不是群成员")
	}
	if err != nil {
		log.Printf("Error checking target user: %v", err)
		return nil, status.Errorf(codes.Internal, "查询用户失败")
	}

	// 3. 不能设置群主为管理员（群主已经是最高权限）
	if req.UserId == creatorID {
		return nil, status.Errorf(codes.InvalidArgument, "不能修改群主的权限")
	}

	// 4. 更新角色
	newRole := "member"
	if req.IsAdmin {
		newRole = "admin"
	}

	_, err = h.db.ExecContext(ctx,
		"UPDATE group_members SET role = ? WHERE group_id = ? AND user_id = ?",
		newRole, req.GroupId, req.UserId)
	if err != nil {
		log.Printf("Failed to update member role: %v", err)
		return nil, status.Errorf(codes.Internal, "更新权限失败")
	}

	message := "管理员设置成功"
	if !req.IsAdmin {
		message = "管理员已取消"
	}

	log.Printf("User %s role in group %s updated to %s", req.UserId, req.GroupId, newRole)

	return &pb.SetAdminResponse{
		Code:    0,
		Message: message,
	}, nil
}

// GetGroupMembers 获取群成员列表
func (h *GroupHandler) GetGroupMembers(ctx context.Context, req *pb.GetGroupMembersRequest) (*pb.GetGroupMembersResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s getting members of group %s", userID, req.GroupId)

	// 1. 检查用户是否是群成员
	var isMember int
	err = h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ? AND is_deleted = 0",
		req.GroupId, userID).Scan(&isMember)
	if err != nil || isMember == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "您不是群成员")
	}

	// 2. 设置默认值
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// 3. 查询成员列表
	query := `
		SELECT gm.user_id, u.username, u.nickname, gm.role, gm.joined_at
		FROM group_members gm
		LEFT JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = ? AND gm.is_deleted = 0
		ORDER BY 
			CASE gm.role 
				WHEN 'admin' THEN 1 
				ELSE 2 
			END,
			gm.joined_at ASC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query, req.GroupId, req.Limit, req.Offset)
	if err != nil {
		log.Printf("Failed to query group members: %v", err)
		return nil, status.Errorf(codes.Internal, "查询成员失败")
	}
	defer rows.Close()

	var members []*pb.GroupMember
	for rows.Next() {
		var member pb.GroupMember
		var joinedAtStr string
		var nickname sql.NullString

		err := rows.Scan(
			&member.UserId,
			&member.Username,
			&nickname,
			&member.Role,
			&joinedAtStr,
		)
		if err != nil {
			log.Printf("Failed to scan member row: %v", err)
			continue
		}

		if nickname.Valid {
			member.Nickname = nickname.String
		}

		joinedAt, _ := time.Parse("2006-01-02 15:04:05", joinedAtStr)
		member.JoinedAt = joinedAt.Unix()

		members = append(members, &member)
	}

	// 4. 查询总数
	var total int32
	h.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM group_members WHERE group_id = ? AND is_deleted = 0",
		req.GroupId).Scan(&total)

	return &pb.GetGroupMembersResponse{
		Code:    0,
		Message: "查询成功",
		Members: members,
		Total:   total,
	}, nil
}

// ==================== 搜索功能 ====================

// SearchGroups 搜索群组
func (h *GroupHandler) SearchGroups(ctx context.Context, req *pb.SearchGroupsRequest) (*pb.SearchGroupsResponse, error) {
	log.Printf("Searching groups with keyword: %s", req.Keyword)

	// 设置默认值
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	// 如果关键词为空，返回空结果
	if req.Keyword == "" {
		return &pb.SearchGroupsResponse{
			Code:    0,
			Message: "搜索成功",
			Groups:  []*pb.GroupSearchResult{},
			Total:   0,
		}, nil
	}

	// 搜索群组（群名称或描述包含关键词）
	keyword := "%" + req.Keyword + "%"
	query := `
		SELECT g.id, g.name, IFNULL(g.description, ''), IFNULL(g.avatar, ''),
		       (SELECT COUNT(*) FROM group_members WHERE group_id = g.id AND is_deleted = 0) as member_count
		FROM ` + "`groups`" + ` g
		WHERE (g.name LIKE ? OR g.description LIKE ?)
		  AND g.is_deleted = 0
		ORDER BY 
			CASE 
				WHEN g.name = ? THEN 1
				WHEN g.name LIKE ? THEN 2
				ELSE 3
			END,
			member_count DESC,
			g.name ASC
		LIMIT ? OFFSET ?`

	rows, err := h.db.QueryContext(ctx, query,
		keyword, keyword,
		req.Keyword, req.Keyword+"%",
		req.Limit, req.Offset)
	if err != nil {
		log.Printf("Failed to search groups: %v", err)
		return &pb.SearchGroupsResponse{
			Code:    -1,
			Message: "搜索失败",
			Groups:  []*pb.GroupSearchResult{},
			Total:   0,
		}, nil
	}
	defer rows.Close()

	var groups []*pb.GroupSearchResult
	for rows.Next() {
		var group pb.GroupSearchResult
		err := rows.Scan(&group.Id, &group.Name, &group.Description, &group.Avatar, &group.MemberCount)
		if err != nil {
			log.Printf("Failed to scan group row: %v", err)
			continue
		}
		groups = append(groups, &group)
	}

	// 查询总数
	var total int32
	countQuery := `
		SELECT COUNT(*) 
		FROM ` + "`groups`" + ` 
		WHERE (name LIKE ? OR description LIKE ?) 
		  AND is_deleted = 0`
	h.db.QueryRowContext(ctx, countQuery, keyword, keyword).Scan(&total)

	log.Printf("Found %d groups matching keyword: %s", len(groups), req.Keyword)

	return &pb.SearchGroupsResponse{
		Code:    0,
		Message: "搜索成功",
		Groups:  groups,
		Total:   total,
	}, nil
}
