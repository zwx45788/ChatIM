-- migrations/002_add_user_status.sql
-- 例子：为 users 表添加状态字段

-- 只有在字段不存在时才添加（迁移安全）
ALTER TABLE `users` 
ADD COLUMN `status` ENUM('online', 'offline', 'away') DEFAULT 'offline',
ADD COLUMN `last_seen_at` TIMESTAMP NULL DEFAULT NULL,
ADD COLUMN `avatar_url` VARCHAR(255) NULL DEFAULT NULL;

-- 为新字段添加索引
ALTER TABLE `users`
ADD INDEX idx_status (status);

-- 记录本次迁移
INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('002_add_user_status');
