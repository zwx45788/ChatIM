package main

import (
	"crypto/sha256"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"ChatIM/internal/api_gateway/handler"
	"ChatIM/internal/api_gateway/middleware"
	"ChatIM/internal/websocket"
	"ChatIM/pkg/config"
	"ChatIM/pkg/logger"
	"ChatIM/pkg/profiling"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	// 初始化 logger
	if err := logger.InitLogger(logger.Config{
		Level:      cfg.Log.Level,
		OutputPath: cfg.Log.OutputPath,
		DevMode:    cfg.Log.DevMode,
	}); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("=== API Gateway starting ===")

	// 初始化 pprof 性能分析
	profiling.InitProfiling("6060")

	// 启动 Prometheus Metrics 服务（独立端口）
	go func() {
		metricsRouter := gin.New()
		metricsRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))
		logger.Info("📊 Prometheus metrics server started at http://localhost:9090/metrics")
		if err := metricsRouter.Run(":9090"); err != nil {
			logger.Error("❌ Failed to start metrics server", zap.Error(err))
		}
	}()
	// CORS：放行本地开发常见来源（包含 file:// 的 Origin: null）
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())
	// 添加 Prometheus 中间件
	r.Use(middleware.PrometheusMiddleware())

	hub := websocket.NewHub()
	go hub.Run()
	go websocket.StartSubscriber(hub)

	// Serve static frontend without conflicting with /api routes
	r.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})
	r.Static("/web", "./web")

	logger.Info("Creating UserGatewayHandler...")
	userHandler, err := handler.NewUserGatewayHandler()
	if err != nil {
		logger.Fatal("Failed to initialize user gateway handler", zap.Error(err))
	}
	logger.Info("UserGatewayHandler created successfully")

	logger.Info("Creating ConversationHandler...")
	conversationHandler, err := handler.NewConversationHandler()
	if err != nil {
		logger.Fatal("Failed to initialize conversation handler", zap.Error(err))
	}
	logger.Info("ConversationHandler created successfully")
	// 设置路由
	api := r.Group("/api/v1")
	{
		// CPU 压力测试端点：/api/v1/debug/cpu-burn?seconds=10&workers=0
		// workers=0 表示使用 runtime.NumCPU()
		api.GET("/debug/cpu-burn", func(c *gin.Context) {
			secStr := c.DefaultQuery("seconds", "10")
			workersStr := c.DefaultQuery("workers", "0")
			seconds, err := strconv.Atoi(secStr)
			if err != nil || seconds <= 0 {
				seconds = 10
			}
			workers, err := strconv.Atoi(workersStr)
			if err != nil || workers <= 0 {
				workers = runtime.NumCPU()
			}

			deadline := time.Now().Add(time.Duration(seconds) * time.Second)
			var ops uint64
			var wg sync.WaitGroup

			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					// 纯CPU计算：重复sha256，尽量少内存分配
					sum := [32]byte{}
					data := []byte{byte(id)}
					for time.Now().Before(deadline) {
						h := sha256.Sum256(append(data, sum[0]))
						sum = h
						atomic.AddUint64(&ops, 1)
					}
				}(i)
			}

			wg.Wait()
			c.JSON(http.StatusOK, gin.H{
				"workers":     workers,
				"seconds":     seconds,
				"ops":         ops,
				"gomaxprocs":  runtime.GOMAXPROCS(0),
				"num_cpu":     runtime.NumCPU(),
				"finished_at": time.Now().Format(time.RFC3339),
			})
		})

		api.GET("/users/:user_id", userHandler.GetUserByID)
		api.POST("/users", userHandler.CreateUser)
		api.POST("/login", userHandler.Login)
		api.GET("/users/:user_id/online", userHandler.CheckUserOnline)
		protected := api.Group("/")
		protected.Use(middleware.AuthMiddleware()) // 👈 应用认证中间件
		{
			protected.POST("/logout", userHandler.Logout)          // 👈 注册 Logout 路由
			protected.GET("/users/me", userHandler.GetCurrentUser) // 👈 获取当前用户信息
			// 以后其他需要认证的路由都加在这里
			// protected.PUT("/users/me", userHandler.UpdateCurrentUser)
			protected.POST("/messages/send", userHandler.SendMessage)
			protected.GET("/messages", userHandler.PullMessage)
			// protected.GET("/messages/unread", userHandler.GetUnreadCount) // 已弃用：未读数由前端计算
			protected.POST("/messages/cursor", userHandler.UpdateLastSeenCursor) // 更新已读游标
			// 标记消息为已读
			protected.POST("/messages/read", userHandler.MarkPrivateMessageAsRead)
			protected.POST("/groups/:group_id/read", userHandler.MarkGroupMessageAsRead)
			// NOTE: `/messages/unread/pull` and `/unread/all` have been deprecated and removed from routes.
			// 登录时请改为调用 `/messages` (PullMessage) 并结合 `/messages/unread` (GetUnreadCount)。

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

			// ========== 文件上传路由 ==========
			protected.GET("/upload/signature", userHandler.GetUploadSignature) // 📌 获取OSS上传签名

			// ========== 好友相关路由 ==========
			protected.POST("/friends/requests", userHandler.SendFriendRequest)           // 发送好友请求
			protected.GET("/friends/requests", userHandler.GetFriendRequests)            // 获取好友请求列表
			protected.POST("/friends/requests/handle", userHandler.ProcessFriendRequest) // 处理好友请求
			protected.GET("/friends", userHandler.GetFriends)                            // 获取好友列表

			// ========== 会话列表相关路由 ==========
			protected.GET("/conversations", conversationHandler.GetConversationList)                       // 📌 获取会话列表
			protected.POST("/conversations", conversationHandler.CreateConversation)                       // 📌 创建会话
			protected.POST("/conversations/:conversation_id/pin", conversationHandler.PinConversation)     // 📌 置顶会话
			protected.DELETE("/conversations/:conversation_id/pin", conversationHandler.UnpinConversation) // 📌 取消置顶
			protected.DELETE("/conversations/:conversation_id", conversationHandler.DeleteConversation)    // 📌 删除会话
		}
	}
	r.GET("/ws", middleware.AuthMiddleware(), hub.HandleWebSocket)
	logger.Info("API Gateway is running", zap.String("port", cfg.Server.APIPort))

	if cfg.Server.CertFile != "" && cfg.Server.KeyFile != "" {
		logger.Info("Starting API Gateway with TLS", zap.String("cert", cfg.Server.CertFile), zap.String("key", cfg.Server.KeyFile))
		if err := r.RunTLS(cfg.Server.APIPort, cfg.Server.CertFile, cfg.Server.KeyFile); err != nil {
			logger.Fatal("Failed to run API Gateway with TLS", zap.Error(err))
		}
	} else {
		if err := r.Run(cfg.Server.APIPort); err != nil {
			logger.Fatal("Failed to run API Gateway", zap.Error(err))
		}
	}
}
