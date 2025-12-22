package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ChatIM/internal/friendship/model"
	"ChatIM/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FriendshipRepository 好友相关的数据库操作
type FriendshipRepository struct {
	db *sql.DB
}

// NewFriendshipRepository 创建好友数据库操作实例
func NewFriendshipRepository(db *sql.DB) *FriendshipRepository {
	return &FriendshipRepository{db: db}
}

// ==================== 好友请求相关操作 ====================

// GetFriendRequestByUsers 根据 from/to 获取请求（用于处理 unique(from,to)）
func (r *FriendshipRepository) GetFriendRequestByUsers(ctx context.Context, fromUserID, toUserID string) (id string, status string, err error) {
	query := `SELECT id, status FROM friend_requests WHERE from_user_id = ? AND to_user_id = ? LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, fromUserID, toUserID)
	if err := row.Scan(&id, &status); err != nil {
		return "", "", err
	}
	return id, status, nil
}

// ResetFriendRequestToPending 将历史请求重置为 pending（允许再次发送）
func (r *FriendshipRepository) ResetFriendRequestToPending(ctx context.Context, requestID string, message string) error {
	query := `UPDATE friend_requests
	          SET message = ?, status = ?, processed_at = NULL, updated_at = ?, created_at = ?
	          WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, message, model.RequestStatusPending, time.Now(), time.Now(), requestID)
	if err != nil {
		logger.Error("Error resetting friend request", zap.Error(err), zap.String("request_id", requestID))
		return err
	}
	return nil
}

// UserExists 检查用户是否存在（避免外键错误被包装成 Internal）
func (r *FriendshipRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	query := `SELECT COUNT(*) FROM users WHERE id = ?`
	var count int
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		logger.Error("Error checking user exists", zap.Error(err), zap.String("user_id", userID))
		return false, err
	}
	return count > 0, nil
}

