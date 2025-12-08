package main

import (
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"log"
	"net"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "ChatIM/api/proto/group"
	"ChatIM/internal/group_service/handler"
)

func main() {
	// 1. 初始化数据源
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	db, err := database.InitDB(cfg.Database.MySQL.DSN)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 2. 创建Redis客户端
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Redis.Addr,
		Password: cfg.Database.Redis.Password,
		DB:       cfg.Database.Redis.DB,
	})

	// 3. 创建gRPC服务器
	grpcSrv := grpc.NewServer()

	lis, err := net.Listen("tcp", cfg.Server.GroupGRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %v: %v", cfg.Server.GroupGRPCPort, err)
	}
	log.Printf("gRPC server is running on %v...", cfg.Server.GroupGRPCPort)

	// 4. 注册GroupService
	pb.RegisterGroupServiceServer(grpcSrv, handler.NewGroupHandler(db, rdb))
	reflection.Register(grpcSrv)

	log.Printf("Group service is running on %v...", cfg.Server.GroupGRPCPort)

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC: %v", err)
	}
}
