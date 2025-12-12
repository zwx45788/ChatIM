package handler

import (
	"context"
	"database/sql"
	"log"

	pb "ChatIM/api/proto/friendship"
	"ChatIM/internal/friendship/model"
	"ChatIM/internal/friendship/repository"
	"ChatIM/pkg/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FriendshipHandler 处理好友和群申请相关的 gRPC 请求
type FriendshipHandler struct {
	pb.UnimplementedFriendshipServiceServer
	repo *repository.FriendshipRepository
}

// NewFriendshipHandler 创建好友处理器实例
func NewFriendshipHandler(repo *repository.FriendshipRepository) *FriendshipHandler {
	return &FriendshipHandler{
		repo: repo,
	}
}

// ==================== 好友请求处理 ====================

// SendFriendRequest 发送好友请求
func (h *FriendshipHandler) SendFriendRequest(ctx context.Context, req *pb.SendFriendRequestRequest) (*pb.SendFriendRequestResponse, error) {
	fromUserID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s sending friend request to %s", fromUserID, req.ToUserId)

	// 验证不能加自己
	if req.ToUserId == fromUserID {
		return nil, status.Errorf(codes.InvalidArgument, "不能添加自己为好友")
	}

	// 检查是否已是好友
	exists, err := h.repo.CheckFriendshipExists(ctx, fromUserID, req.ToUserId)
	if err != nil {
		log.Printf("Error checking friendship: %v", err)
		return nil, status.Errorf(codes.Internal, "检查好友关系失败")
	}
	if exists {
		return nil, status.Errorf(codes.AlreadyExists, "已经是好友了")
	}

	// 检查是否已发送过待处理请求
	pending, err := h.repo.CheckPendingFriendRequest(ctx, fromUserID, req.ToUserId)
	if err != nil {
		log.Printf("Error checking pending request: %v", err)
		return nil, status.Errorf(codes.Internal, "检查待处理请求失败")
	}
	if pending {
		return nil, status.Errorf(codes.AlreadyExists, "已发送过申请，请等待处理")
	}

	// 发送好友请求
	requestID, err := h.repo.SendFriendRequest(ctx, fromUserID, req.ToUserId, req.Message)
	if err != nil {
		log.Printf("Error sending friend request: %v", err)
		return nil, status.Errorf(codes.Internal, "发送请求失败")
	}

	log.Printf("Friend request %s sent successfully", requestID)
	return &pb.SendFriendRequestResponse{
		Code:      0,
		Message:   "好友请求已发送",
		RequestId: requestID,
	}, nil
}

// GetFriendRequests 获取好友请求列表
func (h *FriendshipHandler) GetFriendRequests(ctx context.Context, req *pb.GetFriendRequestsRequest) (*pb.GetFriendRequestsResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s getting friend requests with status %d", userID, req.Status)

	// 转换状态
	statusStr := model.IntToStatus(req.Status)

	// 设置分页默认值
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// 查询请求
	requests, err := h.repo.GetFriendRequests(ctx, userID, statusStr, limit, offset)
	if err != nil {
		log.Printf("Error getting friend requests: %v", err)
		return nil, status.Errorf(codes.Internal, "查询失败")
	}

	// 统计总数
	total, err := h.repo.CountFriendRequests(ctx, userID, statusStr)
	if err != nil {
		log.Printf("Error counting friend requests: %v", err)
		return nil, status.Errorf(codes.Internal, "统计失败")
	}

	// 转换为 protobuf 格式
	pbRequests := make([]*pb.FriendRequest, len(requests))
	for i, req := range requests {
		pbRequests[i] = &pb.FriendRequest{
			Id:           req.ID,
			FromUserId:   req.FromUserID,
			FromUsername: req.FromUsername,
			FromNickname: req.FromNickname,
			Message:      req.Message,
			Status:       model.StatusToInt(req.Status),
			CreatedAt:    req.CreatedAt.Unix(),
		}
	}

	return &pb.GetFriendRequestsResponse{
		Code:     0,
		Message:  "查询成功",
		Requests: pbRequests,
		Total:    total,
	}, nil
}

// ProcessFriendRequest 处理好友请求（接受/拒绝）
func (h *FriendshipHandler) ProcessFriendRequest(ctx context.Context, req *pb.ProcessFriendRequestRequest) (*pb.ProcessFriendRequestResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s processing friend request %s, accept=%v", userID, req.RequestId, req.Accept)

	// 查询请求
	friendReq, err := h.repo.GetFriendRequest(ctx, req.RequestId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "请求不存在")
		}
		log.Printf("Error getting friend request: %v", err)
		return nil, status.Errorf(codes.Internal, "查询失败")
	}

	// 验证权限：只有接收者能处理
	if friendReq.ToUserID != userID {
		return nil, status.Errorf(codes.PermissionDenied, "无权处理此请求")
	}

	// 处理请求
	if req.Accept {
		err = h.repo.AcceptFriendRequest(ctx, req.RequestId)
		if err != nil {
			log.Printf("Error accepting friend request: %v", err)
			return nil, status.Errorf(codes.Internal, "接受失败")
		}
	} else {
		err = h.repo.RejectFriendRequest(ctx, req.RequestId)
		if err != nil {
			log.Printf("Error rejecting friend request: %v", err)
			return nil, status.Errorf(codes.Internal, "拒绝失败")
		}
	}

	message := "请求已处理"
	if req.Accept {
		message = "已添加为好友"
	} else {
		message = "请求已拒绝"
	}

	log.Printf("Friend request %s processed successfully (accept=%v)", req.RequestId, req.Accept)
	return &pb.ProcessFriendRequestResponse{
		Code:    0,
		Message: message,
	}, nil
}

