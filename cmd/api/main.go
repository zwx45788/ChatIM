package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"ChatIM/internal/api_gateway/handler"
	"ChatIM/internal/api_gateway/middleware"
)

func main() {
	r := gin.Default()

	userHandler, err := handler.NewUserGatewayHandler()
	if err != nil {
		log.Fatalf("Failed to initialize user gateway handler: %v", err)
	}

	// 设置路由
	api := r.Group("/api/v1")
	{
		api.GET("/users/:user_id", userHandler.GetUserByID)
		api.POST("/users", userHandler.CreateUser)
		api.POST("/login", userHandler.Login)
		api.GET("/users/:user_id/online", userHandler.CheckUserOnline)
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware()) // 👈 应用认证中间件
		{
			protected.GET("/users/me", userHandler.GetCurrentUser) // 👈 获取当前用户信息
			// 以后其他需要认证的路由都加在这里
			// protected.PUT("/users/me", userHandler.UpdateCurrentUser)
			protected.POST("/messages/send", userHandler.SendMessage)
		}
	}

	log.Println("API Gateway is running on :8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to run API Gateway: %v", err)
	}
}
