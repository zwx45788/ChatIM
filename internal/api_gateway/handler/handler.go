package handler

import (
	"net/http"

	pb "ChatIM/api/proto/user"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserGatewayHandler 封装了 gRPC 客户端
type UserGatewayHandler struct {
	userClient pb.UserServiceClient
}

// NewUserGatewayHandler 创建一个新的 handler
func NewUserGatewayHandler() (*UserGatewayHandler, error) {
	// 连接到 user-service
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// 创建 gRPC 客户端
	client := pb.NewUserServiceClient(conn)

	return &UserGatewayHandler{
		userClient: client,
	}, nil
}

// GetUserByID 处理 GET /api/v1/users/:id 的请求
func (h *UserGatewayHandler) GetUserByID(c *gin.Context) {
	// 从 URL 路径中获取用户 ID
	userID := c.Param("id")

	// 调用后端的 gRPC 服务
	// 注意：这里我们使用 context.Background()，在实际项目中应该传递 gin.Context
	res, err := h.userClient.GetUserByID(c.Request.Context(), &pb.GetUserRequest{Id: userID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 将 gRPC 的响应转换成 JSON 返回给客户端
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": res,
	})
}
