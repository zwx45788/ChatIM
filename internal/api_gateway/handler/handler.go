package handler

import (
	"net/http"

	pb "ChatIM/api/proto/user"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserGatewayHandler struct {
	userClient pb.UserServiceClient
}

func NewUserGatewayHandler() (*UserGatewayHandler, error) {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := pb.NewUserServiceClient(conn)
	return &UserGatewayHandler{
		userClient: client,
	}, nil
}

// GetUserByID 处理 GET /api/v1/users/:id 的请求
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

// CreateUser 处理 POST /api/v1/users 的请求
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

	statusCode := http.StatusCreated
	if res.Code != 0 {
		statusCode = http.StatusBadRequest
	}

	c.JSON(statusCode, gin.H{
		"code":    res.Code,
		"message": res.Message,
		"data":    gin.H{"user_id": res.UserId},
	})
}
