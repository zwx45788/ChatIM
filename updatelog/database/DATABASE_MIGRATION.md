# 数据库迁移系统完全指南

## 问题回顾

你说得完全正确！**`init.sql` 无法用于增量更新数据库字段。**

### ❌ 为什么 init.sql 无法生效？

```
┌─────────────────────────────────────────────────────────┐
│           第一次 docker-compose up -d                    │
└─────────────────────────────────────────────────────────┘

MySQL 容器启动
  ↓
检查 /docker-entrypoint-initdb.d/ 目录
  ↓
找到 init.sql 文件
  ↓
执行 CREATE TABLE IF NOT EXISTS (创建表)
  ↓
数据保存到 mysql_data 数据卷 (持久化)


┌─────────────────────────────────────────────────────────┐
│           第二次 docker-compose restart                   │
└─────────────────────────────────────────────────────────┘

MySQL 容器启动
  ↓
从 mysql_data 数据卷恢复所有数据 ✓
  ↓
检查 /docker-entrypoint-initdb.d/ 目录
  ↓
❌ 发现数据已存在，SKIP 初始化脚本
  ↓
init.sql 再也不会执行！
  ↓
无法添加新列、新表、新索引等
```

---

## ✅ 解决方案：数据库迁移系统

我为你创建了一个完整的迁移系统。

### 📁 新文件结构

```
ChatIM/
├── migrations/                          # 迁移文件目录
│   ├── 001_init_schema.sql             # 初始化基础表
│   ├── 002_add_user_status.sql         # 添加用户状态字段
│   └── (更多迁移文件...)
│
├── pkg/
│   └── migrations/
│       └── migrations.go                # 迁移引擎（运行迁移）
│
├── cmd/
│   ├── user/
│   │   ├── main.go                     # 已集成迁移调用
│   │   └── Dockerfile                  # 已复制migrations文件夹
│   ├── message/Dockerfile              # 已复制migrations文件夹
│   └── group/Dockerfile                # 已复制migrations文件夹
│
└── MIGRATION_GUIDE.md                   # 迁移使用指南
```

---

## 🔄 工作流程

### 启动流程：

```
docker-compose up -d
  ↓
User Service 启动 (cmd/user/main.go)
  ↓ 1️⃣ 加载配置
  ↓ 2️⃣ 连接到 MySQL
  ↓ 3️⃣ 调用 migrations.RunMigrations(db)
      ├→ 创建 schema_migrations 表（如果不存在）
      ├→ 扫描 ./migrations 目录
      ├→ 读取所有 .sql 文件（按版本号排序）
      ├→ 检查每个文件是否在 schema_migrations 中记录
      └→ 如果未执行，执行 SQL 并记录版本号
  ↓ 4️⃣ 继续启动 gRPC 服务
  ↓ 完成
```

### schema_migrations 表：

```sql
CREATE TABLE schema_migrations (
  version VARCHAR(255) PRIMARY KEY,      -- 迁移文件名，如 "001_init_schema"
  executed_at TIMESTAMP DEFAULT NOW()    -- 执行时间
);

-- 例子：
-- | version                | executed_at         |
-- |------------------------+---------------------|
-- | 001_init_schema        | 2025-01-01 10:00:00 |
-- | 002_add_user_status    | 2025-01-01 10:00:30 |
```

---

## 📝 迁移文件格式

### 标准迁移模板：

```sql
-- migrations/NNN_description.sql
-- 简要说明迁移的目的

-- 实际的SQL语句（可以有多条）
ALTER TABLE `users` 
ADD COLUMN IF NOT EXISTS `status` ENUM('online', 'offline', 'away') DEFAULT 'offline';

ALTER TABLE `users`
ADD INDEX IF NOT EXISTS idx_status (status);

-- 最后记录这个迁移已执行
INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('NNN_description');
```

### 关键要素：

1. **文件名格式**: `NNN_description.sql` (NNN是3位数字版本号)
2. **使用 IF NOT EXISTS**: 确保迁移可重复运行（幂等性）
3. **每个SQL语句以 `;` 结尾**
4. **最后插入到 schema_migrations 表**

