package handler

import (
	"context"
	"database/sql"
	"log"

	pb "ChatIM/api/proto/friendship"
	"ChatIM/internal/friendship/model"
	"ChatIM/pkg/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ==================== 群申请处理 ====================

// SendGroupJoinRequest 申请加入群组
func (h *FriendshipHandler) SendGroupJoinRequest(ctx context.Context, req *pb.SendGroupJoinRequestRequest) (*pb.SendGroupJoinRequestResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s sending group join request for group %s", userID, req.GroupId)

	// 检查用户是否已是群成员
	isMember, err := h.repo.CheckGroupMemberExists(ctx, req.GroupId, userID)
	if err != nil {
		log.Printf("Error checking group membership: %v", err)
		return nil, status.Errorf(codes.Internal, "检查成员身份失败")
	}
	if isMember {
		return nil, status.Errorf(codes.AlreadyExists, "已是群成员")
	}

	// 检查是否已发送过待处理的申请
	pending, err := h.repo.CheckPendingGroupJoinRequest(ctx, req.GroupId, userID)
	if err != nil {
		log.Printf("Error checking pending request: %v", err)
		return nil, status.Errorf(codes.Internal, "检查待处理申请失败")
	}
	if pending {
		return nil, status.Errorf(codes.AlreadyExists, "已发送过申请，请等待处理")
	}

	// 发送群申请
	requestID, err := h.repo.SendGroupJoinRequest(ctx, req.GroupId, userID, req.Message)
	if err != nil {
		log.Printf("Error sending group join request: %v", err)
		return nil, status.Errorf(codes.Internal, "发送申请失败")
	}

	log.Printf("Group join request %s sent successfully", requestID)
	return &pb.SendGroupJoinRequestResponse{
		Code:      0,
		Message:   "群申请已发送",
		RequestId: requestID,
	}, nil
}

// GetGroupJoinRequests 获取群申请列表（群主/管理员）
func (h *FriendshipHandler) GetGroupJoinRequests(ctx context.Context, req *pb.GetGroupJoinRequestsRequest) (*pb.GetGroupJoinRequestsResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s getting group join requests for group %s", userID, req.GroupId)

	// 检查权限：必须是群主或管理员
	isAdmin, err := h.repo.CheckGroupAdmin(ctx, req.GroupId, userID)
	if err != nil {
		log.Printf("Error checking admin permission: %v", err)
		return nil, status.Errorf(codes.Internal, "检查权限失败")
	}
	if !isAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "无权限查看群申请")
	}

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

	// 查询申请列表
	requests, err := h.repo.GetGroupJoinRequests(ctx, req.GroupId, statusStr, limit, offset)
	if err != nil {
		log.Printf("Error getting group join requests: %v", err)
		return nil, status.Errorf(codes.Internal, "查询失败")
	}

	// 统计总数
	total, err := h.repo.CountGroupJoinRequests(ctx, req.GroupId, statusStr)
	if err != nil {
		log.Printf("Error counting group join requests: %v", err)
		return nil, status.Errorf(codes.Internal, "统计失败")
	}

	// 转换为 protobuf 格式
	pbRequests := make([]*pb.GroupJoinRequest, len(requests))
	for i, req := range requests {
		pbRequests[i] = &pb.GroupJoinRequest{
			Id:           req.ID,
			GroupId:      req.GroupID,
			FromUserId:   req.FromUserID,
			FromUsername: req.FromUsername,
			FromNickname: req.FromNickname,
			Message:      req.Message,
			Status:       model.StatusToInt(req.Status),
			CreatedAt:    req.CreatedAt.Unix(),
		}
	}

	return &pb.GetGroupJoinRequestsResponse{
		Code:     0,
		Message:  "查询成功",
		Requests: pbRequests,
		Total:    total,
	}, nil
}

// ProcessGroupJoinRequest 处理群申请（接受/拒绝）
func (h *FriendshipHandler) ProcessGroupJoinRequest(ctx context.Context, req *pb.ProcessGroupJoinRequestRequest) (*pb.ProcessGroupJoinRequestResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "未认证用户")
	}

	log.Printf("User %s processing group join request %s, accept=%v", userID, req.RequestId, req.Accept)

	// 查询请求
	joinReq, err := h.repo.GetGroupJoinRequest(ctx, req.RequestId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Errorf(codes.NotFound, "申请不存在")
		}
		log.Printf("Error getting group join request: %v", err)
		return nil, status.Errorf(codes.Internal, "查询失败")
	}

	// 检查权限：必须是群主或管理员
	isAdmin, err := h.repo.CheckGroupAdmin(ctx, joinReq.GroupID, userID)
	if err != nil {
		log.Printf("Error checking admin permission: %v", err)
		return nil, status.Errorf(codes.Internal, "检查权限失败")
	}
	if !isAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "无权限处理此申请")
	}

	// 处理申请
	if req.Accept {
		err = h.repo.AcceptGroupJoinRequest(ctx, req.RequestId, userID)
		if err != nil {
			log.Printf("Error accepting group join request: %v", err)
			return nil, status.Errorf(codes.Internal, "接受失败")
		}
	} else {
		err = h.repo.RejectGroupJoinRequest(ctx, req.RequestId, userID)
		if err != nil {
			log.Printf("Error rejecting group join request: %v", err)
			return nil, status.Errorf(codes.Internal, "拒绝失败")
		}
	}

	message := "申请已处理"
	if req.Accept {
		message = "已加入群组"
	} else {
		message = "申请已拒绝"
	}

	log.Printf("Group join request %s processed successfully (accept=%v)", req.RequestId, req.Accept)
	return &pb.ProcessGroupJoinRequestResponse{
		Code:    0,
		Message: message,
	}, nil
}
