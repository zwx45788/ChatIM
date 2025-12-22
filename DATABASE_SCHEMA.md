# ChatIM 数据库表结构文档

**生成时间**: 2025年12月16日  
**数据库引擎**: InnoDB  
**字符集**: utf8mb4  
**排序规则**: utf8mb4_unicode_ci

---

## 📊 表结构总览

本项目共包含 **10 个数据表**：

| 表名 | 用途 | 主键 | 外键数量 |
|------|------|------|----------|
| users | 用户信息 | id | 0 |
| messages | 私聊消息 | id | 2 |
| groups | 群组信息 | id | 1 |
| group_members | 群组成员 | (group_id, user_id) | 2 |
| group_messages | 群聊消息 | id | 2 |
| group_read_states | 群聊已读状态 | (group_id, user_id) | 2 |
| friend_requests | 好友请求 | id | 2 |
| friends | 好友关系 | (user_id_1, user_id_2) | 2 |
| group_join_requests | 群加入请求 | id | 3 |
| schema_migrations | 迁移版本记录 | version | 0 |

---

## 1. users - 用户表

**用途**: 存储用户基本信息和状态

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | VARCHAR(36) | PRIMARY KEY | - | 用户ID（UUID） |
| username | VARCHAR(100) | NOT NULL, UNIQUE | - | 用户名（登录用） |
| nickname | VARCHAR(100) | NULL | NULL | 昵称（显示用） |
| password_hash | VARCHAR(255) | NOT NULL | - | 密码哈希值 |
| status | ENUM('online', 'offline', 'away') | NULL | 'offline' | 用户状态 |
| last_seen_at | TIMESTAMP | NULL | NULL | 最后在线时间 |
| avatar | VARCHAR(255) | NULL | NULL | 用户头像URL |
| created_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | id | 主键索引 |
| username | UNIQUE | username | 用户名唯一索引 |
| idx_username | 普通 | username | 用户名查询索引 |
| idx_status | 普通 | status | 状态查询索引 |

### 数据迁移历史
- `001_init_schema.sql`: 初始创建（id, username, nickname, password_hash, created_at, updated_at）
- `002_add_user_status.sql`: 添加 status, last_seen_at, avatar 字段

---

## 2. messages - 私聊消息表

**用途**: 存储一对一私聊消息（持久化备份，主要消息流在 Redis Stream）

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | VARCHAR(36) | PRIMARY KEY | - | 消息ID（UUID） |
| from_user_id | VARCHAR(36) | NOT NULL, FK | - | 发送者ID |
| to_user_id | VARCHAR(36) | NOT NULL, FK | - | 接收者ID |
| content | TEXT | NULL | NULL | 消息内容 |
| is_read | BOOLEAN | NOT NULL | FALSE | 是否已读 |
| read_at | TIMESTAMP | NULL | NULL | 已读时间 |
| created_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 创建时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | from_user_id | users | id | - |
| FK2 | to_user_id | users | id | - |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | id | 主键索引 |
| idx_from_user | 普通 | from_user_id | 发送者查询 |
| idx_to_user | 普通 | to_user_id | 接收者查询 |
| idx_to_user_read | 复合 | (to_user_id, is_read) | 查询未读消息 |

### 数据迁移历史
- `001_init_schema.sql`: 初始创建

---

## 3. groups - 群组表

**用途**: 存储群组基本信息

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | VARCHAR(36) | PRIMARY KEY | - | 群组ID（UUID） |
| name | VARCHAR(100) | NOT NULL | - | 群名称 |
| avatar | VARCHAR(255) | NULL | NULL | 群组头像URL |
| description | TEXT | NULL | NULL | 群描述 |
| creator_id | VARCHAR(36) | NOT NULL, FK | - | 创建者ID |
| is_deleted | BOOLEAN | NOT NULL | FALSE | 软删除标记 |
| created_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | creator_id | users | id | - |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | id | 主键索引 |
| idx_creator | 普通 | creator_id | 创建者查询 |
| idx_created_at | 普通 | created_at | 时间排序 |

