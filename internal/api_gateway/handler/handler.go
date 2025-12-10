package handler

import (
	"log"
	"net/http"
	"strconv"

	grpPb "ChatIM/api/proto/group"
	msgPb "ChatIM/api/proto/message"
	pb "ChatIM/api/proto/user"
	"ChatIM/internal/api_gateway/middleware"
	"ChatIM/pkg/config"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type UserGatewayHandler struct {
	userClient    pb.UserServiceClient
	messageClient msgPb.MessageServiceClient
	groupClient   grpPb.GroupServiceClient
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

	return &UserGatewayHandler{
		userClient:    pb.NewUserServiceClient(userConn),
		messageClient: msgPb.NewMessageServiceClient(msgConn),
		groupClient:   grpPb.NewGroupServiceClient(grpConn),
	}, nil
}

func (h *UserGatewayHandler) GetUserByID(c *gin.Context) {
	userID := c.Param("id")
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
func (h *UserGatewayHandler) PullMessage(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
		return
	}
	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset parameter"})
		return
	}
	//检验token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		return
	}
	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)
	req := &msgPb.PullMessagesRequest{
		Limit:  limit,
		Offset: offset,
	}
	res, err := h.messageClient.PullMessages(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 👇 5. 返回响应
	statusCode := http.StatusOK
	if res.Code != 0 {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, res)
}

// MarkMessagesAsRead 标记消息已读
func (h *UserGatewayHandler) MarkMessagesAsRead(c *gin.Context) {
	var req msgPb.MarkMessagesAsReadRequest
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

	res, err := h.messageClient.MarkMessagesAsRead(ctx, &req)
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
