package model

import (
	"database/sql"
	"time"
)

// FriendRequest 好友请求模型
type FriendRequest struct {
	ID          string     `db:"id"`
	FromUserID  string     `db:"from_user_id"`
	ToUserID    string     `db:"to_user_id"`
	Message     string     `db:"message"`
	Status      string     `db:"status"` // pending, accepted, rejected, cancelled
	CreatedAt   time.Time  `db:"created_at"`
	ProcessedAt *time.Time `db:"processed_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// Friend 好友模型
type Friend struct {
	UserID1   string    `db:"user_id_1"`
	UserID2   string    `db:"user_id_2"`
	CreatedAt time.Time `db:"created_at"`
}

// GroupJoinRequest 群加入请求模型
type GroupJoinRequest struct {
	ID          string     `db:"id"`
	GroupID     string     `db:"group_id"`
	FromUserID  string     `db:"from_user_id"`
	Message     string     `db:"message"`
	Status      string     `db:"status"` // pending, accepted, rejected, cancelled
	ReviewedBy  *string    `db:"reviewed_by"`
	CreatedAt   time.Time  `db:"created_at"`
	ProcessedAt *time.Time `db:"processed_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// FriendRequestWithUserInfo 包含用户信息的好友请求
type FriendRequestWithUserInfo struct {
	ID           string
	FromUserID   string
	FromUsername string
	FromNickname string
	Message      string
	Status       string
	CreatedAt    time.Time
}

// GroupJoinRequestWithUserInfo 包含用户信息的群申请
type GroupJoinRequestWithUserInfo struct {
	ID           string
	GroupID      string
	FromUserID   string
	FromUsername string
	FromNickname string
	Message      string
	Status       string
	CreatedAt    time.Time
}

// StatusEnum 状态枚举
const (
	RequestStatusPending   = "pending"
	RequestStatusAccepted  = "accepted"
	RequestStatusRejected  = "rejected"
	RequestStatusCancelled = "cancelled"
)

// StatusToInt 将状态转换为整数
func StatusToInt(status string) int32 {
	switch status {
	case RequestStatusPending:
		return 0
	case RequestStatusAccepted:
		return 1
	case RequestStatusRejected:
		return 2
	case RequestStatusCancelled:
		return 3
	default:
		return -1
	}
}

// IntToStatus 将整数转换为状态
func IntToStatus(code int32) string {
	switch code {
	case 0:
		return RequestStatusPending
	case 1:
		return RequestStatusAccepted
	case 2:
		return RequestStatusRejected
	case 3:
		return RequestStatusCancelled
	default:
		return RequestStatusPending
	}
}

// NullStringPtr 将sql.NullString转换为*string
func NullStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
}
