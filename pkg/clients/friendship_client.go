package clients

import (
	"context"
	"log"

	pb "ChatIM/api/proto/friendship"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// FriendshipClient 友谊服务客户端
type FriendshipClient struct {
	conn   *grpc.ClientConn
	client pb.FriendshipServiceClient
}

// NewFriendshipClient 创建新的友谊服务客户端
func NewFriendshipClient(addr string) (*FriendshipClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to friendship service: %v", err)
		return nil, err
	}

	return &FriendshipClient{
		conn:   conn,
		client: pb.NewFriendshipServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (fc *FriendshipClient) Close() error {
	return fc.conn.Close()
}

// SendFriendRequest 发送好友请求
func (fc *FriendshipClient) SendFriendRequest(ctx context.Context, toUserID, message string) (string, error) {
	resp, err := fc.client.SendFriendRequest(ctx, &pb.SendFriendRequestRequest{
		ToUserId: toUserID,
		Message:  message,
	})
	if err != nil {
		log.Printf("Error sending friend request: %v", err)
		return "", err
	}

	if resp.Code != 0 {
		log.Printf("Friend request failed: %s", resp.Message)
		return "", err
	}

	return resp.RequestId, nil
}

// GetFriendRequests 获取好友请求列表
func (fc *FriendshipClient) GetFriendRequests(ctx context.Context, status int32, limit, offset int64) ([]*pb.FriendRequest, int32, error) {
	resp, err := fc.client.GetFriendRequests(ctx, &pb.GetFriendRequestsRequest{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Printf("Error getting friend requests: %v", err)
		return nil, 0, err
	}

	if resp.Code != 0 {
		log.Printf("Get friend requests failed: %s", resp.Message)
		return nil, 0, err
	}

	return resp.Requests, resp.Total, nil
}

// ProcessFriendRequest 处理好友请求
func (fc *FriendshipClient) ProcessFriendRequest(ctx context.Context, requestID string, accept bool) error {
	resp, err := fc.client.ProcessFriendRequest(ctx, &pb.ProcessFriendRequestRequest{
		RequestId: requestID,
		Accept:    accept,
	})
	if err != nil {
		log.Printf("Error processing friend request: %v", err)
		return err
	}

	if resp.Code != 0 {
		log.Printf("Process friend request failed: %s", resp.Message)
		return err
	}

	return nil
}

// GetFriends 获取好友列表
func (fc *FriendshipClient) GetFriends(ctx context.Context, limit, offset int64) ([]*pb.Friend, int32, error) {
	resp, err := fc.client.GetFriends(ctx, &pb.GetFriendsRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Printf("Error getting friends: %v", err)
		return nil, 0, err
	}

	if resp.Code != 0 {
		log.Printf("Get friends failed: %s", resp.Message)
		return nil, 0, err
	}

	return resp.Friends, resp.Total, nil
}

// RemoveFriend 删除好友
func (fc *FriendshipClient) RemoveFriend(ctx context.Context, friendUserID string) error {
	resp, err := fc.client.RemoveFriend(ctx, &pb.RemoveFriendRequest{
		FriendUserId: friendUserID,
	})
	if err != nil {
		log.Printf("Error removing friend: %v", err)
		return err
	}

	if resp.Code != 0 {
		log.Printf("Remove friend failed: %s", resp.Message)
		return err
	}

	return nil
}

// SendGroupJoinRequest 发送群加入请求
func (fc *FriendshipClient) SendGroupJoinRequest(ctx context.Context, groupID, message string) (string, error) {
	resp, err := fc.client.SendGroupJoinRequest(ctx, &pb.SendGroupJoinRequestRequest{
		GroupId: groupID,
		Message: message,
	})
	if err != nil {
		log.Printf("Error sending group join request: %v", err)
		return "", err
	}

	if resp.Code != 0 {
		log.Printf("Group join request failed: %s", resp.Message)
		return "", err
	}

	return resp.RequestId, nil
}

// GetGroupJoinRequests 获取群加入请求列表
func (fc *FriendshipClient) GetGroupJoinRequests(ctx context.Context, groupID string, status int32, limit, offset int64) ([]*pb.GroupJoinRequest, int32, error) {
	resp, err := fc.client.GetGroupJoinRequests(ctx, &pb.GetGroupJoinRequestsRequest{
		GroupId: groupID,
		Status:  status,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		log.Printf("Error getting group join requests: %v", err)
		return nil, 0, err
	}

	if resp.Code != 0 {
		log.Printf("Get group join requests failed: %s", resp.Message)
		return nil, 0, err
	}

	return resp.Requests, resp.Total, nil
}

// ProcessGroupJoinRequest 处理群加入请求
func (fc *FriendshipClient) ProcessGroupJoinRequest(ctx context.Context, requestID string, accept bool) error {
	resp, err := fc.client.ProcessGroupJoinRequest(ctx, &pb.ProcessGroupJoinRequestRequest{
		RequestId: requestID,
		Accept:    accept,
	})
	if err != nil {
		log.Printf("Error processing group join request: %v", err)
		return err
	}

	if resp.Code != 0 {
		log.Printf("Process group join request failed: %s", resp.Message)
		return err
	}

	return nil
}

// GetUserGroups 获取用户所在的所有群组
func (fc *FriendshipClient) GetUserGroups(ctx context.Context, limit, offset int64) ([]*pb.GroupInfo, int32, error) {
	resp, err := fc.client.GetUserGroups(ctx, &pb.GetUserGroupsRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Printf("Error getting user groups: %v", err)
		return nil, 0, err
	}

	if resp.Code != 0 {
		log.Printf("Get user groups failed: %s", resp.Message)
		return nil, 0, err
	}

	return resp.Groups, resp.Total, nil
}

// LeaveGroup 用户退出群组
func (fc *FriendshipClient) LeaveGroup(ctx context.Context, groupID string) error {
	resp, err := fc.client.LeaveGroup(ctx, &pb.LeaveGroupRequest{
		GroupId: groupID,
	})
	if err != nil {
		log.Printf("Error leaving group: %v", err)
		return err
	}

	if resp.Code != 0 {
		log.Printf("Leave group failed: %s", resp.Message)
		return err
	}

	return nil
}

// RemoveGroupMember 管理员踢出群成员
func (fc *FriendshipClient) RemoveGroupMember(ctx context.Context, groupID, memberUserID string) error {
	resp, err := fc.client.RemoveGroupMember(ctx, &pb.RemoveGroupMemberRequest{
		GroupId:      groupID,
		MemberUserId: memberUserID,
	})
	if err != nil {
		log.Printf("Error removing group member: %v", err)
		return err
	}

	if resp.Code != 0 {
		log.Printf("Remove group member failed: %s", resp.Message)
		return err
	}

	return nil
}
