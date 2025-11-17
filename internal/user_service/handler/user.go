package handler

import (
	pb "ChatIM/api/proto/user"
	"context"
	"database/sql"
	"fmt"
	"log"
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
func (h *UserHandler) GetUserByID(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	log.Printf("Received request for user ID: %s", req.Id)

	// ✅ 现在，我们可以用 h.db 来查询数据库了！
	var username, nickname string
	// 假设你的用户表叫 users，有 id, username, nickname 这几列
	err := h.db.QueryRowContext(ctx, "SELECT username, nickname FROM users WHERE id = ?", req.Id).Scan(&username, &nickname)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("User not found: %s", req.Id)
			return nil, fmt.Errorf("user not found") // 返回一个明确的错误
		}
		log.Printf("Database error: %v", err)
		return nil, err // 返回数据库错误
	}

	// 返回从数据库查到的真实数据
	return &pb.GetUserResponse{
		Id:       req.Id,
		Username: username,
		Nickname: nickname,
	}, nil
}
