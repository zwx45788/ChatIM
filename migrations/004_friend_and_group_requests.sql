-- 好友请求表
CREATE TABLE IF NOT EXISTS `friend_requests` (
  `id` VARCHAR(36) PRIMARY KEY,
  `from_user_id` VARCHAR(36) NOT NULL COMMENT '申请者ID',
  `to_user_id` VARCHAR(36) NOT NULL COMMENT '接收者ID',
  `message` TEXT COMMENT '申请信息/备注',
  `status` ENUM('pending','accepted','rejected','cancelled') DEFAULT 'pending' COMMENT '请求状态',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `processed_at` TIMESTAMP NULL DEFAULT NULL COMMENT '处理时间',
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  FOREIGN KEY (from_user_id) REFERENCES `users`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (to_user_id) REFERENCES `users`(`id`) ON DELETE CASCADE,
  UNIQUE KEY unique_request (from_user_id, to_user_id),
  INDEX idx_to_user_status (to_user_id, status),
  INDEX idx_from_user (from_user_id),
  INDEX idx_created_at (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='好友请求表';

-- 好友关系表
CREATE TABLE IF NOT EXISTS `friends` (
  `user_id_1` VARCHAR(36) NOT NULL COMMENT '用户ID（较小的ID）',
  `user_id_2` VARCHAR(36) NOT NULL COMMENT '用户ID（较大的ID）',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '添加时间',
  PRIMARY KEY (user_id_1, user_id_2),
  FOREIGN KEY (user_id_1) REFERENCES `users`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (user_id_2) REFERENCES `users`(`id`) ON DELETE CASCADE,
  INDEX idx_user1 (user_id_1),
  INDEX idx_user2 (user_id_2)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='好友关系表';

-- 群加入请求表
CREATE TABLE IF NOT EXISTS `group_join_requests` (
  `id` VARCHAR(36) PRIMARY KEY,
  `group_id` VARCHAR(36) NOT NULL COMMENT '群组ID',
  `from_user_id` VARCHAR(36) NOT NULL COMMENT '申请者ID',
  `message` TEXT COMMENT '申请信息',
  `status` ENUM('pending','accepted','rejected','cancelled') DEFAULT 'pending' COMMENT '请求状态',
  `reviewed_by` VARCHAR(36) DEFAULT NULL COMMENT '处理者ID（群主/管理员）',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `processed_at` TIMESTAMP NULL DEFAULT NULL COMMENT '处理时间',
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  FOREIGN KEY (group_id) REFERENCES `groups`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (from_user_id) REFERENCES `users`(`id`) ON DELETE CASCADE,
  FOREIGN KEY (reviewed_by) REFERENCES `users`(`id`) ON DELETE SET NULL,
  UNIQUE KEY unique_group_request (group_id, from_user_id),
  INDEX idx_group_status (group_id, status),
  INDEX idx_from_user (from_user_id),
  INDEX idx_created_at (created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群加入请求表';
