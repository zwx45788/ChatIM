package main

import (
	"log"
	"net"

	pb "ChatIM/api/proto/friendship"
	"ChatIM/internal/friendship/handler"
	"ChatIM/internal/friendship/repository"
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"ChatIM/pkg/migrations"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
		log.Fatalf("Failed to listen on gRPC port %s: %v", port, err)
	}
	log.Printf("Friendship gRPC server is running on %s...", port)

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC: %v", err)
	}
}
