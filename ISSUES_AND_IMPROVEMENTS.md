# ChatIM 系统问题与改进清单

> 更新日期：2025年12月14日

## 📋 目录
- [已修复的问题](#已修复的问题)
- [当前存在的问题](#当前存在的问题)
- [缺失的核心功能](#缺失的核心功能)
- [性能优化建议](#性能优化建议)
- [安全性建议](#安全性建议)

---

## ✅ 已修复的问题

### 1. 消息已读逻辑优化
- **问题描述**：原先在拉取消息时自动标记为已读，可能导致消息未成功送达前端就被标记为已读
- **解决方案**：
  - 移除 `PullMessages` 中的 `AutoMark` 自动标记逻辑
  - 移除 `PullUnreadMessages` 中的 `AutoMark` 自动标记逻辑
  - 前端需要在成功接收消息后主动调用标记已读接口
- **相关文件**：`internal/message_service/handler/message.go`
- **状态**：✅ 已完成

### 2. 数据库表结构修复
- **问题描述**：数据库表结构与代码逻辑不匹配
- **修复内容**：
  - ✅ `users.avatar`：从 `avatar_url` 统一改为 `avatar`
  - ✅ `groups.avatar`：新增群组头像字段
  - ✅ `group_members.is_deleted`：新增软删除标记字段
  - ✅ `group_messages.msg_index`：删除未使用的消息序号字段
  - ✅ `group_read_states`：简化表结构，删除 `last_read_msg_index` 和 `unread_count`
- **相关文件**：
  - `init.sql`
  - `migrations/001_init_schema.sql`
  - `migrations/002_add_user_status.sql`
  - `migrations/003_fix_schema_for_redis_stream.sql` (新增)
- **状态**：✅ 已完成

### 3. 群聊消息架构优化
- **问题描述**：`pullGroupUnread` 方法从错误的 Stream 读取群聊消息
- **解决方案**：
  - 统一消息存储架构：群聊消息写入每个成员的 `stream:private:{user_id}`
  - 重构 `PullAllUnreadOnLogin` 方法，直接从用户的私聊流中读取并分组消息
  - 删除 `pullPrivateUnread` 和 `pullGroupUnread` 方法
  - 删除 `getUserGroups` 和 `convertStreamEntryToMessage` 辅助函数
- **相关文件**：`internal/message_service/handler/message.go`
- **状态**：✅ 已完成

---

## ⚠️ 当前存在的问题

### 1. 消息推送通知机制 ✅ 已完成
**优先级：高** 🔴

**问题描述**：
- `SendMessage` 方法在发送消息后没有发布 Redis 通知
- `SendGroupMessage` 方法同样缺少消息通知发布
- WebSocket 订阅者无法接收到新消息通知

**解决方案**：✅ 已实现
1. ✅ 在 `SendMessage` 中添加 Redis 通知发布
2. ✅ 在 `SendGroupMessage` 中为每个成员发布通知
3. ✅ 优化 `subscribePrivateMessages`，直接使用通知中的数据推送
4. ✅ 移除不必要的数据库查询，提升性能

**实现详情**：
```go
// 私聊消息通知
notification := map[string]interface{}{
    "msg_id":      msgID,
    "to_user_id":  toUserID,
    "from_user_id": fromUserID,
    "type":        "private",
    "content":     content,
    "created_at":  timestamp,
}
h.rdb.Publish(ctx, "message_notifications", notificationJSON)

// 群聊消息通知（循环发送给每个成员）
for _, memberID := range memberIDs {
    notification := map[string]interface{}{
        "msg_id":      msgID,
        "to_user_id":  memberID,
        "from_user_id": fromUserID,
        "group_id":    groupID,
        "type":        "group",
        "content":     content,
        "created_at":  timestamp,
    }
    h.rdb.Publish(ctx, "message_notifications", notificationJSON)
}
```

**优势**：
- ✅ 真正的实时推送
- ✅ 无需轮询，性能更优
- ✅ 直接推送，减少数据库查询
- ✅ 支持私聊和群聊统一架构

**相关文件**：
- `internal/message_service/handler/message.go` ✅ 已修改
- `internal/websocket/subscriber.go` ✅ 已优化

---

### 2. 离线消息处理流程不明确
**优先级：中** 🟡

**问题描述**：
- `PullAllUnreadOnLogin` 已实现离线消息拉取
- 但登录流程中没有自动调用
- 前端需要明确在何时调用此接口

**建议解决方案**：
1. 在用户登录成功后自动调用 `PullAllUnreadOnLogin`
2. 或在 WebSocket 连接建立后自动推送离线消息
3. 在 API 文档中明确说明离线消息拉取流程

**相关文件**：
- `internal/user_service/handler/user.go` (Login 方法)
- `internal/websocket/handler.go` (HandleWebSocket 方法)

---

### 3. 群组管理功能 ✅ 已完成
**优先级：中** 🟡

**已实现功能**：
- ✅ 创建群组 (`CreateGroup`)
- ✅ 获取群组信息 (`GetGroupInfo`)
- ✅ 添加成员 (`AddGroupMember`)
- ✅ 移除成员 (`RemoveGroupMember`)
- ✅ 退出群组 (`LeaveGroup`)
- ✅ 列出群组 (`ListGroups`)
- ✅ 转让群主 (`TransferOwner`)
- ✅ 修改群信息 (`UpdateGroupInfo` - 名称、头像、描述)
- ✅ 解散群组 (`DismissGroup`)
- ✅ 设置/取消管理员 (`SetAdmin`)
- ✅ 获取成员列表 (`GetGroupMembers`)

**实现详情**：
- Proto 定义：11 个消息类型，15 个 RPC 方法
- gRPC Handler：11 个完整实现（~1022 行）
- API Gateway：11 个 HTTP Handler
- REST API：11 个路由
- 权限控制：群主、管理员、成员三级权限
- 事务支持：转让群主、解散群组使用事务保证一致性

**接口列表**：
- `PUT /api/v1/groups/:group_id/info` - 修改群信息
- `POST /api/v1/groups/:group_id/transfer` - 转让群主
- `POST /api/v1/groups/:group_id/dismiss` - 解散群组
- `POST /api/v1/groups/:group_id/admin` - 设置/取消管理员
- `GET /api/v1/groups/:group_id/members` - 获取成员列表

**相关文件**：
- `internal/group_service/handler/group.go` (新增 ~375 行)
- `api/proto/group/group.proto` (新增 6 个消息和 5 个 RPC)
- `internal/api_gateway/handler/handler.go` (新增 5 个 handler)
- `cmd/api/main.go` (新增 5 个路由)
- `docs/GROUP_MANAGEMENT_API.md` (完整 API 文档)

**状态**：✅ 已完成

---

### 4. 群加入请求功能 ✅ 已完成
**优先级：中** 🟡

**实现内容**：
- ✅ Proto 定义：`SendGroupJoinRequest`, `HandleGroupJoinRequest`, `GetGroupJoinRequests`, `GetMyGroupJoinRequests`
- ✅ gRPC Handler 实现：4 个新的 RPC 方法
- ✅ API 网关路由：4 个新的 REST API 接口
- ✅ 权限控制：管理员审核、用户查询自己的申请
- ✅ 业务逻辑：防重复申请、状态流转、自动加群

**接口列表**：
- `POST /api/v1/groups/join-requests` - 发送加群申请
- `POST /api/v1/groups/join-requests/handle` - 处理加群申请（管理员）
- `GET /api/v1/groups/:group_id/join-requests` - 获取群的申请列表（管理员）
- `GET /api/v1/groups/join-requests/my` - 获取我的申请列表

**相关文件**：
- `internal/group_service/handler/group.go` (新增 244 行代码)
- `internal/api_gateway/handler/handler.go` (新增 4 个 handler)
- `cmd/api/main.go` (新增 4 个路由)
- `api/proto/group/group.proto` (新增 9 个消息和 4 个 RPC)
- `docs/GROUP_JOIN_REQUEST_API.md` (新增完整 API 文档)

**状态**：✅ 已完成

---

### 5. 用户头像字段不一致
**优先级：低** 🟢

**问题描述**：
- `migrations/002_add_user_status.sql` 中添加的是 `avatar` 字段
- 但 `init.sql` 中 `users` 表缺少此字段

**解决方案**：
在 `init.sql` 的 `users` 表中添加：
```sql
`avatar` VARCHAR(255) NULL DEFAULT NULL COMMENT '用户头像URL',
`status` ENUM('online', 'offline', 'away') DEFAULT 'offline',
`last_seen_at` TIMESTAMP NULL DEFAULT NULL,
```

**相关文件**：
- `init.sql`

---

## ❌ 缺失的核心功能

### 1. 富媒体消息支持
**优先级：高** 🔴

**当前状态**：
- 表结构支持 `msg_type` (text/image/file/notice)
- 但缺少完整的实现

**需要实现**：
1. **文件上传接口**
   - 支持图片、文件上传
   - 返回文件 URL
   - 文件大小限制
   - 文件类型验证

2. **存储方案**
   - 本地存储 或
   - 对象存储（OSS/S3）

3. **消息结构扩展**
   ```json
   {
     "msg_type": "image",
     "content": "https://cdn.example.com/image.jpg",
     "metadata": {
       "filename": "photo.jpg",
       "size": 123456,
       "width": 1920,
       "height": 1080
     }
   }
   ```

**建议实现文件**：
- `internal/api_gateway/handler/upload.go`
- `pkg/storage/storage.go`

---

### 2. 消息撤回功能
**优先级：中** 🟡

**需要实现**：
1. 撤回时限检查（如 2 分钟内）
2. 权限检查（只能撤回自己的消息）
3. 撤回记录（用于显示"XX 撤回了一条消息"）
4. Redis Stream 中标记消息为已撤回
5. 数据库中记录撤回状态

**建议数据库修改**：
```sql
ALTER TABLE messages ADD COLUMN is_recalled BOOLEAN DEFAULT FALSE;
ALTER TABLE group_messages ADD COLUMN is_recalled BOOLEAN DEFAULT FALSE;
```

**建议接口**：
```protobuf
rpc RecallMessage(RecallMessageRequest) returns (RecallMessageResponse);
```

---

### 3. 用户/群组搜索功能 ✅ 已完成
**优先级：中** 🟡

**功能描述**：
- ✅ 搜索用户（按用户名或昵称）
- ✅ 搜索群组（按群名或描述）
- ✅ 智能排序（完全匹配 > 前缀匹配 > 包含匹配）
- ✅ 分页支持
- ✅ 群组按成员数量排序

**实现详情**：
1. **Proto 定义**：
   - `api/proto/user.proto`: SearchUsers RPC 和相关消息
   - `api/proto/group/group.proto`: SearchGroups RPC 和相关消息

2. **后端实现**：
   - `internal/user_service/handler/user.go`: SearchUsers 方法 (~80 行)
   - `internal/group_service/handler/group.go`: SearchGroups 方法 (~88 行)
   - 使用 LIKE 查询，支持模糊匹配
   - CASE WHEN 语句实现智能排序

3. **API Gateway**：
   - `GET /api/v1/search/users?keyword=xxx&limit=20&offset=0`
   - `GET /api/v1/search/groups?keyword=xxx&limit=20&offset=0`

4. **文档**：
   - `docs/SEARCH_API.md`: 完整的 API 文档

**优势**：
- ✅ 用户可以搜索添加新好友
- ✅ 用户可以搜索加入群组
- ✅ 智能排序提升用户体验
- ✅ 分页支持大量搜索结果

---

### 4. 消息内容搜索功能
**优先级：低** 🟢

**需要实现**：
1. 按关键词搜索消息内容
2. 按发送者搜索
3. 按时间范围搜索
4. 搜索结果分页

**技术方案**：
- 使用 MySQL 全文索引
- 或集成 Elasticsearch

**建议接口**：
```protobuf
rpc SearchMessages(SearchMessagesRequest) returns (SearchMessagesResponse);
```

---

### 5. @提及功能
**优先级：低** 🟢

**需要实现**：
1. 消息中解析 @ 提及
2. 被 @ 的用户收到特殊通知
3. 未读 @ 消息计数
4. 查看所有 @ 我的消息

**数据库修改**：
```sql
CREATE TABLE message_mentions (
  message_id VARCHAR(36),
  mentioned_user_id VARCHAR(36),
  is_read BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (message_id, mentioned_user_id)
);
```

---

### 6. 消息转发功能
**优先级：低** 🟢

**需要实现**：
1. 转发给好友
2. 转发到群组
3. 转发时可添加备注
4. 标记转发来源

---

### 7. 会话置顶/免打扰 ✅ 部分完成
**优先级：低** 🟢

**已实现功能**：
- ✅ 会话置顶 (`POST /api/v1/conversations/:id/pin`)
- ✅ 取消置顶 (`DELETE /api/v1/conversations/:id/pin`)
- ✅ 删除会话 (`DELETE /api/v1/conversations/:id`)
- ✅ 获取会话列表 (`GET /api/v1/conversations`)

**待实现**：
1. 消息免打扰（接收但不通知）

**数据库表**：
```sql
CREATE TABLE conversation_settings (
  user_id VARCHAR(36),
  conversation_id VARCHAR(36),
  is_pinned BOOLEAN DEFAULT FALSE,
  is_muted BOOLEAN DEFAULT FALSE,  -- 👈 免打扰功能待实现
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, conversation_id)
);
```

---

## 🚀 性能优化建议

### 1. Redis Stream 定期清理
**优先级：中** 🟡

**问题描述**：
- Redis Stream 中的消息会一直累积
- 可能导致内存占用过高

**建议方案**：
1. 定期清理已读消息（如 7 天后）
2. 使用 `XTRIM` 命令限制 Stream 长度
3. 或使用 Redis 的 MAXLEN 参数

**示例代码**：
```go
// 添加消息时限制最大长度
rdb.XAdd(ctx, &redis.XAddArgs{
    Stream: streamKey,
    MaxLen: 1000,  // 最多保留 1000 条消息
    Values: payload,
})
```

---

### 2. 数据库查询优化
**优先级：中** 🟡

**建议优化**：
1. 为高频查询添加复合索引
2. 使用连接池优化连接管理
3. 分页查询避免大量数据返回
4. 使用 EXPLAIN 分析慢查询

**建议添加的索引**：
```sql
-- messages 表
CREATE INDEX idx_created_at ON messages(created_at DESC);

-- group_messages 表
CREATE INDEX idx_from_user ON group_messages(from_user_id);

-- friends 表
CREATE INDEX idx_created_at ON friends(created_at DESC);
```

---

### 3. 缓存策略优化
**优先级：低** 🟢

**当前缓存**：
- ✅ 群成员列表缓存
- ✅ 用户所在群组缓存

**建议增加缓存**：
1. 用户信息缓存（减少数据库查询）
2. 好友列表缓存
3. 群组信息缓存
4. 会话列表缓存

**缓存过期策略**：
- 用户信息：1 小时
- 好友列表：30 分钟
- 群组信息：10 分钟

---

## 🔒 安全性建议

### 1. 输入验证加强
**优先级：高** 🔴

**需要加强的验证**：
1. 消息内容长度限制
2. 群组名称、描述长度限制
3. 文件上传大小和类型限制
4. 防止 XSS 攻击（消息内容过滤）
5. 防止 SQL 注入（使用参数化查询）

---

### 2. 权限控制完善
**优先级：高** 🔴

**需要添加的权限检查**：
1. 只能发送消息给好友（可选配置）
2. 群组操作权限检查（管理员 vs 普通成员）
3. 消息撤回权限检查
4. 查看群组信息权限检查（是否为成员）

---

### 3. 速率限制
**优先级：中** 🟡

**建议添加限流**：
1. 消息发送频率限制（防止刷屏）
2. 好友请求发送限制
3. API 请求频率限制
4. 文件上传频率限制

**实现方式**：
- 使用 Redis 计数器
- 或使用 `golang.org/x/time/rate` 库

---

### 4. WebSocket 安全
**优先级：中** 🟡

**建议改进**：
1. WebSocket 连接超时机制
2. 心跳检测（防止僵尸连接）
3. 单用户最大连接数限制
4. 连接来源验证（Origin 检查）

---

## 📝 代码质量改进

### 1. 错误处理标准化
**优先级：低** 🟢

**建议**：
1. 统一错误码定义
2. 使用自定义错误类型
3. 错误日志分级（Error/Warn/Info）
4. 敏感信息脱敏

---

### 2. 单元测试补充
**优先级：低** 🟢

**需要测试的模块**：
1. 消息发送逻辑
2. 权限验证逻辑
3. 数据库操作
4. Redis Stream 操作
5. WebSocket 连接管理

---

### 3. API 文档完善
**优先级：低** 🟢

**建议使用**：
- Swagger/OpenAPI 自动生成 API 文档
- 添加接口使用示例
- 添加错误码说明

---

## 🎯 优先级总结

### 立即处理（高优先级 🔴）
1. ✅ 消息已读逻辑优化（已完成）
2. ✅ 数据库表结构修复（已完成）
3. ✅ 消息推送通知机制完善（已完成）
4. ⚠️ 富媒体消息支持
5. ⚠️ 输入验证和权限控制加强

### 近期处理（中优先级 🟡）
1. ✅ 群组管理功能补充（已完成）
2. ✅ 群加入请求功能实现（已完成）
3. ✅ 用户/群组搜索功能（已完成）
4. 离线消息处理流程明确
5. 消息撤回功能
6. Redis Stream 清理机制
7. 速率限制

### 后期优化（低优先级 🟢）
1. 消息内容搜索功能
2. @提及功能
3. 消息转发功能
4. ✅ 会话置顶/删除（已完成，免打扰待实现）
5. 缓存策略优化
6. 单元测试补充
7. API 文档完善

---

## 📞 联系与反馈

如有问题或建议，请提交 Issue 或 Pull Request。

**最后更新**：2025年12月14日
