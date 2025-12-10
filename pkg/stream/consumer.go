package stream

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

// StreamConsumerManager 管理 Redis Stream 消费者
type StreamConsumerManager struct {
	rdb *redis.Client
}

// NewStreamConsumerManager 创建消费者管理器
func NewStreamConsumerManager(rdb *redis.Client) *StreamConsumerManager {
	return &StreamConsumerManager{
		rdb: rdb,
	}
}

// InitConsumerGroupForPrivateChat 为私聊初始化消费者组
func (scm *StreamConsumerManager) InitConsumerGroupForPrivateChat(ctx context.Context, userID string) error {
	streamKey := fmt.Sprintf("stream:private:%s", userID)
	consumerGroup := fmt.Sprintf("private:%s:consumers", userID)

	// 创建消费者组（如果不存在）
	err := scm.rdb.XGroupCreateMkStream(ctx, streamKey, consumerGroup, "$").Err()
	if err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Printf("Error creating consumer group for private chat %s: %v", userID, err)
			return err
		}
		// BUSYGROUP 表示组已存在，这是正常的
		log.Printf("Consumer group for private chat %s already exists", userID)
	}

	return nil
}

// InitConsumerGroupForGroup 为群聊初始化消费者组
func (scm *StreamConsumerManager) InitConsumerGroupForGroup(ctx context.Context, groupID string) error {
	streamKey := fmt.Sprintf("stream:group:%s", groupID)
	consumerGroup := fmt.Sprintf("group:%s:consumers", groupID)

	// 创建消费者组（如果不存在）
	err := scm.rdb.XGroupCreateMkStream(ctx, streamKey, consumerGroup, "$").Err()
	if err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Printf("Error creating consumer group for group %s: %v", groupID, err)
			return err
		}
		// BUSYGROUP 表示组已存在，这是正常的
		log.Printf("Consumer group for group %s already exists", groupID)
	}

	return nil
}

// GetUserProgress 获取用户在某个流中的消费进度
func (scm *StreamConsumerManager) GetUserProgress(ctx context.Context, streamKey, groupID, userID string) (string, error) {
	consumerGroup := fmt.Sprintf("%s:consumers", groupID)
	consumerName := fmt.Sprintf("user:%s", userID)

	// 获取消费者信息
	consumers, err := scm.rdb.XInfoConsumers(ctx, streamKey, consumerGroup).Result()
	if err != nil {
		if strings.Contains(err.Error(), "NOGROUP") {
			return "", fmt.Errorf("consumer group not found")
		}
		return "", err
	}

	for _, consumer := range consumers {
		if consumer.Name == consumerName {
			return fmt.Sprintf("%d", consumer.Pending), nil
		}
	}

	return "0", nil
}

// GetPendingMessages 获取用户未确认的消息
func (scm *StreamConsumerManager) GetPendingMessages(ctx context.Context, streamKey, groupID, userID string) ([]string, error) {
	consumerGroup := fmt.Sprintf("%s:consumers", groupID)
	consumerName := fmt.Sprintf("user:%s", userID)

	// 获取待处理消息
	pending, err := scm.rdb.XPending(ctx, streamKey, consumerGroup).Result()
	if err != nil {
		return nil, err
	}

	if pending.Count == 0 {
		return []string{}, nil
	}

	// 获取该消费者的待处理消息
	pendingMessages, err := scm.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   streamKey,
		Group:    consumerGroup,
		Start:    "-",
		End:      "+",
		Count:    int64(pending.Count),
		Consumer: consumerName,
	}).Result()

	if err != nil {
		return nil, err
	}

	var msgIDs []string
	for _, pMsg := range pendingMessages {
		msgIDs = append(msgIDs, pMsg.ID)
	}

	return msgIDs, nil
}

// ClaimPendingMessages 声称待处理消息（用于超时恢复）
func (scm *StreamConsumerManager) ClaimPendingMessages(ctx context.Context, streamKey, groupID, userID string, msgIDs []string) error {
	if len(msgIDs) == 0 {
		return nil
	}

	consumerGroup := fmt.Sprintf("%s:consumers", groupID)
	consumerName := fmt.Sprintf("user:%s", userID)

	// 声称消息（设置 1 小时超时）
	_, err := scm.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   streamKey,
		Group:    consumerGroup,
		Consumer: consumerName,
		Messages: msgIDs,
	}).Result()
	if err != nil {
		log.Printf("Error claiming pending messages: %v", err)
		return err
	}

	return nil
}

// AcknowledgeMessage 确认消息已处理
func (scm *StreamConsumerManager) AcknowledgeMessage(ctx context.Context, streamKey, groupID, msgID string) error {
	consumerGroup := fmt.Sprintf("%s:consumers", groupID)

	// 发送 ACK
	err := scm.rdb.XAck(ctx, streamKey, consumerGroup, msgID).Err()
	if err != nil {
		log.Printf("Error acknowledging message %s: %v", msgID, err)
		return err
	}

	return nil
}

// AcknowledgeMessages 批量确认消息
func (scm *StreamConsumerManager) AcknowledgeMessages(ctx context.Context, streamKey, groupID string, msgIDs []string) error {
	if len(msgIDs) == 0 {
		return nil
	}

	consumerGroup := fmt.Sprintf("%s:consumers", groupID)

	// 批量 ACK
	err := scm.rdb.XAck(ctx, streamKey, consumerGroup, msgIDs...).Err()
	if err != nil {
		log.Printf("Error acknowledging messages: %v", err)
		return err
	}

	return nil
}

// MonitorDeadLetters 监控死信（未处理的消息）
func (scm *StreamConsumerManager) MonitorDeadLetters(ctx context.Context, streamKey, groupID string) (map[string]int64, error) {
	consumerGroup := fmt.Sprintf("%s:consumers", groupID)

	// 获取所有消费者信息
	consumers, err := scm.rdb.XInfoConsumers(ctx, streamKey, consumerGroup).Result()
	if err != nil {
		return nil, err
	}

	deadLetters := make(map[string]int64)
	for _, consumer := range consumers {
		if consumer.Pending > 0 {
			deadLetters[consumer.Name] = consumer.Pending
		}
	}

	return deadLetters, nil
}

// DeleteConsumerGroup 删除消费者组（谨慎使用）
func (scm *StreamConsumerManager) DeleteConsumerGroup(ctx context.Context, streamKey, groupID string) error {
	consumerGroup := fmt.Sprintf("%s:consumers", groupID)

	err := scm.rdb.XGroupDestroy(ctx, streamKey, consumerGroup).Err()
	if err != nil {
		log.Printf("Error deleting consumer group: %v", err)
		return err
	}

	return nil
}

// GetConsumerInfo 获取消费者信息
func (scm *StreamConsumerManager) GetConsumerInfo(ctx context.Context, streamKey, groupID string) ([]redis.XInfoConsumer, error) {
	consumerGroup := fmt.Sprintf("%s:consumers", groupID)

	consumers, err := scm.rdb.XInfoConsumers(ctx, streamKey, consumerGroup).Result()
	if err != nil {
		return nil, err
	}

	return consumers, nil
}
