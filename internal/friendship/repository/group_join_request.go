package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"ChatIM/internal/friendship/model"
	"github.com/google/uuid"
)

// ==================== 群申请相关操作 ====================

// SendGroupJoinRequest 发送群申请
func (r *FriendshipRepository) SendGroupJoinRequest(ctx context.Context, groupID, fromUserID, message string) (string, error) {
	requestID := uuid.New().String()

	query := `INSERT INTO group_join_requests (id, group_id, from_user_id, message, status, created_at)
	          VALUES (?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query, requestID, groupID, fromUserID, message, model.RequestStatusPending, time.Now())
	if err != nil {
		log.Printf("Error sending group join request: %v", err)
		return "", err
	}

	return requestID, nil
}

// GetGroupJoinRequest 获取单个群申请
func (r *FriendshipRepository) GetGroupJoinRequest(ctx context.Context, requestID string) (*model.GroupJoinRequest, error) {
	query := `SELECT id, group_id, from_user_id, message, status, reviewed_by, created_at, processed_at, updated_at
	          FROM group_join_requests WHERE id = ?`

	var req model.GroupJoinRequest
	var processedAt sql.NullTime
	var reviewedBy sql.NullString

	err := r.db.QueryRowContext(ctx, query, requestID).Scan(
		&req.ID,
		&req.GroupID,
		&req.FromUserID,
		&req.Message,
		&req.Status,
		&reviewedBy,
		&req.CreatedAt,
		&processedAt,
		&req.UpdatedAt,
	)

	if processedAt.Valid {
		req.ProcessedAt = &processedAt.Time
	}
	if reviewedBy.Valid {
		req.ReviewedBy = &reviewedBy.String
	}

	return &req, err
}

// GetGroupJoinRequests 获取群申请列表
func (r *FriendshipRepository) GetGroupJoinRequests(ctx context.Context, groupID string, status string, limit, offset int64) ([]*model.GroupJoinRequestWithUserInfo, error) {
	var query string
	var args []interface{}

	baseQuery := `SELECT gjr.id, gjr.group_id, gjr.from_user_id, u.username, u.nickname, gjr.message, gjr.status, gjr.created_at
	              FROM group_join_requests gjr
	              JOIN users u ON gjr.from_user_id = u.id
	              WHERE gjr.group_id = ?`

	if status != "" && status != "all" {
		baseQuery += ` AND gjr.status = ?`
		args = append(args, groupID, status)
	} else {
		args = append(args, groupID)
	}

	baseQuery += ` ORDER BY gjr.created_at DESC LIMIT ? OFFSET ?`
	query = baseQuery
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying group join requests: %v", err)
		return nil, err
	}
	defer rows.Close()

	var requests []*model.GroupJoinRequestWithUserInfo
	for rows.Next() {
		var req model.GroupJoinRequestWithUserInfo
		err := rows.Scan(
			&req.ID,
			&req.GroupID,
			&req.FromUserID,
			&req.FromUsername,
			&req.FromNickname,
			&req.Message,
			&req.Status,
			&req.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning group join request: %v", err)
			continue
		}
		requests = append(requests, &req)
	}

	return requests, nil
}

// CountGroupJoinRequests 统计群申请数
func (r *FriendshipRepository) CountGroupJoinRequests(ctx context.Context, groupID string, status string) (int32, error) {
	var query string
	var args []interface{}

	baseQuery := `SELECT COUNT(*) FROM group_join_requests WHERE group_id = ?`
	args = append(args, groupID)

	if status != "" && status != "all" {
		baseQuery += ` AND status = ?`
		args = append(args, status)
	}

	query = baseQuery
	var count int32
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		log.Printf("Error counting group join requests: %v", err)
		return 0, err
	}

	return count, nil
}

// AcceptGroupJoinRequest 接受群申请
func (r *FriendshipRepository) AcceptGroupJoinRequest(ctx context.Context, requestID string, reviewerID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return err
	}
	defer tx.Rollback()

	// 1. 获取请求信息
	query := `SELECT group_id, from_user_id FROM group_join_requests WHERE id = ?`
	var groupID, fromUserID string
	err = tx.QueryRowContext(ctx, query, requestID).Scan(&groupID, &fromUserID)
	if err != nil {
		log.Printf("Error getting group join request: %v", err)
		return err
	}

	// 2. 更新请求状态为 accepted
	updateQuery := `UPDATE group_join_requests 
	                SET status = ?, reviewed_by = ?, processed_at = ?, updated_at = ? 
	                WHERE id = ?`
	_, err = tx.ExecContext(ctx, updateQuery, model.RequestStatusAccepted, reviewerID, time.Now(), time.Now(), requestID)
	if err != nil {
		log.Printf("Error updating group join request status: %v", err)
		return err
	}

	// 3. 添加用户到群成员表
	addMemberQuery := `INSERT INTO group_members (group_id, user_id, role, joined_at)
	                   VALUES (?, ?, ?, ?)
	                   ON DUPLICATE KEY UPDATE role = 'member', joined_at = joined_at`
	_, err = tx.ExecContext(ctx, addMemberQuery, groupID, fromUserID, "member", time.Now())
	if err != nil {
		log.Printf("Error adding group member: %v", err)
		return err
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return err
	}

	log.Printf("Group join request %s accepted: user %s joined group %s", requestID, fromUserID, groupID)
	return nil
}

// RejectGroupJoinRequest 拒绝群申请
func (r *FriendshipRepository) RejectGroupJoinRequest(ctx context.Context, requestID string, reviewerID string) error {
	query := `UPDATE group_join_requests 
	          SET status = ?, reviewed_by = ?, processed_at = ?, updated_at = ? 
	          WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, model.RequestStatusRejected, reviewerID, time.Now(), time.Now(), requestID)
	if err != nil {
		log.Printf("Error rejecting group join request: %v", err)
		return err
	}

	log.Printf("Group join request %s rejected", requestID)
	return nil
}

