package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	grpPb "ChatIM/api/proto/group"
	pb "ChatIM/api/proto/user"
	"ChatIM/internal/api_gateway/middleware"
	"ChatIM/pkg/config"
	"ChatIM/pkg/logger"
	"ChatIM/pkg/stream"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ConversationHandler 会话管理处理器
type ConversationHandler struct {
	streamOp    *stream.StreamOperator
	rdb         *redis.Client
	userClient  pb.UserServiceClient
	groupClient grpPb.GroupServiceClient
}

// NewConversationHandler 创建会话处理器
func NewConversationHandler() (*ConversationHandler, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config for conversation handler", zap.Error(err))
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Redis.Addr,
		Password: cfg.Database.Redis.Password,
		DB:       cfg.Database.Redis.DB,
	})

	// 连接到 user-service
	userAddr := cfg.Server.UserGRPCAddr
	if userAddr == "" {
		userAddr = "127.0.0.1" + cfg.Server.UserGRPCPort
	}
	logger.Info("ConversationHandler connecting to User Service", zap.String("addr", userAddr))

	userConn, err := grpc.Dial(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to user service", zap.Error(err))
		return nil, err
	}

	// 连接到 group-service
	groupAddr := cfg.Server.GroupGRPCAddr
	if groupAddr == "" {
		groupAddr = "127.0.0.1" + cfg.Server.GroupGRPCPort
	}
	logger.Info("ConversationHandler connecting to Group Service", zap.String("addr", groupAddr))

	grpConn, err := grpc.Dial(groupAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to group service", zap.Error(err))
		return nil, err
	}

	return &ConversationHandler{
		streamOp:    stream.NewStreamOperator(rdb),
		rdb:         rdb,
		userClient:  pb.NewUserServiceClient(userConn),
		groupClient: grpPb.NewGroupServiceClient(grpConn),
	}, nil
}

// ConversationResponse 会话响应结构
type ConversationResponse struct {
	ConversationID  string                 `json:"conversation_id"`   // "private:user_123" 或 "group:group_456"
	Type            string                 `json:"type"`              // "private" 或 "group"
	PeerID          string                 `json:"peer_id"`           // 对方用户ID或群组ID
	Title           string                 `json:"title"`             // 显示名称
	Avatar          string                 `json:"avatar"`            // 头像URL
	LastMessage     string                 `json:"last_message"`      // 最后一条消息内容
	LastMessageTime int64                  `json:"last_message_time"` // 毫秒时间戳
	UnreadCount     int                    `json:"unread_count"`
	IsPinned        bool                   `json:"is_pinned"`
	Extra           map[string]interface{} `json:"extra,omitempty"` // 额外信息
}

// GetConversationList 获取会话列表
// GET /api/v1/conversations?offset=0&limit=20
func (h *ConversationHandler) GetConversationList(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	// 获取分页参数
	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "20")

	offset, _ := strconv.ParseInt(offsetStr, 10, 64)
	limit, _ := strconv.ParseInt(limitStr, 10, 64)

	if limit > 100 {
		limit = 100 // 限制最大值
	}

	// 从 Redis 获取会话列表
	conversations, err := h.streamOp.GetConversationList(c.Request.Context(), userID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get conversations"})
		return
	}

	// 补充会话详细信息
	var responseList []ConversationResponse
	for _, conv := range conversations {
		response := h.enrichConversationInfo(c.Request.Context(), userID, conv)
		responseList = append(responseList, response)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":          0,
		"message":       "success",
		"conversations": responseList,
		"total":         len(responseList),
		"has_more":      len(responseList) == int(limit),
	})
}

// PinConversation 置顶会话
// POST /api/v1/conversations/:conversation_id/pin
func (h *ConversationHandler) PinConversation(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}

	err := h.streamOp.PinConversation(c.Request.Context(), userID, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pin conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Conversation pinned successfully",
	})
}

