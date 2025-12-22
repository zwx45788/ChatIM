package main

import (
	"ChatIM/pkg/config"
	"ChatIM/pkg/database"
	"ChatIM/pkg/logger"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "ChatIM/api/proto/group"
	"ChatIM/internal/group_service/handler"
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

	logger.Info("=== Group Service starting ===")

	db, err := database.InitDB(cfg.Database.MySQL.DSN)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// 2. 创建gRPC服务器
	grpcSrv := grpc.NewServer()

	lis, err := net.Listen("tcp", cfg.Server.GroupGRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port",
			zap.String("port", cfg.Server.GroupGRPCPort),
			zap.Error(err))
	}

	// 3. 注册GroupService
	pb.RegisterGroupServiceServer(grpcSrv, handler.NewGroupHandler(db))
	reflection.Register(grpcSrv)

	logger.Info("🚀 Group Service gRPC server started",
		zap.String("port", cfg.Server.GroupGRPCPort))

	if err := grpcSrv.Serve(lis); err != nil {
		logger.Fatal("Failed to serve gRPC", zap.Error(err))
	}
}
