package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"

	friendPb "ChatIM/api/proto/friendship"
	grpPb "ChatIM/api/proto/group"
	msgPb "ChatIM/api/proto/message"
	pb "ChatIM/api/proto/user"
	"ChatIM/internal/api_gateway/middleware"
	"ChatIM/pkg/config"
	"ChatIM/pkg/oss"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// withAuthMetadata attaches Authorization header into outgoing gRPC context.
func withAuthMetadata(c *gin.Context) context.Context {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return c.Request.Context()
	}
	md := metadata.New(map[string]string{"authorization": authHeader})
	return metadata.NewOutgoingContext(c.Request.Context(), md)
}

type UserGatewayHandler struct {
	userClient       pb.UserServiceClient
	messageClient    msgPb.MessageServiceClient
	groupClient      grpPb.GroupServiceClient
	friendshipClient friendPb.FriendshipServiceClient
	ossClient        *oss.OSSClient
}

func NewUserGatewayHandler() (*UserGatewayHandler, error) {
	// 👇 2. 在这里加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Printf("Failed to load config in handler: %v", err)
		return nil, err
	}

	// 👇 3. 使用配置中的地址创建连接
	// 连接到 user-service
	// 如果环境变量提供了完整地址（如 user-service:50051），直接使用
	// 否则使用默认的 127.0.0.1:port
	userAddr := cfg.Server.UserGRPCAddr
	if userAddr == "" {
		userAddr = "127.0.0.1" + cfg.Server.UserGRPCPort
	}
	log.Printf("Connecting to User Service at: %s", userAddr)

	userConn, err := grpc.Dial(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("did not connect to user service: %v", err)
		return nil, err
	}

	// 连接到 message-service
	messageAddr := cfg.Server.MessageGRPCAddr
	if messageAddr == "" {
		messageAddr = "127.0.0.1" + cfg.Server.MessageGRPCPort
	}
	log.Printf("Connecting to Message Service at: %s", messageAddr)

	msgConn, err := grpc.Dial(messageAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("did not connect to message service: %v", err)
		return nil, err
	}

	// 连接到 group-service
	groupAddr := cfg.Server.GroupGRPCAddr
	if groupAddr == "" {
		groupAddr = "127.0.0.1" + cfg.Server.GroupGRPCPort
	}
	log.Printf("Connecting to Group Service at: %s", groupAddr)

	grpConn, err := grpc.Dial(groupAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("did not connect to group service: %v", err)
		return nil, err
	}

	// 连接到 friendship-service
	friendshipAddr := cfg.Server.FriendshipGRPCAddr
	if friendshipAddr == "" {
		friendshipAddr = "127.0.0.1" + cfg.Server.FriendshipGRPCPort
	}
	log.Printf("Connecting to Friendship Service at: %s", friendshipAddr)

	frConn, err := grpc.Dial(friendshipAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("did not connect to friendship service: %v", err)
		return nil, err
	}

	// 初始化OSS客户端
	ossClient := oss.NewOSSClient(
		cfg.OSS.AccessKeyID,
		cfg.OSS.AccessKeySecret,
		cfg.OSS.Endpoint,
		cfg.OSS.BucketName,
	)

	return &UserGatewayHandler{
		userClient:       pb.NewUserServiceClient(userConn),
		messageClient:    msgPb.NewMessageServiceClient(msgConn),
		groupClient:      grpPb.NewGroupServiceClient(grpConn),
		friendshipClient: friendPb.NewFriendshipServiceClient(frConn),
		ossClient:        ossClient,
	}, nil
}

// ==================== 好友相关 API 转发 ====================

