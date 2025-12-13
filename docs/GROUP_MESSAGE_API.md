# 群聊消息 API 文档

## 概述
群聊消息功能已集成到消息服务和 API 网关。支持发送群聊消息、Redis Stream 实时推送和数据库持久化。

---

## API 接口

### 发送群聊消息

**端点**: `POST /api/v1/groups/messages`

**认证**: 需要 Bearer Token（通过 `Authorization` header）

**请求体**:
```json
{
  "group_id": "group_123",
  "content": "大家好！这是一条群聊消息"
}
```

**响应示例（成功）**:
```json
{
  "code": 0,
  "message": "群聊消息发送成功",
  "msg": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "group_id": "group_123",
    "from_user_id": "user_456",
    "content": "大家好！这是一条群聊消息",
    "created_at": 1702512000
  }
}
```

**响应示例（未授权）**:
```json
{
  "error": "Authorization header is required"
}
```

---

## 技术实现

### 后端架构

1. **API 网关** (`cmd/api/main.go`)
   - 路由: `POST /api/v1/groups/messages`
   - Handler: `userHandler.SendGroupMessage`
   - 中间件: `AuthMiddleware` (JWT 验证)

2. **消息服务** (`internal/message_service/handler/message.go`)
   - 方法: `SendGroupMessage(ctx, req)`
   - 步骤:
     1. 从 context 提取当前用户 ID
     2. 校验 `group_id` 参数
     3. 写入 Redis Stream (`stream:group:{group_id}`)
     4. 异步写入 MySQL `group_messages` 表
     5. 返回成功响应

3. **Redis Stream** (`pkg/stream/operator.go`)
   - 方法: `AddGroupMessage(ctx, msgID, groupID, fromUserID, content, msgType)`
   - Stream Key: `stream:group:{group_id}`
   - 字段: `id`, `group_id`, `from_user_id`, `content`, `msg_type`, `created_at`, `is_read`

4. **数据库持久化**
   - 表: `group_messages`
   - 字段: `id`, `group_id`, `from_user_id`, `content`, `created_at`
   - 执行: 异步（5秒超时）

---

## 使用示例

### cURL 示例

```bash
# 1. 登录获取 token
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "password": "password123"
  }'

# 响应: { "token": "eyJhbGciOiJIUzI1NiIs..." }

# 2. 发送群聊消息
curl -X POST http://localhost:8080/api/v1/groups/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -d '{
    "group_id": "group_123",
    "content": "Hello everyone in the group!"
  }'
```

### JavaScript 示例

```javascript
// 发送群聊消息
async function sendGroupMessage(token, groupId, content) {
  const response = await fetch('http://localhost:8080/api/v1/groups/messages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({
      group_id: groupId,
      content: content
    })
  });

  const result = await response.json();
  console.log('Message sent:', result);
  return result;
}

// 使用示例
const token = 'eyJhbGciOiJIUzI1NiIs...';
sendGroupMessage(token, 'group_123', 'Hello everyone!')
  .then(res => console.log('Success:', res.msg))
  .catch(err => console.error('Error:', err));
```

---

## Proto 定义

### 消息类型

```protobuf
// 群聊消息数据结构
message GroupMessage {
  string id = 1;          // 消息唯一ID
  string group_id = 2;    // 群组ID
  string from_user_id = 3; // 发送者ID
  string content = 4;      // 消息内容
  int64 created_at = 5;    // 创建时间
}

// 发送群聊消息的请求
message SendGroupMessageRequest {
  string group_id = 1;  // 群组ID
  string content = 2;   // 消息内容
}

// 发送群聊消息的响应
message SendGroupMessageResponse {
  int32 code = 1;
  string message = 2;
  GroupMessage msg = 3; // 返回成功存储的群聊消息详情
}
```

### 服务定义

```protobuf
service MessageService {
  // 发送一条群聊消息
  rpc SendGroupMessage (SendGroupMessageRequest) returns (SendGroupMessageResponse);
  // ... 其他 RPC 方法
}
```

---

## 数据库 Schema

### group_messages 表

```sql
CREATE TABLE group_messages (
    id VARCHAR(36) PRIMARY KEY,
    group_id VARCHAR(36) NOT NULL,
    from_user_id VARCHAR(36) NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    INDEX idx_group_created (group_id, created_at),
    INDEX idx_from_user (from_user_id)
);
```

---

## 错误处理

| HTTP 状态码 | 错误场景 | 响应示例 |
|------------|---------|---------|
| 400 | 请求体格式错误 | `{"error": "Invalid request body"}` |
| 401 | 缺少或无效 Token | `{"error": "Authorization header is required"}` |
| 400 | 缺少 group_id | `{"code": 3, "message": "group_id is required"}` |
| 500 | Redis Stream 写入失败 | `{"error": "Failed to save group message"}` |

---

## 测试清单

- [x] Proto 定义已添加 (GroupMessage, SendGroupMessageRequest/Response)
- [x] Proto 代码已重新生成 (`build.bat`)
- [x] 消息服务实现 SendGroupMessage 方法
- [x] API 网关添加 handler (`SendGroupMessage`)
- [x] API 网关注册路由 (`POST /api/v1/groups/messages`)
- [x] 代码编译通过 (`go build ./...`)
- [ ] 单元测试（待添加）
- [ ] 集成测试（待添加）
- [ ] WebSocket 实时推送集成（可选）

---

## 下一步扩展

1. **拉取群聊消息历史**
   - 接口: `GET /api/v1/groups/:group_id/messages`
   - 支持分页、时间范围过滤

2. **群聊未读消息数**
   - 接口: `GET /api/v1/groups/:group_id/unread`
   - 基于 `group_read_states` 表

3. **@提及功能**
   - 消息内容解析 `@username`
   - 单独推送被提及的用户

4. **消息撤回**
   - 接口: `DELETE /api/v1/groups/messages/:message_id`
   - 限制撤回时间窗口（如2分钟内）

5. **富文本消息**
   - 支持图片、文件、表情等类型
   - 扩展 `msg_type` 字段（text/image/file/emoji）

---

## 相关文档

- [Message Service 实现](../internal/message_service/handler/message.go)
- [API Gateway Handler](../internal/api_gateway/handler/handler.go)
- [Redis Stream Operator](../pkg/stream/operator.go)
- [Proto 定义](../api/proto/message.proto)