---

## 🚀 如何添加新迁移

### 场景：添加用户头像字段

#### 步骤 1️⃣：创建迁移文件

```bash
# Windows PowerShell
New-Item -Path "migrations\003_add_user_avatar.sql"

# 或使用 VS Code 直接创建
```

#### 步骤 2️⃣：编写 SQL

```sql
-- migrations/003_add_user_avatar.sql
-- 为用户表添加头像字段

ALTER TABLE `users` 
ADD COLUMN IF NOT EXISTS `avatar_url` VARCHAR(255) NULL DEFAULT NULL,
ADD COLUMN IF NOT EXISTS `avatar_updated_at` TIMESTAMP NULL DEFAULT NULL;

-- 创建索引（可选）
ALTER TABLE `users`
ADD INDEX IF NOT EXISTS idx_avatar_updated (avatar_updated_at);

-- 记录迁移
INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('003_add_user_avatar');
```

#### 步骤 3️⃣：启动服务（迁移自动执行）

```bash
# 开发环境
go run ./cmd/user/main.go

# Docker 环境
docker-compose restart user-service

# 查看日志
docker logs -f chatim_user_service | grep -i migration
```

#### 步骤 4️⃣：验证迁移成功

```bash
# 进入 MySQL 容器
docker exec -it chatim_mysql mysql -u chatim_user -p chatim

# 查询迁移记录
mysql> SELECT * FROM schema_migrations;
-- 输出：
-- +-----------------------+---------------------+
-- | version               | executed_at         |
-- +-----------------------+---------------------+
-- | 001_init_schema       | 2025-01-01 10:00:00 |
-- | 002_add_user_status   | 2025-01-01 10:00:30 |
-- | 003_add_user_avatar   | 2025-01-01 10:01:00 |
-- +-----------------------+---------------------+

# 验证新列已添加
mysql> DESCRIBE users;
-- 会看到 avatar_url 和 avatar_updated_at 列
```

---

## 📊 已创建的迁移

### 001_init_schema.sql

初始化所有基础表：
- `users` - 用户表
- `messages` - 私聊消息表
- `groups` - 群组表
- `group_members` - 群成员表
- `group_messages` - 群消息表
- `group_read_states` - 群已读状态表
- `schema_migrations` - 迁移追踪表

### 002_add_user_status.sql

示例迁移，展示如何添加新列：
- 添加 `status` 字段 (online/offline/away)
- 添加 `last_seen_at` 时间戳
- 添加 `avatar_url` 头像URL
- 添加索引以优化查询

---

## 🔧 常见迁移操作

### 添加列

```sql
ALTER TABLE `messages` 
ADD COLUMN IF NOT EXISTS `deleted_at` TIMESTAMP NULL DEFAULT NULL;
```

### 删除列

```sql
ALTER TABLE `messages` 
DROP COLUMN IF EXISTS deprecated_field;
```

### 修改列类型

```sql
ALTER TABLE `users` 
MODIFY COLUMN nickname VARCHAR(200);  -- 从 100 改为 200
```

### 添加索引

```sql
ALTER TABLE `group_messages` 
ADD INDEX IF NOT EXISTS idx_new_field (new_field);
```

### 删除索引

```sql
ALTER TABLE `group_messages` 
DROP INDEX IF EXISTS idx_old_field;
```

### 创建新表

