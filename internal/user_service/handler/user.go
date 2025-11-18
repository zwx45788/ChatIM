package handler

import (
	pb "ChatIM/api/proto/user"
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/google/uuid"
)

type UserHandler struct {
	pb.UnimplementedUserServiceServer
	db *sql.DB
}

func NewUserHandler(database *sql.DB) *UserHandler {
	return &UserHandler{
		db: database,
	}
}

// GetUserByID 获取用户信息
func (h *UserHandler) GetUserByID(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	log.Printf("Received request for user ID: %s", req.Id)

	var username, nickname string
	err := h.db.QueryRowContext(ctx, "SELECT username, nickname FROM users WHERE id = ?", req.Id).Scan(&username, &nickname)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("User not found: %s", req.Id)
			return nil, fmt.Errorf("user not found")
		}
		log.Printf("Database error: %v", err)
		return nil, err
	}

	return &pb.GetUserResponse{
		Id:       req.Id,
		Username: username,
		Nickname: nickname,
	}, nil
}

// CreateUser 创建新用户
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

	// 2. 生成一个唯一的用户 ID
	newUserID := uuid.New().String()

	// 3. 插入新用户到数据库 (注意：这里先存明文密码，仅用于测试！)
	_, err = h.db.ExecContext(ctx, "INSERT INTO users (id, username, nickname, password_hash) VALUES (?, ?, ?, ?)",
		newUserID, req.Username, req.Nickname, req.Password)
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