// CheckGroupMemberExists 检查用户是否是群成员
func (r *FriendshipRepository) CheckGroupMemberExists(ctx context.Context, groupID, userID string) (bool, error) {
	query := `SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, groupID, userID).Scan(&count)
	if err != nil {
		log.Printf("Error checking group member: %v", err)
		return false, err
	}

	return count > 0, nil
}

// CheckPendingGroupJoinRequest 检查是否存在待处理的群申请
func (r *FriendshipRepository) CheckPendingGroupJoinRequest(ctx context.Context, groupID, userID string) (bool, error) {
	query := `SELECT COUNT(*) FROM group_join_requests 
	          WHERE group_id = ? AND from_user_id = ? AND status = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, groupID, userID, model.RequestStatusPending).Scan(&count)
	if err != nil {
		log.Printf("Error checking pending group join request: %v", err)
		return false, err
	}

	return count > 0, nil
}

// CheckGroupAdmin 检查用户是否是群主或管理员
func (r *FriendshipRepository) CheckGroupAdmin(ctx context.Context, groupID, userID string) (bool, error) {
	query := `SELECT COUNT(*) FROM group_members 
	          WHERE group_id = ? AND user_id = ? AND role IN ('admin', 'creator')`
	var count int
	err := r.db.QueryRowContext(ctx, query, groupID, userID).Scan(&count)
	if err != nil {
		log.Printf("Error checking group admin: %v", err)
		return false, err
	}

	return count > 0, nil
}

// GetGroupCreator 获取群创建者ID
func (r *FriendshipRepository) GetGroupCreator(ctx context.Context, groupID string) (string, error) {
	query := `SELECT creator_id FROM groups WHERE id = ?`
	var creatorID string
	err := r.db.QueryRowContext(ctx, query, groupID).Scan(&creatorID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("group not found")
		}
		log.Printf("Error getting group creator: %v", err)
		return "", err
	}

	return creatorID, nil
}