// GetFriends 获取好友列表
func (h *FriendshipHandler) GetFriends(ctx context.Context, req *pb.GetFriendsRequest) (*pb.GetFriendsResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s getting friends list", userID)

	// 设置分页默认值
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// 查询好友列表
	friends, err := h.repo.GetFriends(ctx, userID, limit, offset)
	if err != nil {
		log.Printf("Error getting friends: %v", err)
		return nil, status.Errorf(codes.Internal, "查询失败")
	}

	// 统计总数
	total, err := h.repo.CountFriends(ctx, userID)
	if err != nil {
		log.Printf("Error counting friends: %v", err)
		return nil, status.Errorf(codes.Internal, "统计失败")
	}

	// 转换为 protobuf 格式
	pbFriends := make([]*pb.Friend, len(friends))
	for i, f := range friends {
		pbFriends[i] = &pb.Friend{
			UserId:    f["user_id"].(string),
			Username:  f["username"].(string),
			Nickname:  f["nickname"].(string),
			CreatedAt: f["created_at"].(int64),
		}
	}

	return &pb.GetFriendsResponse{
		Code:    0,
		Message: "查询成功",
		Friends: pbFriends,
		Total:   total,
	}, nil
}

// RemoveFriend 删除好友
func (h *FriendshipHandler) RemoveFriend(ctx context.Context, req *pb.RemoveFriendRequest) (*pb.RemoveFriendResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s removing friend %s", userID, req.FriendUserId)

	// 删除好友关系
	err = h.repo.RemoveFriend(ctx, userID, req.FriendUserId)
	if err != nil {
		log.Printf("Error removing friend: %v", err)
		if err.Error() == "friendship not found" {
			return nil, status.Errorf(codes.NotFound, "好友关系不存在")
		}
		return nil, status.Errorf(codes.Internal, "删除失败")
	}

	log.Printf("Friend relationship removed successfully")
	return &pb.RemoveFriendResponse{
		Code:    0,
		Message: "好友已删除",
	}, nil
}

// ==================== 用户群组相关处理 ====================