```sql
CREATE TABLE IF NOT EXISTS `analytics` (
  `id` VARCHAR(36) PRIMARY KEY,
  `event_type` VARCHAR(100) NOT NULL,
  `user_id` VARCHAR(36),
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_event (event_type),
  INDEX idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

## ⚠️ 迁移最佳实践

### ✅ 推荐做法

1. **每个迁移只做一件事**
   ```sql
   -- ✅ 好：一个迁移添加一组相关列
   -- migrations/003_add_user_profile.sql
   ALTER TABLE users ADD COLUMN avatar_url VARCHAR(255);
   ALTER TABLE users ADD COLUMN bio TEXT;
   ```

2. **使用 IF NOT EXISTS**
   ```sql
   -- ✅ 好：幂等（可安全重复执行）
   ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(255);
   
   -- ❌ 差：非幂等（重复执行会报错）
   ALTER TABLE users ADD COLUMN avatar_url VARCHAR(255);
   ```

3. **版本号按时间顺序**
   ```
   001_init_schema.sql
   002_add_user_status.sql
   003_add_analytics.sql
   ```

4. **在注释中说明目的**
   ```sql
   -- 添加用户在线状态和头像支持
   -- 用于支持新的用户档案功能
   ALTER TABLE users ADD COLUMN IF NOT EXISTS status ...
   ```

### ❌ 避免的做法

1. **不要跳过版本号**
   ```
   ❌ 001, 003, 005  (跳过了 002, 004)
   ✅ 001, 002, 003, 004, 005  (连续)
   ```

2. **不要修改已执行的迁移**
   ```
   ❌ 修改 001_init_schema.sql 的内容
   ✅ 创建新迁移 003_fix_schema.sql 来修正问题
   ```

3. **不要在迁移中混入应用逻辑**
   ```
   ❌ 数据库迁移中不要使用 DELETE / UPDATE 来修改数据
   ✅ 只修改表结构（DDL），不修改数据（DML）
   ```

---

## 🆘 故障排除

### 问题 1️⃣：迁移没有执行

```bash
# 查看日志
docker logs chatim_user_service | grep -i migration

# 检查迁移文件位置
docker exec chatim_user_service ls -la /root/migrations/

# 检查 MySQL 中的迁移记录
mysql> SELECT * FROM schema_migrations;
```

### 问题 2️⃣：迁移执行失败

```bash
# 查看详细错误日志
docker logs chatim_user_service

# 手动执行迁移 SQL 来调试
docker exec chatim_mysql mysql -u chatim_user -p chatim < migrations/003_add_avatar.sql
```

### 问题 3️⃣：需要回滚迁移

```bash
# 连接到 MySQL
docker exec -it chatim_mysql mysql -u chatim_user -p chatim

# 1. 执行反向操作（手动）
mysql> ALTER TABLE users DROP COLUMN avatar_url;

# 2. 从迁移表中移除记录
mysql> DELETE FROM schema_migrations WHERE version = '003_add_user_avatar';

# 3. 重启服务，迁移会被重新执行
docker-compose restart user-service
```

---

## 📈 完整的迁移生命周期

```
开发者提交新迁移
  ↓
003_add_user_avatar.sql 被添加到 migrations/ 目录
  ↓
构建 Docker 镜像
  ↓ Dockerfile 复制 migrations 文件夹
  ↓
docker-compose up -d
  ↓
User Service 启动
  ↓
调用 migrations.RunMigrations(db)
  ↓
✅ 迁移自动执行
  ├→ 检查 schema_migrations 表
  ├→ 发现 003_add_user_avatar 未执行
  ├→ 执行 ALTER TABLE 语句
  ├→ 插入 schema_migrations 记录
  └→ 打印日志
     "→ Running migration: 003_add_user_avatar"
     "✓ Migration 003_add_user_avatar executed successfully"
  ↓
服务正常运行
  ↓
下次启动时
  ↓
迁移系统检查发现 003_add_user_avatar 已执行
  ↓
跳过该迁移，继续执行新迁移（如果有）
```

---

## 🎯 总结

| 方面 | init.sql | 迁移系统 |
|------|----------|---------|
| 初始化 | ✅ 有效 | ✅ 有效 |
| 添加字段 | ❌ 无法工作 | ✅ 自动执行 |
| 修改表结构 | ❌ 无法工作 | ✅ 自动执行 |
| 版本追踪 | ❌ 无法跟踪 | ✅ 完整记录 |
| 可重复执行 | ❌ 会报错 | ✅ 幂等设计 |
| 生产就绪 | ❌ 不推荐 | ✅ 生产级别 |

---

**现在你可以随时添加新的迁移文件，所有服务启动时会自动执行！** 🚀
