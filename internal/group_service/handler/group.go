package handler

import (
	"context"
	"database/sql"
	"log"
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