// GetUserGroups 获取用户所在的所有群组
func (h *FriendshipHandler) GetUserGroups(ctx context.Context, req *pb.GetUserGroupsRequest) (*pb.GetUserGroupsResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s getting user groups", userID)

	// 设置分页默认值
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// 查询群组列表
	groups, err := h.repo.GetUserGroups(ctx, userID, limit, offset)
	if err != nil {
		log.Printf("Error getting user groups: %v", err)
		return nil, status.Errorf(codes.Internal, "查询失败")
	}

	// 统计总数
	total, err := h.repo.CountUserGroups(ctx, userID)
	if err != nil {
		log.Printf("Error counting user groups: %v", err)
		return nil, status.Errorf(codes.Internal, "统计失败")
	}

	// 转换为 protobuf 格式
	pbGroups := make([]*pb.GroupInfo, len(groups))
	for i, g := range groups {
		pbGroups[i] = &pb.GroupInfo{
			GroupId:     g["group_id"].(string),
			GroupName:   g["group_name"].(string),
			Description: g["description"].(string),
			MemberCount: g["member_count"].(int32),
			CreatedAt:   g["created_at"].(int64),
		}
	}

	return &pb.GetUserGroupsResponse{
		Code:    0,
		Message: "查询成功",
		Groups:  pbGroups,
		Total:   total,
	}, nil
}

// LeaveGroup 用户退出群组
func (h *FriendshipHandler) LeaveGroup(ctx context.Context, req *pb.LeaveGroupRequest) (*pb.LeaveGroupResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s leaving group %s", userID, req.GroupId)

	// 检查用户是否在群组中
	isMember, err := h.repo.CheckGroupMembership(ctx, req.GroupId, userID)
	if err != nil {
		log.Printf("Error checking group membership: %v", err)
		return nil, status.Errorf(codes.Internal, "检查失败")
	}
	if !isMember {
		return nil, status.Errorf(codes.NotFound, "用户不在该群组中")
	}

	// 退出群组
	err = h.repo.LeaveGroup(ctx, req.GroupId, userID)
	if err != nil {
		log.Printf("Error leaving group: %v", err)
		return nil, status.Errorf(codes.Internal, "退出失败")
	}

	log.Printf("User %s successfully left group %s", userID, req.GroupId)
	return &pb.LeaveGroupResponse{
		Code:    0,
		Message: "已退出群组",
	}, nil
}

// RemoveGroupMember 管理员踢出群成员
func (h *FriendshipHandler) RemoveGroupMember(ctx context.Context, req *pb.RemoveGroupMemberRequest) (*pb.RemoveGroupMemberResponse, error) {
	operatorUserID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s removing member %s from group %s", operatorUserID, req.MemberUserId, req.GroupId)

	// 检查操作者是否是群主
	isOwner, err := h.repo.CheckGroupOwner(ctx, req.GroupId, operatorUserID)
	if err != nil {
		log.Printf("Error checking group owner: %v", err)
		return nil, status.Errorf(codes.Internal, "检查失败")
	}
	if !isOwner {
		return nil, status.Errorf(codes.PermissionDenied, "只有群主才能踢人")
	}

	// 不能踢自己
	if req.MemberUserId == operatorUserID {
		return nil, status.Errorf(codes.InvalidArgument, "不能踢出自己")
	}

	// 检查被踢者是否在群组中
	isMember, err := h.repo.CheckGroupMembership(ctx, req.GroupId, req.MemberUserId)
	if err != nil {
		log.Printf("Error checking member status: %v", err)
		return nil, status.Errorf(codes.Internal, "检查失败")
	}
	if !isMember {
		return nil, status.Errorf(codes.NotFound, "该用户不在群组中")
	}

	// 踢出成员
	err = h.repo.RemoveGroupMember(ctx, req.GroupId, req.MemberUserId)
	if err != nil {
		log.Printf("Error removing group member: %v", err)
		return nil, status.Errorf(codes.Internal, "踢出失败")
	}

	log.Printf("User %s removed from group %s by %s", req.MemberUserId, req.GroupId, operatorUserID)
	return &pb.RemoveGroupMemberResponse{
		Code:    0,
		Message: "已踢出该成员",
	}, nil
}
