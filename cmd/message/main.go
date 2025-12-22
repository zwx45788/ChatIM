package main

import (
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"ChatIM/pkg/logger"
	"net"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "ChatIM/api/proto/message"
	"ChatIM/internal/message_service/handler"
)

func main() {
	// 1. 初始化数据源
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

	logger.Info("=== Message Service starting ===")

	db, err := database.InitDB(cfg.Database.MySQL.DSN)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// 2. 创建 gRPC 服务器
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Redis.Addr,
		Password: cfg.Database.Redis.Password,
		DB:       cfg.Database.Redis.DB,
	})
	logger.Info("✅ Redis client initialized")

	grpcSrv := grpc.NewServer()

	lis, err := net.Listen("tcp", cfg.Server.MessageGRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port",
			zap.String("port", cfg.Server.MessageGRPCPort),
			zap.Error(err))
	}

	// 3. 注册服务
	pb.RegisterMessageServiceServer(grpcSrv, handler.NewMessageHandler(db, rdb))
	reflection.Register(grpcSrv)

	logger.Info("🚀 Message Service gRPC server started",
		zap.String("port", cfg.Server.MessageGRPCPort))

	if err := grpcSrv.Serve(lis); err != nil {
		logger.Fatal("Failed to serve gRPC", zap.Error(err))
	}

	// // 4. 优雅启动
	// // ... (和 user_service 一样的优雅关闭逻辑) ...
	// r := gin.Default()
	// // ... 可以添加一些 HTTP 路由 ...

	// stop := func() {
	// 	log.Println("Shutting down gRPC server...")
	// 	grpcSrv.GracefulStop()
	// }

	// pkg.Run(r, "User Service HTTP", "127.0.0.1:8080", stop)
}
