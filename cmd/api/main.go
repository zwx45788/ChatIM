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

	// Serve static frontend without conflicting with /api routes
	r.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})
	r.Static("/web", "./web")

	log.Println("Creating UserGatewayHandler...")
	userHandler, err := handler.NewUserGatewayHandler()
	if err != nil {
		log.Fatalf("Failed to initialize user gateway handler: %v", err)
	}
	log.Println("UserGatewayHandler created successfully")

	log.Println("Creating ConversationHandler...")
	conversationHandler, err := handler.NewConversationHandler()
	if err != nil {
		log.Fatalf("Failed to initialize conversation handler: %v", err)
	}
	log.Println("ConversationHandler created successfully")

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
			protected.POST("/groups/messages", userHandler.SendGroupMessage) // 📌 发送群聊消息

			// ========== 群加入请求相关路由 ==========
			protected.POST("/groups/join-requests", userHandler.SendGroupJoinRequest)          // 📌 发送加群申请
			protected.POST("/groups/join-requests/handle", userHandler.HandleGroupJoinRequest) // 📌 处理加群申请（接受/拒绝）
			protected.GET("/groups/:group_id/join-requests", userHandler.GetGroupJoinRequests) // 📌 获取群的加入申请列表（管理员）
			protected.GET("/groups/join-requests/my", userHandler.GetMyGroupJoinRequests)      // 📌 获取我的加入申请列表

			// ========== 群组管理功能路由 ==========
			protected.PUT("/groups/:group_id/info", userHandler.UpdateGroupInfo)         // 📌 修改群信息
			protected.POST("/groups/:group_id/transfer", userHandler.TransferGroupOwner) // 📌 转让群主
			protected.POST("/groups/:group_id/dismiss", userHandler.DismissGroup)        // 📌 解散群组
			protected.POST("/groups/:group_id/admin", userHandler.SetGroupAdmin)         // 📌 设置/取消管理员
			protected.GET("/groups/:group_id/members", userHandler.GetGroupMembers)      // 📌 获取群成员列表

			// ========== 搜索功能路由 ==========
			protected.GET("/search/users", userHandler.SearchUsers)   // 📌 搜索用户
			protected.GET("/search/groups", userHandler.SearchGroups) // 📌 搜索群组

			// ========== 会话列表相关路由 ==========
			protected.GET("/conversations", conversationHandler.GetConversationList)                       // 📌 获取会话列表
			protected.POST("/conversations/:conversation_id/pin", conversationHandler.PinConversation)     // 📌 置顶会话
			protected.DELETE("/conversations/:conversation_id/pin", conversationHandler.UnpinConversation) // 📌 取消置顶
			protected.DELETE("/conversations/:conversation_id", conversationHandler.DeleteConversation)    // 📌 删除会话
		}
	}
	r.GET("/ws", middleware.AuthMiddleware(), hub.HandleWebSocket)
	log.Printf("API Gateway is running on :%v...", cfg.Server.APIPort)
	if err := r.Run(cfg.Server.APIPort); err != nil {
		log.Fatalf("Failed to run API Gateway: %v", err)
	}
}
