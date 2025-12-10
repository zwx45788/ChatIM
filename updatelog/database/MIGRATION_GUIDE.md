#!/bin/bash
# 说明：如何创建和运行新的迁移文件

# 迁移文件命名规则：
# 格式: NNN_description.sql
# 例子: 
#   001_init_schema.sql        (初始化基础表)
#   002_add_user_status.sql    (添加用户状态字段)
#   003_create_analytics.sql   (创建分析表)

# ============================================
# 如何添加新迁移：
# ============================================

# 1. 在 migrations/ 目录创建新文件，例如：
#    migrations/003_add_avatar.sql

# 2. 编写迁移 SQL，例如：
cat > migrations/003_add_avatar.sql << 'EOF'
-- 为 users 表添加头像字段

ALTER TABLE `users` 
ADD COLUMN IF NOT EXISTS `avatar_url` VARCHAR(255) NULL DEFAULT NULL,
ADD COLUMN IF NOT EXISTS `avatar_updated_at` TIMESTAMP NULL DEFAULT NULL;

INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('003_add_avatar');
EOF

# 3. 启动服务（User Service 会自动执行迁移）
#    docker-compose restart user-service

# 4. 查看迁移是否成功：
#    docker logs chatim_user_service | grep -i migration

# ============================================
# 常见迁移模式：
# ============================================

# 添加列
ALTER TABLE table_name 
ADD COLUMN IF NOT EXISTS column_name INT DEFAULT 0;

# 删除列（注意：主键/外键列无法直接删除）
ALTER TABLE table_name 
DROP COLUMN IF EXISTS column_name;

# 修改列类型
ALTER TABLE table_name 
MODIFY COLUMN column_name VARCHAR(255);

# 添加索引
ALTER TABLE table_name 
ADD INDEX IF NOT EXISTS idx_name (column_name);

# 删除索引
ALTER TABLE table_name 
DROP INDEX IF EXISTS idx_name;

# 创建新表
CREATE TABLE IF NOT EXISTS new_table (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

# ============================================
# 迁移的自动执行流程：
# ============================================

# 用户启动 User Service
#   ↓
# User Service 连接到 MySQL
#   ↓
# User Service 调用 migrations.RunMigrations(db)
#   ↓
# 迁移系统：
#   1. 检查 schema_migrations 表是否存在
#   2. 获取 migrations/ 目录下所有 .sql 文件
#   3. 按版本号排序
#   4. 对每个文件检查是否已在 schema_migrations 中记录
#   5. 如果未执行，执行 SQL 并记录版本号
#   ↓
# 返回到应用程序继续启动

# ============================================
# 检查迁移状态：
# ============================================

# 连接到 MySQL 容器
docker exec -it chatim_mysql mysql -u chatim_user -p chatim

# 查询已执行的迁移
mysql> SELECT * FROM schema_migrations;

# 查看某个表的当前结构
mysql> DESCRIBE users;

# ============================================
# 回滚迁移（手动）：
# ============================================

# 如果迁移有问题，需要手动回滚：
# 1. 通过 MySQL 客户端执行反向操作
# 2. 从 schema_migrations 表中删除该版本的记录：

DELETE FROM schema_migrations WHERE version = '003_add_avatar';

# ============================================
# 测试新迁移：
# ============================================

# 1. 清除本地 MySQL 数据卷（仅用于开发环境！）
docker-compose down -v

# 2. 重启所有服务（会重新初始化数据库）
docker-compose up -d

# 3. 查看日志
docker logs -f chatim_user_service
