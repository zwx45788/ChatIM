# 功能 1 快速参考 - 已读确认

## 🎯 功能目标

用户可以标记消息为已读，系统可以统计未读消息数量。

## 📝 新增 API 端点

### 1️⃣ 标记消息已读
```
POST /api/v1/messages/read
Authorization: Bearer <token>
Content-Type: application/json

请求体:
{
  "message_ids": ["id1", "id2", "id3"]
}

响应:
{
  "code": 0,
  "message": "消息已标记为已读",
  "marked_count": 3
}
```

### 2️⃣ 获取未读消息数
```
GET /api/v1/messages/unread
Authorization: Bearer <token>

响应:
{
  "code": 0,
  "message": "查询成功",
  "unread_count": 5
}
```

### 3️⃣ 拉取消息（已更新）
```
GET /api/v1/messages?limit=20&offset=0
Authorization: Bearer <token>

响应包含额外字段:
{
  "msgs": [
    {
      "id": "msg-123",
      "from_user_id": "user-456",
      "to_user_id": "user-789",
      "content": "Hello!",
      "created_at": 1701939600,
      "is_read": false,      ✨ 新增
      "read_at": 0           ✨ 新增
    }
  ]
}
```

## 🛠️ 实现清单

- [x] 数据库: 添加 `is_read` 和 `read_at` 字段
- [x] 数据库: 添加复合索引 `idx_to_user_read`
- [x] Proto: 更新 `Message` 消息体
- [x] Proto: 新增 `MarkMessagesAsReadRequest/Response`
- [x] Proto: 新增 `GetUnreadCountRequest/Response`
- [x] Proto: 添加两个新 RPC 方法
- [x] gRPC: 实现 `MarkMessagesAsRead` 方法
- [x] gRPC: 实现 `GetUnreadCount` 方法
- [x] gRPC: 更新 `PullMessages` 方法返回已读字段
- [x] HTTP: 添加 `/messages/read` 路由
- [x] HTTP: 添加 `/messages/unread` 路由
- [x] Proto: 生成代码 (`protoc`)

## 📊 关键代码片段

### 数据库查询

**标记已读**:
```sql
UPDATE messages 
SET is_read = TRUE, read_at = NOW() 
WHERE to_user_id = ? AND id IN (?, ?, ...)
```

**获取未读数**:
```sql
SELECT COUNT(*) 
FROM messages 
WHERE to_user_id = ? AND is_read = FALSE
```

**拉取消息**:
```sql
SELECT id, from_user_id, to_user_id, content, 
       is_read, read_at, created_at
FROM messages
WHERE to_user_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?
```

### Go 代码

```go
// MarkMessagesAsRead - gRPC 方法
func (h *MessageHandler) MarkMessagesAsRead(ctx context.Context, 
    req *pb.MarkMessagesAsReadRequest) (*pb.MarkMessagesAsReadResponse, error) {
    
    userID, err := auth.GetUserID(ctx)
    // ... 批量更新逻辑
    return &pb.MarkMessagesAsReadResponse{
        Code: 0,
        MarkedCount: int32(rowsAffected),
    }, nil
}

// GetUnreadCount - gRPC 方法
func (h *MessageHandler) GetUnreadCount(ctx context.Context, 
    req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
    
    userID, err := auth.GetUserID(ctx)
    // ... 查询逻辑
    return &pb.GetUnreadCountResponse{
        Code: 0,
        UnreadCount: unreadCount,
    }, nil
}
```

## 🧪 测试用例

```bash
# 1. 登录
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"pwd"}' | jq -r '.token')

# 2. 查询未读消息数
curl -X GET http://localhost:8080/api/v1/messages/unread \
  -H "Authorization: $TOKEN" | jq

# 3. 拉取消息
curl -X GET "http://localhost:8080/api/v1/messages?limit=10" \
  -H "Authorization: $TOKEN" | jq '.msgs[] | {id, is_read, read_at}'

# 4. 标记消息为已读
curl -X POST http://localhost:8080/api/v1/messages/read \
  -H "Authorization: $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message_ids":["id1","id2"]}' | jq

# 5. 再次查询未读消息数（应该减少）
curl -X GET http://localhost:8080/api/v1/messages/unread \
  -H "Authorization: $TOKEN" | jq
```

## 📁 文件变更

| 文件 | 变更类型 | 行数 |
|------|--------|------|
| `init.sql` | 修改 | +2 字段, +1 索引 |
| `api/proto/message.proto` | 修改 | +4 消息, +2 RPC |
| `internal/message_service/handler/message.go` | 修改 | +95 行 |
| `internal/api_gateway/handler/handler.go` | 修改 | +57 行 |
| `cmd/api/main.go` | 修改 | +2 行 |

## ⏱️ 预计开发时间

- 数据库设计: 15 分钟 ✅
- Proto 定义: 10 分钟 ✅
- gRPC 实现: 30 分钟 ✅
- API Gateway: 15 分钟 ✅
- 测试验证: 15 分钟 ⏳
- **总计**: ~85 分钟

## 🚀 下一步行动

```bash
# 1. 重启容器应用数据库变更
docker-compose down -v
docker-compose up -d
sleep 30

# 2. 验证编译
cd internal/message_service && go build

# 3. 运行测试
curl -X GET http://localhost:8080/api/v1/messages/unread \
  -H "Authorization: <your-token>"
```

## 💡 性能优化建议

- 使用 Redis 缓存 `unread_count`（减少数据库查询）
- 使用消息队列异步更新已读状态
- 每天定时清理超过 30 天的已读消息
- 为频繁查询的字段添加数据库统计表

## 📚 相关文档

- 完整实现: `FEATURE_1_READ_CONFIRMATION.md`
- 变更摘要: `FEATURE_1_CHANGES_SUMMARY.md`
- Proto 定义: `api/proto/message.proto`
