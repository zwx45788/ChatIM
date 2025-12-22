-- migrations/003_fix_schema_for_redis_stream.sql
-- 修复表结构以匹配 Redis Stream 架构

-- 1. 为 groups 表添加 avatar 字段
-- 为 groups 表添加 avatar 字段（如果不存在）
SET @col_exists := (
	SELECT COUNT(1)
	FROM INFORMATION_SCHEMA.COLUMNS 
	WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'groups' AND COLUMN_NAME = 'avatar'
);
SET @sql := IF(@col_exists = 0,
	'ALTER TABLE `groups` ADD COLUMN `avatar` VARCHAR(255) NULL DEFAULT NULL COMMENT ''群组头像URL''',
	'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. 为 group_members 表添加 is_deleted 字段
-- 为 group_members 表添加 is_deleted 字段（如果不存在）
SET @col_exists := (
	SELECT COUNT(1)
	FROM INFORMATION_SCHEMA.COLUMNS 
	WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'group_members' AND COLUMN_NAME = 'is_deleted'
);
SET @sql := IF(@col_exists = 0,
	'ALTER TABLE `group_members` ADD COLUMN `is_deleted` BOOLEAN DEFAULT FALSE COMMENT ''软删除标记''',
	'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 3. 删除 group_messages 表中未使用的字段和索引
-- 安全删除索引与列（如果存在）
SET @idx_exists := (SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'group_messages' AND INDEX_NAME = 'idx_group_msg_index');
SET @sql := IF(@idx_exists > 0, 'ALTER TABLE `group_messages` DROP INDEX `idx_group_msg_index`', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx_exists := (SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'group_messages' AND INDEX_NAME = 'idx_msg_index');
SET @sql := IF(@idx_exists > 0, 'ALTER TABLE `group_messages` DROP INDEX `idx_msg_index`', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists := (SELECT COUNT(1) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'group_messages' AND COLUMN_NAME = 'msg_index');
SET @sql := IF(@col_exists > 0, 'ALTER TABLE `group_messages` DROP COLUMN `msg_index`', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 4. 删除 group_read_states 表中未使用的字段
-- 删除旧索引与列（如果存在）
-- 注意：`idx_user_groups` 可能用于外键约束索引，不能删除；只在缺失时补充创建

SET @col_exists := (SELECT COUNT(1) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'group_read_states' AND COLUMN_NAME = 'last_read_msg_index');
SET @sql := IF(@col_exists > 0, 'ALTER TABLE `group_read_states` DROP COLUMN `last_read_msg_index`', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists := (SELECT COUNT(1) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'group_read_states' AND COLUMN_NAME = 'unread_count');
SET @sql := IF(@col_exists > 0, 'ALTER TABLE `group_read_states` DROP COLUMN `unread_count`', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 5. 重新创建简化后的索引
-- 重新创建索引（如果不存在）
SET @idx_exists := (SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'group_read_states' AND INDEX_NAME = 'idx_user_groups');
SET @sql := IF(@idx_exists = 0, 'ALTER TABLE `group_read_states` ADD INDEX `idx_user_groups` (user_id)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 记录本次迁移
INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('003_fix_schema_for_redis_stream');
