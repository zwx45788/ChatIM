package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// StreamOperator 处理 Stream 相关的操作
type StreamOperator struct {
	rdb *redis.Client
}

const emptyGroupSentinel = "__empty_group__"

// NewStreamOperator 创建 Stream 操作器
func NewStreamOperator(rdb *redis.Client) *StreamOperator {
	return &StreamOperator{
		rdb: rdb,
	}
}

// MessagePayload 消息负载结构
type MessagePayload struct {
	ID         string `json:"id"`
	FromUserID string `json:"from_user_id"`
	ToUserID   string `json:"to_user_id"`
	GroupID    string `json:"group_id"`
	Content    string `json:"content"`
	MsgType    string `json:"msg_type"`
	CreatedAt  int64  `json:"created_at"`
	IsRead     bool   `json:"is_read"`
	ReadAt     int64  `json:"read_at"`
}

// AddPrivateMessage 添加私聊消息到 Stream
func (so *StreamOperator) AddPrivateMessage(ctx context.Context, msgID, fromUserID, toUserID, content string) (string, error) {
	streamKey := fmt.Sprintf("stream:private:%s", toUserID)
	now := time.Now()

	payload := map[string]interface{}{
		"id":           msgID,
		"from_user_id": fromUserID,
		"to_user_id":   toUserID,
		"content":      content,
		"created_at":   now.Unix(),
		"msg_type":     "text",
		"is_read":      "false",
		"read_at":      "0",
	}

	// 写入 Stream
	msgStreamID, err := so.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: payload,
	}).Result()

	if err != nil {
		log.Printf("Error adding private message to stream: %v", err)
		return "", err
	}

	log.Printf("Private message %s added to stream with ID %s", msgID, msgStreamID)
	return msgStreamID, nil
}

// AddGroupMessage 添加群聊消息到 Stream
func (so *StreamOperator) AddGroupMessage(ctx context.Context, msgID, groupID, fromUserID, content, msgType string) (string, error) {
	streamKey := fmt.Sprintf("stream:group:%s", groupID)
	now := time.Now()

	payload := map[string]interface{}{
		"id":           msgID,
		"group_id":     groupID,
		"from_user_id": fromUserID,
		"content":      content,
		"created_at":   now.Unix(),
		"msg_type":     msgType,
		"is_read":      "false",
		"read_at":      "0",
	}

	// 写入 Stream
	msgStreamID, err := so.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: payload,
	}).Result()

	if err != nil {
		log.Printf("Error adding group message to stream: %v", err)
		return "", err
	}

	log.Printf("Group message %s added to stream with ID %s", msgID, msgStreamID)
	return msgStreamID, nil
}

// ReadMessages 从 Stream 读取消息
func (so *StreamOperator) ReadMessages(ctx context.Context, streamKey string, startID string, count int64) ([]map[string]string, error) {
	if count <= 0 {
		count = 10
	}

	result, err := so.rdb.XRange(ctx, streamKey, startID, "+").Result()
	if err != nil {
		log.Printf("Error reading messages from stream: %v", err)
		return nil, err
	}

	var messages []map[string]string
	for i, entry := range result {
		if int64(i) >= count {
			break
		}

		// 转换 entry.Values 为 map[string]string
		msg := make(map[string]string)
		for k, v := range entry.Values {
			if s, ok := v.(string); ok {
				msg[k] = s
			} else {
				msg[k] = fmt.Sprintf("%v", v)
			}
		}
		msg["stream_id"] = entry.ID
		messages = append(messages, msg)
	}

	return messages, nil
}

