-- migrations/003_fix_schema_for_redis_stream.sql
-- 修复表结构以匹配 Redis Stream 架构

-- 1. 为 groups 表添加 avatar 字段
ALTER TABLE `groups` 
ADD COLUMN IF NOT EXISTS `avatar` VARCHAR(255) NULL DEFAULT NULL COMMENT '群组头像URL';

-- 2. 为 group_members 表添加 is_deleted 字段（软删除标记）
ALTER TABLE `group_members` 
ADD COLUMN IF NOT EXISTS `is_deleted` BOOLEAN DEFAULT FALSE COMMENT '软删除标记';

-- 3. 删除 group_messages 表中未使用的 msg_index 字段和相关索引
-- 因为消息顺序由 Redis Stream 管理，数据库仅作持久化备份
ALTER TABLE `group_messages` 
DROP INDEX IF EXISTS `idx_group_msg_index`,
DROP INDEX IF EXISTS `idx_msg_index`,
DROP COLUMN IF EXISTS `msg_index`;

-- 4. 删除 group_read_states 表中未使用的字段
-- 主要已读状态存储在 Redis Stream 中，此表仅作备份
ALTER TABLE `group_read_states` 
DROP INDEX IF EXISTS `idx_user_groups`,
DROP COLUMN IF EXISTS `last_read_msg_index`,
DROP COLUMN IF EXISTS `unread_count`;

-- 5. 重新创建简化后的索引
ALTER TABLE `group_read_states` 
ADD INDEX IF NOT EXISTS `idx_user_groups` (user_id);

-- 记录本次迁移
INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('003_fix_schema_for_redis_stream');