### 数据迁移历史
- `001_init_schema.sql`: 初始创建（包含 avatar 字段）
- `003_fix_schema_for_redis_stream.sql`: 确保 avatar 字段存在

---

## 4. group_members - 群组成员表

**用途**: 存储群组成员关系和角色

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| group_id | VARCHAR(36) | PRIMARY KEY, FK | - | 群组ID |
| user_id | VARCHAR(36) | PRIMARY KEY, FK | - | 用户ID |
| role | ENUM('admin', 'member') | NOT NULL | 'member' | 成员角色 |
| is_deleted | BOOLEAN | NOT NULL | FALSE | 软删除标记 |
| joined_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 加入时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | group_id | groups | id | ON DELETE CASCADE |
| FK2 | user_id | users | id | ON DELETE CASCADE |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | (group_id, user_id) | 复合主键 |
| idx_user | 普通 | user_id | 用户的所有群组 |
| idx_group | 普通 | group_id | 群组的所有成员 |

### 数据迁移历史
- `001_init_schema.sql`: 初始创建（包含 is_deleted 字段）
- `003_fix_schema_for_redis_stream.sql`: 确保 is_deleted 字段存在

### 特殊说明
- `role` 字段只有 'admin' 和 'member' 两个值，群主角色通过 `groups.creator_id` 判断
- `is_deleted` 用于软删除，退群时设为 TRUE 而不是真删除记录

---

## 5. group_messages - 群聊消息表

**用途**: 存储群聊消息（持久化备份，消息流在 Redis Stream）

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | VARCHAR(36) | PRIMARY KEY | - | 消息ID（UUID） |
| group_id | VARCHAR(36) | NOT NULL, FK | - | 群组ID |
| from_user_id | VARCHAR(36) | NOT NULL, FK | - | 发送者ID |
| content | TEXT | NOT NULL | - | 消息内容 |
| msg_type | ENUM('text', 'image', 'file', 'notice') | NOT NULL | 'text' | 消息类型 |
| created_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 创建时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | group_id | groups | id | ON DELETE CASCADE |
| FK2 | from_user_id | users | id | - |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | id | 主键索引 |
| idx_group_created | 复合 | (group_id, created_at DESC) | 群消息时间排序 |

### 消息类型说明

| msg_type | 用途 | content 内容格式 |
|----------|------|------------------|
| text | 文本消息 | 纯文本字符串 |
| image | 图片消息 | 图片URL |
| file | 文件消息 | 文件URL |
| notice | 通知消息 | 系统通知文本 |

### 数据迁移历史
- `001_init_schema.sql`: 初始创建（包含 msg_type 字段）
- `003_fix_schema_for_redis_stream.sql`: 删除了 msg_index 字段和相关索引

### 特殊说明
- 消息顺序主要由 Redis Stream 管理，此表仅作持久化备份
- 已删除 `msg_index` 字段，不再使用数据库维护消息序号

---

## 6. group_read_states - 群聊已读状态表

**用途**: 记录用户在各群组的已读状态（备份，主状态在 Redis）

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| group_id | VARCHAR(36) | PRIMARY KEY, FK | - | 群组ID |
| user_id | VARCHAR(36) | PRIMARY KEY, FK | - | 用户ID |
| last_read_msg_id | VARCHAR(36) | NULL | NULL | 最后已读消息ID |
| last_read_at | TIMESTAMP | NULL | NULL | 最后已读时间 |
| updated_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | group_id | groups | id | ON DELETE CASCADE |
| FK2 | user_id | users | id | ON DELETE CASCADE |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | (group_id, user_id) | 复合主键 |
| idx_user_groups | 普通 | user_id | 用户所有群组的已读状态 |
| idx_group_user | 复合 | (group_id, user_id) | 群组内用户已读状态 |