// ReadMessagesWithGroup 使用消费者组读取消息
func (so *StreamOperator) ReadMessagesWithGroup(ctx context.Context, streamKey, groupID, userID string, count int64, blockMs time.Duration) ([]map[string]string, error) {
	if count <= 0 {
		count = 10
	}

	if blockMs <= 0 {
		blockMs = 100 * time.Millisecond
	}

	consumerGroup := fmt.Sprintf("%s:consumers", groupID)
	consumerName := fmt.Sprintf("user:%s", userID)

	// 使用 XREADGROUP 读取新消息
	result, err := so.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{streamKey, ">"},
		Count:    count,
		Block:    blockMs,
	}).Result()

	if err != nil {
		if err == context.DeadlineExceeded {
			// 超时，没有新消息
			return []map[string]string{}, nil
		}
		log.Printf("Error reading from consumer group: %v", err)
		return nil, err
	}

	var messages []map[string]string
	for _, streamResult := range result {
		for _, entry := range streamResult.Messages {
			// 转换 entry.Values 为 map[string]string
			msg := make(map[string]string)
			for k, v := range entry.Values {
				if s, ok := v.(string); ok {
					msg[k] = s
				} else {
					msg[k] = fmt.Sprintf("%v", v)
				}
			}
			msg["stream_id"] = entry.ID
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// TrimStream 修剪 Stream（删除老消息）
func (so *StreamOperator) TrimStream(ctx context.Context, streamKey string, maxLen int64) error {
	// 使用 XTRIM MAXLEN 保留最近的消息
	err := so.rdb.XTrimMaxLen(ctx, streamKey, maxLen).Err()
	if err != nil {
		log.Printf("Error trimming stream: %v", err)
		return err
	}

	return nil
}

// TrimStreamByMinID 按最小 ID 修剪 Stream
func (so *StreamOperator) TrimStreamByMinID(ctx context.Context, streamKey string, minID string) error {
	// 删除所有小于 minID 的消息
	err := so.rdb.XTrimMinID(ctx, streamKey, minID).Err()
	if err != nil {
		log.Printf("Error trimming stream by minID: %v", err)
		return err
	}

	return nil
}

// GetStreamLength 获取 Stream 长度
func (so *StreamOperator) GetStreamLength(ctx context.Context, streamKey string) (int64, error) {
	length, err := so.rdb.XLen(ctx, streamKey).Result()
	if err != nil {
		log.Printf("Error getting stream length: %v", err)
		return 0, err
	}

	return length, nil
}

// GetStreamInfo 获取 Stream 信息
func (so *StreamOperator) GetStreamInfo(ctx context.Context, streamKey string) (*redis.XInfoStream, error) {
	info, err := so.rdb.XInfoStream(ctx, streamKey).Result()
	if err != nil {
		log.Printf("Error getting stream info: %v", err)
		return nil, err
	}

	return info, nil
}

// SaveReadState 保存已读状态到 Redis
func (so *StreamOperator) SaveReadState(ctx context.Context, groupID, userID, lastReadMsgID string) error {
	readKey := fmt.Sprintf("read:group:%s:user:%s", groupID, userID)

	// 设置 24 小时过期
	err := so.rdb.Set(ctx, readKey, lastReadMsgID, 24*time.Hour).Err()
	if err != nil {
		log.Printf("Error saving read state: %v", err)
		return err
	}

	// 同时保存时间戳
	timestampKey := fmt.Sprintf("read:group:%s:user:%s:time", groupID, userID)
	err = so.rdb.Set(ctx, timestampKey, time.Now().Unix(), 24*time.Hour).Err()
	if err != nil {
		log.Printf("Error saving read timestamp: %v", err)
		return err
	}

	return nil
}

// GetReadState 获取已读状态
func (so *StreamOperator) GetReadState(ctx context.Context, groupID, userID string) (string, error) {
	readKey := fmt.Sprintf("read:group:%s:user:%s", groupID, userID)

	lastReadMsgID, err := so.rdb.Get(ctx, readKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // 未读状态
		}
		log.Printf("Error getting read state: %v", err)
		return "", err
	}

	return lastReadMsgID, nil
}

// RecordUserOnlineTime 记录用户上线时间
func (so *StreamOperator) RecordUserOnlineTime(ctx context.Context, userID string) error {
	onlineKey := fmt.Sprintf("user:last_online:%s", userID)

	// 记录当前时间（不设置过期，除非需要清理）
	err := so.rdb.Set(ctx, onlineKey, time.Now().Unix(), 0).Err()
	if err != nil {
		log.Printf("Error recording user online time: %v", err)
		return err
	}

	return nil
}

// GetUserLastOnlineTime 获取用户上次上线时间
func (so *StreamOperator) GetUserLastOnlineTime(ctx context.Context, userID string) (int64, error) {
	onlineKey := fmt.Sprintf("user:last_online:%s", userID)

	timeStr, err := so.rdb.Get(ctx, onlineKey).Result()
	if err != nil {
		if err == redis.Nil {
			// 首次登录，返回 7 天前
			return time.Now().AddDate(0, 0, -7).Unix(), nil
		}
		log.Printf("Error getting user last online time: %v", err)
		return 0, err
	}

	var lastOnlineTime int64
	err = json.Unmarshal([]byte(timeStr), &lastOnlineTime)
	if err != nil {
		// 如果解析失败，可能是直接存储的时间戳
		fmt.Sscanf(timeStr, "%d", &lastOnlineTime)
	}

	return lastOnlineTime, nil
}

// CacheUserGroups 缓存用户所在的群列表
func (so *StreamOperator) CacheUserGroups(ctx context.Context, userID string, groups []string) error {
	cacheKey := fmt.Sprintf("user:groups:%s", userID)

	// 使用 Set 结构存储，便于后续操作
	if len(groups) == 0 {
		if err := so.rdb.SAdd(ctx, cacheKey, emptyGroupSentinel).Err(); err != nil {
			log.Printf("Error caching empty user group set: %v", err)
			return err
		}
		so.rdb.Expire(ctx, cacheKey, 1*time.Minute)
		return nil
	}

	for _, groupID := range groups {
		if err := so.rdb.SAdd(ctx, cacheKey, groupID).Err(); err != nil {
			log.Printf("Error caching user group: %v", err)
			return err
		}
	}

	// 设置 1 小时过期
	so.rdb.Expire(ctx, cacheKey, 1*time.Hour)

	return nil
}

// GetCachedUserGroups 获取缓存的用户群列表，第二个返回值表示是否命中缓存
func (so *StreamOperator) GetCachedUserGroups(ctx context.Context, userID string) ([]string, bool, error) {
	cacheKey := fmt.Sprintf("user:groups:%s", userID)

	groups, err := so.rdb.SMembers(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return []string{}, false, nil
		}
		log.Printf("Error getting cached user groups: %v", err)
		return nil, false, err
	}

	filtered := groups[:0]
	for _, g := range groups {
		if g == emptyGroupSentinel {
			continue
		}
		filtered = append(filtered, g)
	}

	return filtered, true, nil
}

