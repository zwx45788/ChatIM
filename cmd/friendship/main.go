package main

import (
	"net"

	pb "ChatIM/api/proto/friendship"
	"ChatIM/internal/friendship/handler"
	"ChatIM/internal/friendship/repository"
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"ChatIM/pkg/logger"
	"ChatIM/pkg/migrations"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 1. 加载配置
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

	logger.Info("=== Friendship Service starting ===")

	// 2. 初始化数据库连接
	db, err := database.InitDB(cfg.Database.MySQL.DSN)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// 2.5 运行数据库迁移
	logger.Info("Running database migrations...")
	if err := migrations.RunMigrations(db); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// 3. 创建 gRPC 服务器
	grpcSrv := grpc.NewServer()

	// 4. 初始化仓储层和处理器
	friendshipRepo := repository.NewFriendshipRepository(db)
	friendshipHandler := handler.NewFriendshipHandler(friendshipRepo)

	// 5. 注册 FriendshipService
	pb.RegisterFriendshipServiceServer(grpcSrv, friendshipHandler)
	reflection.Register(grpcSrv)

	// 6. 启动 gRPC 监听
	port := cfg.Server.FriendshipGRPCPort
	if port == "" {
		port = ":50053" // 默认端口
	}
	lis, err := net.Listen("tcp", port)
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port",
			zap.String("port", port),
			zap.Error(err))
	}

	logger.Info("🚀 Friendship Service gRPC server started",
		zap.String("port", port))

	if err := grpcSrv.Serve(lis); err != nil {
		logger.Fatal("Failed to serve gRPC", zap.Error(err))
	}
}
