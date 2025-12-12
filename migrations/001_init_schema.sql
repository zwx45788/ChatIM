-- migrations/001_init_schema.sql
-- 初始化基础表结构

-- 创建 users 表
CREATE TABLE IF NOT EXISTS `users` (
  `id` VARCHAR(36) PRIMARY KEY,
  `username` VARCHAR(100) NOT NULL UNIQUE,
  `nickname` VARCHAR(100),
  `password_hash` VARCHAR(255) NOT NULL,
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建 messages 表（一对一消息）
CREATE TABLE IF NOT EXISTS `messages` (
  `id` VARCHAR(36) PRIMARY KEY,
  `from_user_id` VARCHAR(36) NOT NULL,
  `to_user_id` VARCHAR(36) NOT NULL,
  `content` TEXT,
  `is_read` BOOLEAN DEFAULT FALSE,
  `read_at` TIMESTAMP NULL DEFAULT NULL,
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (from_user_id) REFERENCES users(id),
  FOREIGN KEY (to_user_id) REFERENCES users(id),
  INDEX idx_from_user (from_user_id),
  INDEX idx_to_user (to_user_id),
  INDEX idx_to_user_read (to_user_id, is_read)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建 groups 表（群组）
CREATE TABLE IF NOT EXISTS `groups` (
  `id` VARCHAR(36) PRIMARY KEY,
  `name` VARCHAR(100) NOT NULL,
  `description` TEXT,
  `creator_id` VARCHAR(36) NOT NULL,
  `is_deleted` BOOLEAN DEFAULT FALSE,
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (creator_id) REFERENCES users(id),
  INDEX idx_creator (creator_id),
  INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建 group_members 表（群组成员）
CREATE TABLE IF NOT EXISTS `group_members` (
  `group_id` VARCHAR(36),
  `user_id` VARCHAR(36),
  `role` ENUM('admin', 'member') DEFAULT 'member',
  `joined_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, user_id),
  FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  INDEX idx_user (user_id),
  INDEX idx_group (group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建 group_messages 表（群聊消息）
CREATE TABLE IF NOT EXISTS `group_messages` (
  `id` VARCHAR(36) PRIMARY KEY,
  `msg_index` BIGINT AUTO_INCREMENT UNIQUE,
  `group_id` VARCHAR(36) NOT NULL,
  `from_user_id` VARCHAR(36) NOT NULL,
  `content` TEXT NOT NULL,
  `msg_type` ENUM('text', 'image', 'file', 'notice') DEFAULT 'text',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE,
  FOREIGN KEY (from_user_id) REFERENCES users(id),
  INDEX idx_group_msg_index (group_id, msg_index DESC),
  INDEX idx_group_created (group_id, created_at DESC),
  INDEX idx_msg_index (msg_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建 group_read_states 表（群聊已读状态）
CREATE TABLE IF NOT EXISTS `group_read_states` (
  `group_id` VARCHAR(36),
  `user_id` VARCHAR(36),
  `last_read_msg_index` BIGINT,
  `last_read_msg_id` VARCHAR(36),
  `last_read_at` TIMESTAMP,
  `unread_count` INT DEFAULT 0,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, user_id),
  FOREIGN KEY (group_id) REFERENCES `groups`(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (last_read_msg_id) REFERENCES group_messages(id),
  INDEX idx_user_groups (user_id, unread_count),
  INDEX idx_group_user (group_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建迁移版本表（用于追踪哪些迁移已执行）
CREATE TABLE IF NOT EXISTS `schema_migrations` (
  `version` VARCHAR(255) PRIMARY KEY,
  `executed_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 记录本次迁移
INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('001_init_schema');
