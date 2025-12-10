package main

import (
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"log"
	"net"

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

	// 2. 创建gRPC服务器
	grpcSrv := grpc.NewServer()

	lis, err := net.Listen("tcp", cfg.Server.GroupGRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port %v: %v", cfg.Server.GroupGRPCPort, err)
	}
	log.Printf("gRPC server is running on %v...", cfg.Server.GroupGRPCPort)

	// 3. 注册GroupService
	pb.RegisterGroupServiceServer(grpcSrv, handler.NewGroupHandler(db))
	reflection.Register(grpcSrv)

	log.Printf("Group service is running on %v...", cfg.Server.GroupGRPCPort)

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC: %v", err)
	}
}