// SendFriendRequest 发送好友请求
func (r *FriendshipRepository) SendFriendRequest(ctx context.Context, fromUserID, toUserID, message string) (string, error) {
	// 先查是否存在历史记录（表上通常有 unique(from_user_id,to_user_id)）
	if existingID, existingStatus, err := r.GetFriendRequestByUsers(ctx, fromUserID, toUserID); err == nil {
		// pending 理论上应当被上层拦截，但这里兜底
		if existingStatus == model.RequestStatusPending {
			return existingID, nil
		}
		// 历史请求已处理/取消：重置为 pending，实现“可再次发送”
		if err := r.ResetFriendRequestToPending(ctx, existingID, message); err != nil {
			logger.Error("Error resending friend request", zap.Error(err), zap.String("from_user_id", fromUserID), zap.String("to_user_id", toUserID))
			return "", err
		}
		return existingID, nil
	}

	requestID := uuid.New().String()
	query := `INSERT INTO friend_requests (id, from_user_id, to_user_id, message, status, created_at)
	          VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, requestID, fromUserID, toUserID, message, model.RequestStatusPending, time.Now())
	if err != nil {
		logger.Error("Error sending friend request", zap.Error(err), zap.String("from_user_id", fromUserID), zap.String("to_user_id", toUserID))
		return "", err
	}
	return requestID, nil
}

// GetFriendRequest 获取单个好友请求
func (r *FriendshipRepository) GetFriendRequest(ctx context.Context, requestID string) (*model.FriendRequest, error) {
	query := `SELECT id, from_user_id, to_user_id, message, status, created_at, processed_at, updated_at
	          FROM friend_requests WHERE id = ?`

	var req model.FriendRequest
	var processedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, requestID).Scan(
		&req.ID,
		&req.FromUserID,
		&req.ToUserID,
		&req.Message,
		&req.Status,
		&req.CreatedAt,
		&processedAt,
		&req.UpdatedAt,
	)

	if processedAt.Valid {
		req.ProcessedAt = &processedAt.Time
	}

	return &req, err
}

// GetFriendRequests 获取好友请求列表（分状态）
func (r *FriendshipRepository) GetFriendRequests(ctx context.Context, toUserID string, status string, limit, offset int64) ([]*model.FriendRequestWithUserInfo, error) {
	var query string
	var args []interface{}

	baseQuery := `SELECT fr.id, fr.from_user_id, u.username, u.nickname, fr.message, fr.status, fr.created_at
	              FROM friend_requests fr
	              JOIN users u ON fr.from_user_id = u.id
	              WHERE fr.to_user_id = ?`

	if status != "" && status != "all" {
		baseQuery += ` AND fr.status = ?`
		args = append(args, toUserID, status)
	} else {
		args = append(args, toUserID)
	}

	baseQuery += ` ORDER BY fr.created_at DESC LIMIT ? OFFSET ?`
	query = baseQuery
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		logger.Error("Error querying friend requests", zap.Error(err), zap.String("to_user_id", toUserID))
		return nil, err
	}
	defer rows.Close()

	var requests []*model.FriendRequestWithUserInfo
	for rows.Next() {
		var req model.FriendRequestWithUserInfo
		err := rows.Scan(
			&req.ID,
			&req.FromUserID,
			&req.FromUsername,
			&req.FromNickname,
			&req.Message,
			&req.Status,
			&req.CreatedAt,
		)
		if err != nil {
			logger.Warn("Error scanning friend request", zap.Error(err))
			continue
		}
		requests = append(requests, &req)
	}

	return requests, nil
}

// CountFriendRequests 统计好友请求数
func (r *FriendshipRepository) CountFriendRequests(ctx context.Context, toUserID string, status string) (int32, error) {
	var query string
	var args []interface{}

	baseQuery := `SELECT COUNT(*) FROM friend_requests WHERE to_user_id = ?`
	args = append(args, toUserID)

	if status != "" && status != "all" {
		baseQuery += ` AND status = ?`
		args = append(args, status)
	}

	query = baseQuery
	var count int32
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		logger.Error("Error counting friend requests", zap.Error(err), zap.String("to_user_id", toUserID))
		return 0, err
	}

	return count, nil
}

// AcceptFriendRequest 接受好友请求
func (r *FriendshipRepository) AcceptFriendRequest(ctx context.Context, requestID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("Error starting transaction", zap.Error(err), zap.String("request_id", requestID))
		return err
	}
	defer tx.Rollback()

	// 1. 获取请求信息
	query := `SELECT from_user_id, to_user_id FROM friend_requests WHERE id = ?`
	var fromUserID, toUserID string
	err = tx.QueryRowContext(ctx, query, requestID).Scan(&fromUserID, &toUserID)
	if err != nil {
		logger.Error("Error getting friend request", zap.Error(err), zap.String("request_id", requestID))
		return err
	}

	// 2. 更新请求状态为 accepted
	updateQuery := `UPDATE friend_requests SET status = ?, processed_at = ?, updated_at = ? WHERE id = ?`
	_, err = tx.ExecContext(ctx, updateQuery, model.RequestStatusAccepted, time.Now(), time.Now(), requestID)
	if err != nil {
		logger.Error("Error updating friend request status", zap.Error(err), zap.String("request_id", requestID))
		return err
	}

	// 3. 添加到 friends 表（确保 user_id_1 < user_id_2）
	var user1, user2 string
	if fromUserID < toUserID {
		user1, user2 = fromUserID, toUserID
	} else {
		user1, user2 = toUserID, fromUserID
	}

	addFriendQuery := `INSERT INTO friends (user_id_1, user_id_2, created_at) VALUES (?, ?, ?)
	                   ON DUPLICATE KEY UPDATE created_at = created_at`
	_, err = tx.ExecContext(ctx, addFriendQuery, user1, user2, time.Now())
	if err != nil {
		logger.Error("Error adding friend", zap.Error(err), zap.String("user1", user1), zap.String("user2", user2))
		return err
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		logger.Error("Error committing transaction", zap.Error(err), zap.String("request_id", requestID))
		return err
	}

	logger.Info("Friend request accepted", zap.String("request_id", requestID), zap.String("from_user_id", fromUserID), zap.String("to_user_id", toUserID))
	return nil
}

// RejectFriendRequest 拒绝好友请求
func (r *FriendshipRepository) RejectFriendRequest(ctx context.Context, requestID string) error {
	query := `UPDATE friend_requests SET status = ?, processed_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, model.RequestStatusRejected, time.Now(), time.Now(), requestID)
	if err != nil {
		logger.Error("Error rejecting friend request", zap.Error(err), zap.String("request_id", requestID))
		return err
	}

	logger.Info("Friend request rejected", zap.String("request_id", requestID))
	return nil
}

