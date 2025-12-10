// cmd/user/main.go
package main

import (
	"context"
	"log"
	"net"

	pb "ChatIM/api/proto/user"
	"ChatIM/internal/user_service/handler"
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"ChatIM/pkg/migrations"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. 初始化数据库连接
	db, err := database.InitDB(cfg.Database.MySQL.DSN)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 2.5 运行数据库迁移
	log.Println("Running database migrations...")
	if err := migrations.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// 3. 初始化 Redis 连接
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Redis.Addr,
		Password: cfg.Database.Redis.Password,
		DB:       cfg.Database.Redis.DB,
	})
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis")

	// 4. 创建 gRPC 服务
	userHandler := handler.NewUserHandler(db, rdb)
	grpcSrv := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcSrv, userHandler)

	// 5. 启动 gRPC 监听
	lis, err := net.Listen("tcp", cfg.Server.UserGRPCPort)
	if err != nil {
		log.Fatalf("failed to listen on gRPC port %s: %v", cfg.Server.UserGRPCPort, err)
	}
	log.Printf("gRPC server is running on %s...", cfg.Server.UserGRPCPort)

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC: %v", err)
	}
}
