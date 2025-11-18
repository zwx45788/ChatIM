// cmd/api_gateway/main.go

package main

import (
	"log"

	"ChatIM/internal/api_gateway/handler"

	"github.com/gin-gonic/gin"
)

func main() {
	// 创建 Gin 路由器
	r := gin.Default()

	// 初始化 User Gateway Handler
	userHandler, err := handler.NewUserGatewayHandler()
	if err != nil {
		log.Fatalf("Failed to initialize user gateway handler: %v", err)
	}

	// 设置路由
	// 当访问 GET /api/v1/users/123 时，会调用 userHandler.GetUserByID
	api := r.Group("/api/v1")
	{
		api.GET("/users/:id", userHandler.GetUserByID)
		// 以后可以添加更多路由，比如：
		api.POST("/users", userHandler.CreateUser)
	}

	// 启动 HTTP 服务器
	log.Println("API Gateway is running on :8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to run API Gateway: %v", err)
	}
}