// UnpinConversation 取消置顶会话
// DELETE /api/v1/conversations/:conversation_id/pin
func (h *ConversationHandler) UnpinConversation(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}

	err := h.streamOp.UnpinConversation(c.Request.Context(), userID, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unpin conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Conversation unpinned successfully",
	})
}
func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	var req struct {
		ConversationID string `json:"conversation_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}

	err := h.streamOp.CreateConversation(c.Request.Context(), userID, req.ConversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Conversation created successfully",
	})
}

// DeleteConversation 删除会话
// DELETE /api/v1/conversations/:conversation_id
func (h *ConversationHandler) DeleteConversation(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
		return
	}

	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}

	err := h.streamOp.DeleteConversation(c.Request.Context(), userID, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete conversation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Conversation deleted successfully",
	})
}

// enrichConversationInfo 补充会话详细信息（标题、头像、最后消息等）
func (h *ConversationHandler) enrichConversationInfo(ctx context.Context, userID string, conv stream.ConversationItem) ConversationResponse {
	response := ConversationResponse{
		ConversationID:  conv.ConversationID,
		LastMessageTime: conv.LastMessageTime,
		IsPinned:        conv.IsPinned,
	}

	// 解析会话类型和对方ID
	if len(conv.ConversationID) > 8 && conv.ConversationID[:8] == "private:" {
		response.Type = "private"
		response.PeerID = conv.ConversationID[8:]

		// 查询对方用户信息
		peerInfo := h.getUserInfo(ctx, response.PeerID)
		response.Title = peerInfo["nickname"]
		response.Avatar = peerInfo["avatar"]

	} else if len(conv.ConversationID) > 6 && conv.ConversationID[:6] == "group:" {
		response.Type = "group"
		response.PeerID = conv.ConversationID[6:]

		// 查询群组信息
		groupInfo := h.getGroupInfo(ctx, response.PeerID)
		response.Title = groupInfo["name"]
		response.Avatar = groupInfo["avatar"]
	}

	// 获取最后一条消息（从 Stream 读取）
	lastMsg := h.getLastMessage(ctx, userID, conv.ConversationID)
	response.LastMessage = lastMsg

	// 获取未读数（从 Stream 统计）
	response.UnreadCount = h.getUnreadCount(ctx, userID, conv.ConversationID)

	return response
}

// getUserInfo 获取用户信息
func (h *ConversationHandler) getUserInfo(ctx context.Context, userID string) map[string]string {
	// 调用 User Service 获取用户信息
	res, err := h.userClient.GetUserByID(ctx, &pb.GetUserRequest{Id: userID})
	if err != nil {
		logger.Warn("Failed to get user info", zap.String("user_id", userID), zap.Error(err))
		// 返回默认值
		return map[string]string{
			"nickname": "User_" + userID[len(userID)-4:],
			"avatar":   "",
		}
	}

	// 使用 nickname，如果为空则使用 username
	displayName := res.Nickname
	if displayName == "" {
		displayName = res.Username
	}

	// TODO: proto 定义中暂时没有 avatar 字段，后续可以添加
	return map[string]string{
		"nickname": displayName,
		"avatar":   "",
	}
}

// getGroupInfo 获取群组信息
func (h *ConversationHandler) getGroupInfo(ctx context.Context, groupID string) map[string]string {
	// 调用 Group Service 获取群组信息
	res, err := h.groupClient.GetGroupInfo(ctx, &grpPb.GetGroupInfoRequest{GroupId: groupID})
	if err != nil {
		logger.Warn("Failed to get group info", zap.String("group_id", groupID), zap.Error(err))
		// 返回默认值
		return map[string]string{
			"name":   "Group_" + groupID[len(groupID)-4:],
			"avatar": "",
		}
	}

	if res.Code != 0 || res.Group == nil {
		logger.Warn("Failed to get group info", zap.String("message", res.Message))
		return map[string]string{
			"name":   "Group_" + groupID[len(groupID)-4:],
			"avatar": "",
		}
	}

	// TODO: proto 定义中暂时没有 avatar 字段，后续可以添加
	return map[string]string{
		"name":   res.Group.Name,
		"avatar": "",
	}
}

// getLastMessage 获取最后一条消息内容
func (h *ConversationHandler) getLastMessage(ctx context.Context, userID, conversationID string) string {
	streamKey := fmt.Sprintf("stream:private:%s", userID)

	// 读取最后一条消息
	messages, err := h.rdb.XRevRangeN(ctx, streamKey, "+", "-", 20).Result()
	if err != nil {
		return ""
	}

	// 查找该会话的最后一条消息
	for _, msg := range messages {
		if conversationID[:8] == "private:" {
			// 私聊消息
			if msg.Values["to_user_id"] == conversationID[8:] || msg.Values["from_user_id"] == conversationID[8:] {
				if content, ok := msg.Values["content"].(string); ok {
					return truncateString(content, 50)
				}
			}
		} else if conversationID[:6] == "group:" {
			// 群聊消息
			if msg.Values["group_id"] == conversationID[6:] {
				if content, ok := msg.Values["content"].(string); ok {
					return truncateString(content, 50)
				}
			}
		}
	}

	return ""
}

// getUnreadCount 获取未读消息数
func (h *ConversationHandler) getUnreadCount(ctx context.Context, userID, conversationID string) int {
	streamKey := fmt.Sprintf("stream:private:%s", userID)

	messages, err := h.rdb.XRevRangeN(ctx, streamKey, "+", "-", 100).Result()
	if err != nil {
		return 0
	}

	count := 0
	for _, msg := range messages {
		isRead := msg.Values["is_read"] == "true"
		if isRead {
			continue
		}

		// 判断是否属于该会话
		if conversationID[:8] == "private:" {
			if msg.Values["to_user_id"] == conversationID[8:] || msg.Values["from_user_id"] == conversationID[8:] {
				count++
			}
		} else if conversationID[:6] == "group:" {
			if msg.Values["group_id"] == conversationID[6:] {
				count++
			}
		}
	}

	return count
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// FormatTime 格式化时间显示
func FormatTime(timestamp int64) string {
	t := time.Unix(timestamp/1000, 0)
	now := time.Now()

	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		// 今天：显示时分
		return t.Format("15:04")
	} else if t.Year() == now.Year() && t.YearDay() == now.YearDay()-1 {
		// 昨天
		return "昨天"
	} else if t.Year() == now.Year() {
		// 今年：显示月日
		return t.Format("01/02")
	} else {
		// 往年：显示年月日
		return t.Format("2006/01/02")
	}
}