// InvalidateUserGroupCache 清除用户群列表缓存
func (so *StreamOperator) InvalidateUserGroupCache(ctx context.Context, userID string) error {
	cacheKey := fmt.Sprintf("user:groups:%s", userID)

	err := so.rdb.Del(ctx, cacheKey).Err()
	if err != nil {
		log.Printf("Error invalidating user group cache: %v", err)
		return err
	}

	return nil
}

// UpdatePrivateMessageAsRead 标记私聊消息为已读（在 Stream 中更新）
func (so *StreamOperator) UpdatePrivateMessageAsRead(ctx context.Context, toUserID, messageID string) error {
	streamKey := fmt.Sprintf("stream:private:%s", toUserID)

	// 读取流中所有消息
	messages, err := so.rdb.XRange(ctx, streamKey, "-", "+").Result()
	if err != nil {
		log.Printf("Error reading stream: %v", err)
		return err
	}

	// 找到目标消息并更新
	now := time.Now().Unix()
	for _, msg := range messages {
		if msg.Values["id"] == messageID {
			// Stream 中的数据不可修改，需要删除后重新添加
			// 或者使用 Redis Hash 存储已读状态
			// 这里使用 Hash 方案更高效
			hashKey := fmt.Sprintf("msg:read:%s", messageID)
			so.rdb.HSet(ctx, hashKey, map[string]interface{}{
				"is_read": "true",
				"read_at": now,
			}).Err()
			log.Printf("Marked message %s as read", messageID)
			return nil
		}
	}

	return fmt.Errorf("message %s not found in stream", messageID)
}

// UpdateGroupMessageAsRead 标记群聊消息为已读
func (so *StreamOperator) UpdateGroupMessageAsRead(ctx context.Context, groupID, messageID string) error {
	// 使用 Hash 存储已读状态
	now := time.Now().Unix()
	hashKey := fmt.Sprintf("msg:read:%s", messageID)

	err := so.rdb.HSet(ctx, hashKey, map[string]interface{}{
		"is_read": "true",
		"read_at": now,
	}).Err()

	if err != nil {
		log.Printf("Error marking message %s as read: %v", messageID, err)
		return err
	}

	log.Printf("Marked group message %s as read", messageID)
	return nil
}

// GetMessageReadStatus 获取消息的已读状态
func (so *StreamOperator) GetMessageReadStatus(ctx context.Context, messageID string) (bool, error) {
	hashKey := fmt.Sprintf("msg:read:%s", messageID)

	result, err := so.rdb.HGet(ctx, hashKey, "is_read").Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil // 消息未读
		}
		log.Printf("Error getting message read status: %v", err)
		return false, err
	}

	return result == "true", nil
}
