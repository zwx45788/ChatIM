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
	log.Println("=== API Gateway starting ===")
	r := gin.Default()
	hub := websocket.NewHub()
	go hub.Run()
	go websocket.StartSubscriber(hub)

	log.Println("Creating UserGatewayHandler...")
	userHandler, err := handler.NewUserGatewayHandler()
	if err != nil {
		log.Fatalf("Failed to initialize user gateway handler: %v", err)
	}
	log.Println("UserGatewayHandler created successfully")

	log.Println("Loading config...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("Config loaded successfully")
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
			protected.POST("/messages/read", userHandler.MarkMessagesAsRead)
			protected.GET("/messages/unread", userHandler.GetUnreadCount)
			protected.GET("/messages/unread/pull", userHandler.PullUnreadMessages)

			// ========== 统一上线初始化接口 ==========
			protected.GET("/unread/all", userHandler.PullAllUnreadMessages) // 📌 一次性拉取私聊+群聊未读

			// ========== 群聊相关路由 ==========
			protected.POST("/groups", userHandler.CreateGroup)
			protected.GET("/groups/:group_id", userHandler.GetGroupInfo)
			protected.GET("/groups", userHandler.ListGroups)
			protected.POST("/groups/:group_id/members", userHandler.AddGroupMember)
			protected.DELETE("/groups/:group_id/members", userHandler.RemoveGroupMember)
			protected.DELETE("/groups/:group_id", userHandler.LeaveGroup)
		}
	}
	r.GET("/ws", middleware.AuthMiddleware(), hub.HandleWebSocket)
	log.Printf("API Gateway is running on :%v...", cfg.Server.APIPort)
	if err := r.Run(cfg.Server.APIPort); err != nil {
		log.Fatalf("Failed to run API Gateway: %v", err)
	}
}