// CheckFriendshipExists 检查两个用户是否是好友
func (r *FriendshipRepository) CheckFriendshipExists(ctx context.Context, userID1, userID2 string) (bool, error) {
	var user1, user2 string
	if userID1 < userID2 {
		user1, user2 = userID1, userID2
	} else {
		user1, user2 = userID2, userID1
	}

	query := `SELECT COUNT(*) FROM friends WHERE user_id_1 = ? AND user_id_2 = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, user1, user2).Scan(&count)
	if err != nil {
		logger.Error("Error checking friendship", zap.Error(err), zap.String("user1", user1), zap.String("user2", user2))
		return false, err
	}

	return count > 0, nil
}

// CheckPendingFriendRequest 检查是否存在待处理的好友请求
func (r *FriendshipRepository) CheckPendingFriendRequest(ctx context.Context, fromUserID, toUserID string) (bool, error) {
	query := `SELECT COUNT(*) FROM friend_requests 
	          WHERE from_user_id = ? AND to_user_id = ? AND status = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, fromUserID, toUserID, model.RequestStatusPending).Scan(&count)
	if err != nil {
		logger.Error("Error checking pending friend request", zap.Error(err), zap.String("from_user_id", fromUserID), zap.String("to_user_id", toUserID))
		return false, err
	}

	return count > 0, nil
}

// GetFriends 获取用户的好友列表
func (r *FriendshipRepository) GetFriends(ctx context.Context, userID string, limit, offset int64) ([]map[string]interface{}, error) {
	query := `SELECT CASE 
	           WHEN user_id_1 = ? THEN user_id_2
	           ELSE user_id_1
	           END as friend_id,
	           u.username, u.nickname, f.created_at
	          FROM friends f
	          JOIN users u ON (
	            (f.user_id_1 = ? AND u.id = f.user_id_2) OR
	            (f.user_id_2 = ? AND u.id = f.user_id_1)
	          )
	          ORDER BY f.created_at DESC
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, userID, userID, userID, limit, offset)
	if err != nil {
		logger.Error("Error querying friends", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}
	defer rows.Close()

	var friends []map[string]interface{}
	for rows.Next() {
		var friendID, username, nickname string
		var createdAt time.Time
		err := rows.Scan(&friendID, &username, &nickname, &createdAt)
		if err != nil {
			logger.Warn("Error scanning friend", zap.Error(err))
			continue
		}
		friends = append(friends, map[string]interface{}{
			"user_id":    friendID,
			"username":   username,
			"nickname":   nickname,
			"created_at": createdAt.Unix(),
		})
	}

	return friends, nil
}

// CountFriends 统计好友数量
func (r *FriendshipRepository) CountFriends(ctx context.Context, userID string) (int32, error) {
	query := `SELECT COUNT(*) FROM friends 
	          WHERE user_id_1 = ? OR user_id_2 = ?`
	var count int32
	err := r.db.QueryRowContext(ctx, query, userID, userID).Scan(&count)
	if err != nil {
		logger.Error("Error counting friends", zap.Error(err), zap.String("user_id", userID))
		return 0, err
	}

	return count, nil
}

// RemoveFriend 删除好友关系
func (r *FriendshipRepository) RemoveFriend(ctx context.Context, userID1, userID2 string) error {
	var user1, user2 string
	if userID1 < userID2 {
		user1, user2 = userID1, userID2
	} else {
		user1, user2 = userID2, userID1
	}

	query := `DELETE FROM friends WHERE user_id_1 = ? AND user_id_2 = ?`
	result, err := r.db.ExecContext(ctx, query, user1, user2)
	if err != nil {
		logger.Error("Error removing friend", zap.Error(err), zap.String("user1", user1), zap.String("user2", user2))
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("Error getting rows affected", zap.Error(err))
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("friendship not found")
	}

	logger.Info("Friend relationship removed", zap.String("user1", user1), zap.String("user2", user2))
	return nil
}

// ==================== 用户群组相关操作 ====================

// GetUserGroups 获取用户所在的所有群组
func (r *FriendshipRepository) GetUserGroups(ctx context.Context, userID string, limit, offset int64) ([]map[string]interface{}, error) {
	query := `
		SELECT g.id, g.name, g.description, COUNT(gm.user_id) as member_count, g.created_at
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = ?
		GROUP BY g.id, g.name, g.description, g.created_at
		ORDER BY g.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		logger.Error("Error querying user groups", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}
	defer rows.Close()

	var groups []map[string]interface{}
	for rows.Next() {
		var id, name, description string
		var memberCount int32
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &description, &memberCount, &createdAt); err != nil {
			logger.Error("Error scanning group row", zap.Error(err))
			return nil, err
		}

		groups = append(groups, map[string]interface{}{
			"group_id":     id,
			"group_name":   name,
			"description":  description,
			"member_count": memberCount,
			"created_at":   createdAt.Unix(),
		})
	}

	if err := rows.Err(); err != nil {
		logger.Error("Error iterating groups", zap.Error(err))
		return nil, err
	}

	return groups, nil
}

