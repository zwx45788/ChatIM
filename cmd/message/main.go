package main

import (
	"ChatIM/pkg/database"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "ChatIM/api/proto/message"
	"ChatIM/internal/message_service/handler"
)

func main() {
	// 1. 初始化数据源

	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	// 2. 创建 gRPC 服务器
	grpcSrv := grpc.NewServer()

	lis, err := net.Listen("tcp", ":50052") // 👈 使用新端口 50052
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port 50052: %v", err)
	}
	log.Println("gRPC server is running on :50052...")

	// 3. 注册服务
	pb.RegisterMessageServiceServer(grpcSrv, handler.NewMessageHandler(db))

	log.Println("Message service is running on :50052...")
	reflection.Register(grpcSrv)

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC: %v", err)
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