### 数据迁移历史
- `001_init_schema.sql`: 初始创建
- `003_fix_schema_for_redis_stream.sql`: 删除了 last_read_msg_index 和 unread_count 字段

### 特殊说明
- 主要已读状态存储在 Redis Stream 中，此表仅作备份
- 已删除 `last_read_msg_index` 和 `unread_count` 字段，简化为只记录最后已读消息ID

---

## 7. friend_requests - 好友请求表

**用途**: 管理好友添加请求

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | VARCHAR(36) | PRIMARY KEY | - | 请求ID（UUID） |
| from_user_id | VARCHAR(36) | NOT NULL, FK | - | 申请者ID |
| to_user_id | VARCHAR(36) | NOT NULL, FK | - | 接收者ID |
| message | TEXT | NULL | NULL | 申请信息/备注 |
| status | ENUM('pending','accepted','rejected','cancelled') | NOT NULL | 'pending' | 请求状态 |
| created_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 创建时间 |
| processed_at | TIMESTAMP | NULL | NULL | 处理时间 |
| updated_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | from_user_id | users | id | ON DELETE CASCADE |
| FK2 | to_user_id | users | id | ON DELETE CASCADE |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | id | 主键索引 |
| unique_request | UNIQUE | (from_user_id, to_user_id) | 防止重复请求 |
| idx_to_user_status | 复合 | (to_user_id, status) | 接收者的待处理请求 |
| idx_from_user | 普通 | from_user_id | 申请者的所有请求 |
| idx_created_at | 普通 | created_at DESC | 时间排序 |

### 状态说明

| status | 含义 |
|--------|------|
| pending | 待处理 |
| accepted | 已接受 |
| rejected | 已拒绝 |
| cancelled | 已取消（申请者撤回） |

### 数据迁移历史
- `004_friend_and_group_requests.sql`: 初始创建

---

## 8. friends - 好友关系表

**用途**: 存储已建立的好友关系

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| user_id_1 | VARCHAR(36) | PRIMARY KEY, FK | - | 用户ID（较小的ID） |
| user_id_2 | VARCHAR(36) | PRIMARY KEY, FK | - | 用户ID（较大的ID） |
| created_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 添加时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | user_id_1 | users | id | ON DELETE CASCADE |
| FK2 | user_id_2 | users | id | ON DELETE CASCADE |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | (user_id_1, user_id_2) | 复合主键 |
| idx_user1 | 普通 | user_id_1 | 查询user1的好友 |
| idx_user2 | 普通 | user_id_2 | 查询user2的好友 |

### 数据迁移历史
- `004_friend_and_group_requests.sql`: 初始创建

### 特殊说明
- 采用**双向关系单条记录**设计：user_id_1 < user_id_2
- 查询好友时需要检查两个字段：`WHERE user_id_1 = ? OR user_id_2 = ?`
- 这种设计避免了重复记录（A→B 和 B→A）

---

## 9. group_join_requests - 群加入请求表

**用途**: 管理用户申请加入群组的请求

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| id | VARCHAR(36) | PRIMARY KEY | - | 请求ID（UUID） |
| group_id | VARCHAR(36) | NOT NULL, FK | - | 群组ID |
| from_user_id | VARCHAR(36) | NOT NULL, FK | - | 申请者ID |
| message | TEXT | NULL | NULL | 申请信息 |
| status | ENUM('pending','accepted','rejected','cancelled') | NOT NULL | 'pending' | 请求状态 |
| reviewed_by | VARCHAR(36) | NULL, FK | NULL | 处理者ID（群主/管理员） |
| created_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 创建时间 |
| processed_at | TIMESTAMP | NULL | NULL | 处理时间 |
| updated_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP ON UPDATE | 更新时间 |

### 外键约束

