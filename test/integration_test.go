package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "ChatIM/api/proto/user"
	"ChatIM/pkg/config"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestUserServiceIntegration 用户服务集成测试
func TestUserServiceIntegration(t *testing.T) {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 连接到 user-service
	conn, err := grpc.Dial("localhost"+cfg.Server.UserGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to user service: %v", err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 生成测试用户名
	timestamp := time.Now().Unix()
	username := fmt.Sprintf("test_integration_%d", timestamp)
	password := "Test@123456"
	var userID string
	var token string

	// 测试注册（使用CreateUser）
	t.Run("CreateUser", func(t *testing.T) {
		req := &pb.CreateUserRequest{
			Username: username,
			Password: password,
			Nickname: username,
		}

		resp, err := client.CreateUser(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), resp.Code)
		assert.NotEmpty(t, resp.UserId)
		userID = resp.UserId
		t.Logf("User created with ID: %s", resp.UserId)
	})

	// 测试登录
	t.Run("Login", func(t *testing.T) {
		req := &pb.LoginRequest{
			Username: username,
			Password: password,
		}

		resp, err := client.Login(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), resp.Code)
		assert.NotEmpty(t, resp.Token)
		token = resp.Token
		t.Logf("Login successful, token: %s", token)
	})

	// 测试获取用户信息
	t.Run("GetUserByID", func(t *testing.T) {
		req := &pb.GetUserRequest{
			Id: userID,
		}

		resp, err := client.GetUserByID(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, username, resp.Username)
		t.Logf("User info: %s", resp.Username)
	})

	// 测试搜索用户
	t.Run("SearchUsers", func(t *testing.T) {
		req := &pb.SearchUsersRequest{
			Keyword: "test_integration",
			Limit:   10,
			Offset:  0,
		}

		resp, err := client.SearchUsers(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), resp.Code)
		assert.GreaterOrEqual(t, len(resp.Users), 0)
		t.Logf("Found %d users", len(resp.Users))
	})
}

// TestConcurrentRequests 并发请求测试
func TestConcurrentRequests(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	conn, err := grpc.Dial("localhost"+cfg.Server.UserGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to user service: %v", err)
	}
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)

	// 并发测试
	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			timestamp := time.Now().UnixNano()
			username := fmt.Sprintf("concurrent_test_%d_%d", timestamp, id)

			req := &pb.CreateUserRequest{
				Username: username,
				Password: "Test@123456",
				Nickname: username,
			}

			resp, err := client.CreateUser(ctx, req)
			if err != nil {
				t.Errorf("Concurrent request %d failed: %v", id, err)
			} else if resp.Code != 0 {
				t.Errorf("Concurrent request %d returned code %d", id, resp.Code)
			}

			done <- true
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	t.Log("All concurrent requests completed")
}

// BenchmarkUserRegistration 注册性能基准测试
func BenchmarkUserRegistration(b *testing.B) {
	cfg, _ := config.LoadConfig()
	conn, _ := grpc.Dial("localhost"+cfg.Server.UserGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()

	client := pb.NewUserServiceClient(conn)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		username := fmt.Sprintf("bench_user_%d", time.Now().UnixNano())

		req := &pb.CreateUserRequest{
			Username: username,
			Password: "Test@123456",
			Nickname: username,
		}

		_, err := client.CreateUser(ctx, req)
		if err != nil {
			b.Errorf("Benchmark failed: %v", err)
		}
		cancel()
	}
}