// SendFriendRequest POST /friends/requests
func (h *UserGatewayHandler) SendFriendRequest(c *gin.Context) {
	var req friendPb.SendFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	ctx := withAuthMetadata(c)
	res, err := h.friendshipClient.SendFriendRequest(ctx, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": res.Code, "message": res.Message, "request_id": res.RequestId})
}

// GetFriendRequests GET /friends/requests
func (h *UserGatewayHandler) GetFriendRequests(c *gin.Context) {
	statusStr := c.DefaultQuery("status", "pending")
	status := int32(0)
	switch statusStr {
	case "pending":
		status = 0
	case "approved":
		status = 1
	case "rejected":
		status = 2
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ctx := withAuthMetadata(c)
	res, err := h.friendshipClient.GetFriendRequests(ctx, &friendPb.GetFriendRequestsRequest{
		Status: int32(status),
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": res.Code, "message": res.Message, "requests": res.Requests, "total": res.Total})
}

// ProcessFriendRequest POST /friends/requests/handle
func (h *UserGatewayHandler) ProcessFriendRequest(c *gin.Context) {
	var req friendPb.ProcessFriendRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	ctx := withAuthMetadata(c)
	res, err := h.friendshipClient.ProcessFriendRequest(ctx, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": res.Code, "message": res.Message})
}

// GetFriends GET /friends
func (h *UserGatewayHandler) GetFriends(c *gin.Context) {
	ctx := withAuthMetadata(c)
	res, err := h.friendshipClient.GetFriends(ctx, &friendPb.GetFriendsRequest{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": res.Code, "message": res.Message, "data": res.Friends})
}

func (h *UserGatewayHandler) GetUserByID(c *gin.Context) {
	userID := c.Param("user_id")
	res, err := h.userClient.GetUserByID(c.Request.Context(), &pb.GetUserRequest{Id: userID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": res,
	})
}

func (h *UserGatewayHandler) CreateUser(c *gin.Context) {
	var req pb.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	res, err := h.userClient.CreateUser(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user: " + err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusBadRequest
	}

	c.JSON(statusCode, gin.H{
		"code":    res.Code,
		"message": res.Message,
		"user_id": res.UserId,
	})
}

// Login 处理 POST /api/v1/login 的请求
func (h *UserGatewayHandler) Login(c *gin.Context) {
	var req pb.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	res, err := h.userClient.Login(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login: " + err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusUnauthorized // 401
		c.JSON(statusCode, gin.H{
			"code":    res.Code,
			"message": res.Message,
		})
		return
	}

	// 👇 新增：登录成功后，自动拉取未读消息
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// 创建新的Authorization header（使用新的token）
		authHeader = "Bearer " + res.Token
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	// 并发拉取私聊和群聊未读消息
	type UnreadResult struct {
		privateRes *msgPb.PullUnreadMessagesResponse
		groupRes   *grpPb.PullAllGroupsUnreadMessagesResponse
		err        error
	}

	resultChan := make(chan UnreadResult, 2)

	// 拉取私聊未读
	go func() {
		res, err := h.messageClient.PullUnreadMessages(ctx, &msgPb.PullUnreadMessagesRequest{
			Limit:    100,
			AutoMark: false, // 只查看，不自动标记
		})
		resultChan <- UnreadResult{privateRes: res, err: err}
	}()

	// 拉取群聊未读
	go func() {
		res, err := h.groupClient.PullAllGroupsUnreadMessages(ctx, &grpPb.PullAllGroupsUnreadMessagesRequest{
			Limit: 20,
		})
		resultChan <- UnreadResult{groupRes: res, err: err}
	}()

	// 等待两个结果
	var privateResult, groupResult UnreadResult
	for i := 0; i < 2; i++ {
		result := <-resultChan
		if result.privateRes != nil {
			privateResult = result
		} else {
			groupResult = result
		}
	}

	// 构建未读消息响应（失败时返回空而不是错误）
	var privateUnreads interface{}
	var privateUnreadCount int32
	if privateResult.err == nil && privateResult.privateRes != nil {
		privateUnreads = privateResult.privateRes.Msgs
		privateUnreadCount = privateResult.privateRes.TotalUnread
	}

	var groupUnreads interface{}
	var groupUnreadCount int32
	if groupResult.err == nil && groupResult.groupRes != nil {
		groupUnreads = groupResult.groupRes.GroupUnreads
		groupUnreadCount = groupResult.groupRes.TotalUnreadCount
	}

	totalUnreadCount := privateUnreadCount + groupUnreadCount

	// 返回token和未读消息
	c.JSON(statusCode, gin.H{
		"code":                 res.Code,
		"message":              res.Message,
		"token":                res.Token,
		"private_unreads":      privateUnreads,
		"private_unread_count": privateUnreadCount,
		"group_unreads":        groupUnreads,
		"group_unread_count":   groupUnreadCount,
		"total_unread_count":   totalUnreadCount,
	})

	log.Printf("User logged in successfully, total unread messages: %d", totalUnreadCount)
}
func (h *UserGatewayHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	// 👇 核心改动：创建一个新的 context，并将 userID 放入 gRPC Metadata
	// Metadata 的 key 通常用小写，并用 - 连接
	md := metadata.New(map[string]string{"user-id": userID})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	// 👇 使用这个带有 Metadata 的新 context 来调用 gRPC
	res, err := h.userClient.GetCurrentUser(ctx, &pb.GetCurrentUserRequest{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user: " + err.Error()})
		return
	}

	// ... (后续的响应逻辑保持不变) ...
	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusBadRequest
	}

	c.JSON(statusCode, gin.H{
		"code":    res.Code,
		"message": res.Message,
		"data": map[string]string{
			"user_id":  res.UserId,
			"username": res.Username,
			"nickname": res.Nickname,
		},
	})
}
func (h *UserGatewayHandler) CheckUserOnline(c *gin.Context) {
	// 👇 从 URL 路径参数中获取 user_id
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	// 调用 gRPC 服务
	res, err := h.userClient.CheckUserOnline(c.Request.Context(), &pb.CheckUserOnlineRequest{
		UserId: userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user online status: " + err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, gin.H{
		"code":      res.Code,
		"message":   res.Message,
		"is_online": res.IsOnline,
	})
}

// SendMessage 发送消息的 HTTP 处理函数
func (h *UserGatewayHandler) SendMessage(c *gin.Context) {
	var req msgPb.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 👇 1. 从 HTTP Header 中获取完整的 Authorization Token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	// 👇 2. 创建 gRPC metadata，key 必须是 "authorization"
	//    value 就是完整的 Token 字符串
	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	// 👇 3. 使用这个带 metadata 的新上下文进行 gRPC 调用
	res, err := h.messageClient.SendMessage(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// SendGroupMessage 发送群聊消息的 HTTP 处理函数
func (h *UserGatewayHandler) SendGroupMessage(c *gin.Context) {
	var req msgPb.SendGroupMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.messageClient.SendGroupMessage(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// PullMessage 拉取按会话分组的消息（支持私聊和群聊）
func (h *UserGatewayHandler) PullMessage(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	autoMarkStr := c.DefaultQuery("auto_mark", "false")
	includeReadStr := c.DefaultQuery("include_read", "false")

	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	autoMark := autoMarkStr == "true" || autoMarkStr == "1"
	includeRead := includeReadStr == "true" || includeReadStr == "1"

	// 检验 token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	req := &msgPb.PullMessagesRequest{
		Limit:       limit,
		AutoMark:    autoMark,
		IncludeRead: includeRead,
	}

	res, err := h.messageClient.PullMessages(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回响应
	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// GetUnreadCount 获取未读消息数
func (h *UserGatewayHandler) GetUnreadCount(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.messageClient.GetUnreadCount(ctx, &msgPb.GetUnreadCountRequest{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// PullUnreadMessages 拉取所有未读消息
func (h *UserGatewayHandler) PullUnreadMessages(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	autoMarkStr := c.DefaultQuery("auto_mark", "true")

	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}

	// 将 true/false 字符串转换为布尔值
	autoMark := autoMarkStr == "true" || autoMarkStr == "1"

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	req := &msgPb.PullUnreadMessagesRequest{
		Limit:    limit,
		AutoMark: autoMark,
	}

	res, err := h.messageClient.PullUnreadMessages(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// ========== 群聊相关 API ==========

// CreateGroup 创建群组
func (h *UserGatewayHandler) CreateGroup(c *gin.Context) {
	var req grpPb.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.CreateGroup(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusBadRequest
	}

	c.JSON(statusCode, res)
}

// GetGroupInfo 获取群组信息
func (h *UserGatewayHandler) GetGroupInfo(c *gin.Context) {
	groupID := c.Param("group_id")

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.GetGroupInfo(ctx, &grpPb.GetGroupInfoRequest{GroupId: groupID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// AddGroupMember 添加群成员
func (h *UserGatewayHandler) AddGroupMember(c *gin.Context) {
	var req grpPb.AddGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	groupID := c.Param("group_id")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id is required"})
		return
	}
	req.GroupId = groupID

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.AddGroupMember(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// RemoveGroupMember 移除群成员
func (h *UserGatewayHandler) RemoveGroupMember(c *gin.Context) {
	var req grpPb.RemoveGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.RemoveGroupMember(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// LeaveGroup 离开群组
func (h *UserGatewayHandler) LeaveGroup(c *gin.Context) {
	groupID := c.Param("group_id")

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.LeaveGroup(ctx, &grpPb.LeaveGroupRequest{GroupId: groupID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// ListGroups 列出用户的所有群组
func (h *UserGatewayHandler) ListGroups(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.ListGroups(ctx, &grpPb.ListGroupsRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// PullAllUnreadMessages 拉取所有未读消息（私聊 + 群聊，用于上线一次性同步）
func (h *UserGatewayHandler) PullAllUnreadMessages(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization header"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	// 调用 Message Service 的 PullAllUnreadOnLogin 获取私聊 + 群聊未读
	res, err := h.messageClient.PullAllUnreadOnLogin(ctx, &msgPb.PullAllUnreadOnLoginRequest{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":               res.Code,
		"message":            res.Message,
		"private_unreads":    res.PrivateMessages,
		"group_unreads":      res.GroupMessages,
		"total_unread_count": res.TotalUnreadCount,
	})

	log.Printf("User %s pulled all unread messages from Message Service, total: %d", userID, res.TotalUnreadCount)
}

// ==================== 搜索功能接口 ====================

// SearchUsers 搜索用户
func (h *UserGatewayHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.userClient.SearchUsers(ctx, &pb.SearchUsersRequest{
		Keyword: keyword,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// SearchGroups 搜索群组
func (h *UserGatewayHandler) SearchGroups(c *gin.Context) {
	keyword := c.Query("keyword")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.SearchGroups(ctx, &grpPb.SearchGroupsRequest{
		Keyword: keyword,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// ==================== 文件上传相关接口 ====================

// GetUploadSignature 获取OSS上传签名
func (h *UserGatewayHandler) GetUploadSignature(c *gin.Context) {
	fileType := c.DefaultQuery("type", "file") // image 或 file

	// 验证文件类型
	if fileType != "image" && fileType != "file" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1001,
			"message": "无效的文件类型，只支持 image 或 file",
		})
		return
	}

	// 设置文件大小限制
	var maxSize int64
	if fileType == "image" {
		maxSize = 10 * 1024 * 1024 // 10MB
	} else {
		maxSize = 50 * 1024 * 1024 // 50MB
	}

	// 生成上传签名
	signature, err := h.ossClient.GenerateUploadSignature(fileType, maxSize)
	if err != nil {
		log.Printf("Failed to generate upload signature: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1002,
			"message": "生成上传签名失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "成功",
		"data":    signature,
	})
}

// ==================== 群加入请求相关接口 ====================

// SendGroupJoinRequest 发送群加入请求
func (h *UserGatewayHandler) SendGroupJoinRequest(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id" binding:"required"`
		Message string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.SendGroupJoinRequest(ctx, &grpPb.SendGroupJoinRequestRequest{
		GroupId: req.GroupID,
		Message: req.Message,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// HandleGroupJoinRequest 处理群加入请求（接受/拒绝）
func (h *UserGatewayHandler) HandleGroupJoinRequest(c *gin.Context) {
	var req struct {
		RequestID string `json:"request_id" binding:"required"`
		Action    int32  `json:"action" binding:"required"` // 1: 接受, 2: 拒绝
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.HandleGroupJoinRequest(ctx, &grpPb.HandleGroupJoinRequestRequest{
		RequestId: req.RequestID,
		Action:    req.Action,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// GetGroupJoinRequests 获取群的加入申请列表（管理员查看）
func (h *UserGatewayHandler) GetGroupJoinRequests(c *gin.Context) {
	groupID := c.Param("group_id")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id is required"})
		return
	}

	statusStr := c.DefaultQuery("status", "0") // 0: all, 1: pending, 2: accepted, 3: rejected
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	status, _ := strconv.ParseInt(statusStr, 10, 32)
	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.GetGroupJoinRequests(ctx, &grpPb.GetGroupJoinRequestsRequest{
		GroupId: groupID,
		Status:  int32(status),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// GetMyGroupJoinRequests 获取我的加入申请列表
func (h *UserGatewayHandler) GetMyGroupJoinRequests(c *gin.Context) {
	statusStr := c.DefaultQuery("status", "0") // 0: all, 1: pending, 2: accepted, 3: rejected
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	status, _ := strconv.ParseInt(statusStr, 10, 32)
	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.GetMyGroupJoinRequests(ctx, &grpPb.GetMyGroupJoinRequestsRequest{
		Status: int32(status),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// ==================== 群组管理功能相关接口 ====================

// UpdateGroupInfo 修改群信息
func (h *UserGatewayHandler) UpdateGroupInfo(c *gin.Context) {
	var req struct {
		GroupID     string `json:"group_id" binding:"required"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.UpdateGroupInfo(ctx, &grpPb.UpdateGroupInfoRequest{
		GroupId:     req.GroupID,
		Name:        req.Name,
		Description: req.Description,
		Avatar:      req.Avatar,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// TransferGroupOwner 转让群主
func (h *UserGatewayHandler) TransferGroupOwner(c *gin.Context) {
	var req struct {
		GroupID    string `json:"group_id" binding:"required"`
		NewOwnerID string `json:"new_owner_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.TransferOwner(ctx, &grpPb.TransferOwnerRequest{
		GroupId:    req.GroupID,
		NewOwnerId: req.NewOwnerID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// DismissGroup 解散群组
func (h *UserGatewayHandler) DismissGroup(c *gin.Context) {
	groupID := c.Param("group_id")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id is required"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.DismissGroup(ctx, &grpPb.DismissGroupRequest{
		GroupId: groupID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// SetGroupAdmin 设置/取消管理员
func (h *UserGatewayHandler) SetGroupAdmin(c *gin.Context) {
	var req struct {
		GroupID string `json:"group_id" binding:"required"`
		UserID  string `json:"user_id" binding:"required"`
		IsAdmin bool   `json:"is_admin"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.SetAdmin(ctx, &grpPb.SetAdminRequest{
		GroupId: req.GroupID,
		UserId:  req.UserID,
		IsAdmin: req.IsAdmin,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}

// GetGroupMembers 获取群成员列表
func (h *UserGatewayHandler) GetGroupMembers(c *gin.Context) {
	groupID := c.Param("group_id")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}

	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	res, err := h.groupClient.GetGroupMembers(ctx, &grpPb.GetGroupMembersRequest{
		GroupId: groupID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, res)
}