| 约束 | 字段 | 引用表 | 引用字段 | 级联操作 |
|------|------|--------|----------|----------|
| FK1 | group_id | groups | id | ON DELETE CASCADE |
| FK2 | from_user_id | users | id | ON DELETE CASCADE |
| FK3 | reviewed_by | users | id | ON DELETE SET NULL |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | id | 主键索引 |
| unique_group_request | UNIQUE | (group_id, from_user_id) | 防止同一用户重复申请同一群 |
| idx_group_status | 复合 | (group_id, status) | 群组的待处理请求 |
| idx_from_user | 普通 | from_user_id | 申请者的所有请求 |
| idx_created_at | 普通 | created_at DESC | 时间排序 |

### 状态说明

| status | 含义 |
|--------|------|
| pending | 待处理 |
| accepted | 已接受（已加入群） |
| rejected | 已拒绝 |
| cancelled | 已取消（申请者撤回） |

### 数据迁移历史
- `004_friend_and_group_requests.sql`: 初始创建

### 特殊说明
- `reviewed_by` 记录是哪个管理员/群主处理的请求
- 群主和管理员都可以处理加群请求

---

## 10. schema_migrations - 迁移版本表

**用途**: 记录已执行的数据库迁移版本

### 字段详情

| 字段名 | 类型 | 约束 | 默认值 | 说明 |
|--------|------|------|--------|------|
| version | VARCHAR(255) | PRIMARY KEY | - | 迁移版本号 |
| executed_at | TIMESTAMP | NOT NULL | CURRENT_TIMESTAMP | 执行时间 |

### 索引

| 索引名 | 类型 | 字段 | 说明 |
|--------|------|------|------|
| PRIMARY | 主键 | version | 主键索引 |

### 数据迁移历史
- `001_init_schema.sql`: 初始创建

### 已执行的迁移记录

| version | 说明 |
|---------|------|
| 001_init_schema | 初始化基础表结构 |
| 002_add_user_status | 添加用户状态和头像字段 |
| 003_fix_schema_for_redis_stream | 修复表结构以匹配 Redis Stream 架构 |
| 004_friend_and_group_requests | 添加好友和群组请求相关表 |

---

## 📈 数据库架构设计要点

### 1. 混合存储架构
- **Redis Stream**: 主消息流，用于实时消息推送和已读状态管理
- **MySQL**: 持久化备份，用于历史消息查询和数据恢复

### 2. 软删除设计
- `groups.is_deleted`: 群组软删除
- `group_members.is_deleted`: 成员退出标记
- 避免级联删除导致的数据丢失

### 3. 消息类型扩展
- 支持 text、image、file、notice 四种消息类型
- 为富媒体消息预留扩展空间

### 4. 好友关系优化
- 采用 user_id_1 < user_id_2 设计
- 单条记录表示双向关系
- 减少50%的存储空间和重复数据

### 5. 请求状态管理
- 统一使用 pending/accepted/rejected/cancelled 状态
- 记录处理时间和处理者
- 支持审计和追溯

### 6. 索引策略
- 外键字段必建索引
- 高频查询字段建复合索引
- 时间排序使用降序索引

---

## 🔧 维护建议

### 数据清理
1. **定期清理已读消息**: Redis Stream 保留最近7天
2. **归档历史消息**: 超过6个月的消息可归档到冷存储
3. **清理已处理请求**: accepted/rejected 状态的请求超过30天可删除

### 性能优化
1. **分区表**: 考虑对 messages 和 group_messages 按时间分区
2. **读写分离**: 消息查询可使用只读从库
3. **缓存热数据**: 用户信息、群组成员列表等使用 Redis 缓存

### 监控指标
1. **表大小增长**: 监控 messages 和 group_messages 表大小
2. **慢查询**: 关注复杂的联表查询
3. **索引使用率**: 定期检查索引是否被有效利用

---

**文档版本**: 1.0  
**最后更新**: 2025年12月16日
