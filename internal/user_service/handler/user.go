package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	pb "ChatIM/api/proto/user"
	"ChatIM/internal/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/metadata"
)

type UserHandler struct {
	pb.UnimplementedUserServiceServer
	db    *sql.DB
	redis *redis.Client
}

func NewUserHandler(db *sql.DB, redis *redis.Client) *UserHandler {
	return &UserHandler{
		db:    db,
		redis: redis,
	}
}

func (h *UserHandler) GetUserByID(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	log.Printf("Received request for user ID: %s", req.Id)

	var username, nickname string
	err := h.db.QueryRowContext(ctx, "SELECT username, nickname FROM users WHERE id = ?", req.Id).Scan(&username, &nickname)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &pb.GetUserResponse{
		Id:       req.Id,
		Username: username,
		Nickname: nickname,
	}, nil
}

func (h *UserHandler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	log.Printf("Received request to create user with username: %s", req.Username)

	// 1. 检查用户名是否已存在
	var existingID string
	err := h.db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", req.Username).Scan(&existingID)
	if err == nil {
		log.Printf("Username %s already exists", req.Username)
		return &pb.CreateUserResponse{
			Code:    -1,
			Message: "用户名已存在",
		}, nil
	}
	if err != sql.ErrNoRows {
		log.Printf("Database error while checking username: %v", err)
		return nil, err
	}

	// 2. 对密码进行哈希处理
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		return nil, err
	}

	// 3. 插入新用户到数据库 (这次我们存哈希后的密码)
	newUserID := uuid.New().String()
	_, err = h.db.ExecContext(ctx, "INSERT INTO users (id, username, nickname, password_hash) VALUES (?, ?, ?, ?)",
		newUserID, req.Username, req.Nickname, string(hashedPassword))
	if err != nil {
		log.Printf("Failed to insert new user: %v", err)
		return nil, err
	}

	log.Printf("Successfully created user %s with ID: %s", req.Username, newUserID)

	return &pb.CreateUserResponse{
		Code:    0,
		Message: "注册成功",
		UserId:  newUserID,
	}, nil
}

// Login 用户登录
func (h *UserHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	log.Printf("Received login request for username: %s", req.Username)

	// 1. 从数据库查询用户
	var userID, hashedPassword string
	err := h.db.QueryRowContext(ctx, "SELECT id, password_hash FROM users WHERE username = ?", req.Username).Scan(&userID, &hashedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			return &pb.LoginResponse{Code: -1, Message: "用户名或密码错误"}, nil
		}
		return nil, err
	}

	// 2. 比较密码
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	if err != nil {
		// 密码不匹配
		return &pb.LoginResponse{Code: -1, Message: "用户名或密码错误"}, nil
	}

	// 3. 密码正确，生成真实的 JWT
	tokenString, err := utils.GenerateToken(userID) // 👈 调用工具包生成 Token
	if err != nil {
		log.Printf("Failed to generate token for user %s: %v", req.Username, err)
		return nil, fmt.Errorf("failed to generate token")
	}
	err = h.redis.Set(ctx, "online_status:"+userID, "1", 24*time.Hour).Err()
	if err != nil {
		// Redis 写入失败不应该影响登录，但应该记录日志
		log.Printf("Warning: failed to set user online status in Redis for user %s: %v", userID, err)
	}
	log.Printf("User %s logged in successfully", req.Username)

	return &pb.LoginResponse{
		Code:    0,
		Message: "登录成功",
		Token:   tokenString, // 👈 返回真实的 Token
	}, nil
}
func (h *UserHandler) GetCurrentUser(ctx context.Context, req *pb.GetCurrentUserRequest) (*pb.GetCurrentUserResponse, error) {
	// 👇 核心改动：从 context 中获取 Metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &pb.GetCurrentUserResponse{
			Code:    -1,
			Message: "用户未认证",
		}, nil
	}

	// 👇 从 Metadata 中获取 user-id 的值
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return &pb.GetCurrentUserResponse{
			Code:    -1,
			Message: "用户未认证",
		}, nil
	}
	userID := userIDs[0] // 取第一个值

	// 现在我们有了 userID，可以继续后续的逻辑了
	log.Printf("Received request to get current user info for ID: %s", userID)

	// ... (后续的数据库查询逻辑保持不变) ...
	var username, nickname string
	err := h.db.QueryRowContext(ctx, "SELECT username, nickname FROM users WHERE id = ?", userID).Scan(&username, &nickname)
	if err != nil {
		if err == sql.ErrNoRows {
			return &pb.GetCurrentUserResponse{
				Code:    -1,
				Message: "用户不存在",
			}, nil
		}
		return nil, err
	}

	return &pb.GetCurrentUserResponse{
		Code:     0,
		Message:  "获取成功",
		UserId:   userID,
		Username: username,
		Nickname: nickname,
	}, nil
}
func (h *UserHandler) CheckUserOnline(ctx context.Context, req *pb.CheckUserOnlineRequest) (*pb.CheckUserOnlineResponse, error) {
	log.Printf("Received request to check online status for user_id: %s", req.UserId)

	// 👇 核心逻辑：从 Redis 中查询 key 是否存在
	key := "online_status:" + req.UserId
	result, err := h.redis.Exists(ctx, key).Result()
	if err != nil {
		// Redis 查询出错
		log.Printf("Error checking user online status in Redis: %v", err)
		return &pb.CheckUserOnlineResponse{
			Code:     -1,
			Message:  "服务内部错误",
			IsOnline: false,
		}, nil
	}

	// Redis 的 Exists 命令：如果 key 存在，返回 1；否则返回 0
	isOnline := result == 1

	log.Printf("User %s is online: %t", req.UserId, isOnline)

	return &pb.CheckUserOnlineResponse{
		Code:     0,
		Message:  "查询成功",
		IsOnline: isOnline,
	}, nil
}
