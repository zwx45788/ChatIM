package handler

import (
	"log"
	"net/http"
	"strconv"

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
	userConn, err := grpc.Dial("127.0.0.1"+cfg.Server.UserGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("did not connect to user service: %v", err)
		return nil, err
	}

	// 连接到 message-service
	msgConn, err := grpc.Dial("127.0.0.1"+cfg.Server.MessageGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("did not connect to message service: %v", err)
		return nil, err
	}

	return &UserGatewayHandler{
		userClient:    pb.NewUserServiceClient(userConn),
		messageClient: msgPb.NewMessageServiceClient(msgConn),
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
	}

	c.JSON(statusCode, gin.H{
		"code":    res.Code,
		"message": res.Message,
		"token":   res.Token, // 返回 token
	})
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
