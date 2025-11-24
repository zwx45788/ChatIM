// 在 ChatIM/internal/user_service/main.go

package main

import (
	pb "ChatIM/api/proto/user"
	"ChatIM/internal/user_service/handler" // 导入 database/sql
	"context"
	"log"
	"net"

	"ChatIM/pkg/database"

	_ "github.com/go-sql-driver/mysql" // 匿名导入 MySQL 驱动
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	// 1. 【新增】初始化数据库连接
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	// 确保在程序退出时关闭数据库连接
	defer db.Close()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // 因为我们用 Docker 启动了 Redis
	})
	// 验证 Redis 连接
	ctx := context.Background()
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis")
	// 2. 【修改】将数据库连接传递给 handler
	userHandler := handler.NewUserHandler(db, rdb)

	// 3. 启动 gRPC 服务
	grpcSrv := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcSrv, userHandler)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen on gRPC port 50051: %v", err)
	}
	log.Println("gRPC server is running on :50051...")

	if err := grpcSrv.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC: %v", err)
	}

	// // 4. (可选) 启动 HTTP 服务
	// r := gin.Default()
	// // ... 可以添加一些 HTTP 路由 ...

	// stop := func() {
	// 	log.Println("Shutting down gRPC server...")
	// 	grpcSrv.GracefulStop()
	// }

	// pkg.Run(r, "User Service HTTP", "127.0.0.1:8080", stop)
}

// 【新增】将 InitDB 函数放在这里，或者放在一个 db/db.go 文件里
