// cmd/user/main.go
package main

import (
	"context"
	"net"

	pb "ChatIM/api/proto/user"
	"ChatIM/internal/user_service/handler"
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"ChatIM/pkg/logger"
	"ChatIM/pkg/migrations"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	logger.Info("=== User Service starting ===")

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

	// 3. 初始化 Redis 连接
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Redis.Addr,
		Password: cfg.Database.Redis.Password,
		DB:       cfg.Database.Redis.DB,
	})
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	logger.Info("✅ Successfully connected to Redis")

	// 4. 创建 gRPC 服务
	userHandler := handler.NewUserHandler(db, rdb)
	grpcSrv := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcSrv, userHandler)

	// 5. 启动 gRPC 监听
	lis, err := net.Listen("tcp", cfg.Server.UserGRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port",
			zap.String("port", cfg.Server.UserGRPCPort),
			zap.Error(err))
	}
	logger.Info("🚀 User Service gRPC server started",
		zap.String("port", cfg.Server.UserGRPCPort))

	if err := grpcSrv.Serve(lis); err != nil {
		logger.Fatal("Failed to serve gRPC", zap.Error(err))
	}
}