// CountUserGroups 获取用户所在群组的总数
func (r *FriendshipRepository) CountUserGroups(ctx context.Context, userID string) (int32, error) {
	query := `
		SELECT COUNT(DISTINCT g.id)
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = ?
	`

	var count int32
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		logger.Error("Error counting user groups", zap.Error(err), zap.String("user_id", userID))
		return 0, err
	}

	return count, nil
}

// ==================== 群组成员管理相关操作 ====================

// LeaveGroup 用户退出群组
func (r *FriendshipRepository) LeaveGroup(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`
	result, err := r.db.ExecContext(ctx, query, groupID, userID)
	if err != nil {
		logger.Error("Error leaving group", zap.Error(err), zap.String("group_id", groupID), zap.String("user_id", userID))
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("Error getting rows affected", zap.Error(err))
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("用户不在该群组中")
	}

	logger.Info("User left group", zap.String("user_id", userID), zap.String("group_id", groupID))
	return nil
}

// RemoveGroupMember 管理员踢出群成员
func (r *FriendshipRepository) RemoveGroupMember(ctx context.Context, groupID, memberUserID string) error {
	query := `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`
	result, err := r.db.ExecContext(ctx, query, groupID, memberUserID)
	if err != nil {
		logger.Error("Error removing group member", zap.Error(err), zap.String("group_id", groupID), zap.String("member_id", memberUserID))
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("Error getting rows affected", zap.Error(err))
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("用户不在该群组中")
	}

	logger.Info("User removed from group", zap.String("member_id", memberUserID), zap.String("group_id", groupID))
	return nil
}

// CheckGroupMembership 检查用户是否在群组中
func (r *FriendshipRepository) CheckGroupMembership(ctx context.Context, groupID, userID string) (bool, error) {
	query := `SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, groupID, userID).Scan(&count)
	if err != nil {
		logger.Error("Error checking group membership", zap.Error(err), zap.String("group_id", groupID), zap.String("user_id", userID))
		return false, err
	}

	return count > 0, nil
}

// CheckGroupOwner 检查用户是否是群主
func (r *FriendshipRepository) CheckGroupOwner(ctx context.Context, groupID, userID string) (bool, error) {
	query := `SELECT owner_id FROM groups WHERE id = ?`
	var ownerID string
	err := r.db.QueryRowContext(ctx, query, groupID).Scan(&ownerID)
	if err != nil {
		logger.Error("Error checking group owner", zap.Error(err), zap.String("group_id", groupID), zap.String("user_id", userID))
		return false, err
	}

	return ownerID == userID, nil
}
