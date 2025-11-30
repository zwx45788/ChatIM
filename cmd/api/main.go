package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"ChatIM/internal/api_gateway/handler"
	"ChatIM/internal/api_gateway/middleware"
	"ChatIM/internal/websocket"
	"ChatIM/pkg/config"
)

func main() {
	r := gin.Default()
	hub := websocket.NewHub()
	go hub.Run()
	go websocket.StartSubscriber(hub)
	userHandler, err := handler.NewUserGatewayHandler()
	if err != nil {
		log.Fatalf("Failed to initialize user gateway handler: %v", err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
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
			protected.GET("/messages", userHandler.PullMessage)
		}
	}
	r.GET("/ws", middleware.AuthMiddleware(), hub.HandleWebSocket)
	log.Printf("API Gateway is running on :%v...", cfg.Server.APIPort)
	if err := r.Run(cfg.Server.APIPort); err != nil {
		log.Fatalf("Failed to run API Gateway: %v", err)
	}
}
