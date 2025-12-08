# 功能 1 实现总结 - 已读确认

## 📋 修改清单

### 1. 数据库 (`init.sql`)
**修改位置**: `messages` 表定义

**之前**:
```sql
CREATE TABLE IF NOT EXISTS `messages` (
  `id` VARCHAR(36) PRIMARY KEY,
  `from_user_id` VARCHAR(36) NOT NULL,
  `to_user_id` VARCHAR(36) NOT NULL,
  `content` TEXT,
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (from_user_id) REFERENCES users(id),
  FOREIGN KEY (to_user_id) REFERENCES users(id),
  INDEX idx_from_user (from_user_id),
  INDEX idx_to_user (to_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**之后**:
```sql
CREATE TABLE IF NOT EXISTS `messages` (
  `id` VARCHAR(36) PRIMARY KEY,
  `from_user_id` VARCHAR(36) NOT NULL,
  `to_user_id` VARCHAR(36) NOT NULL,
  `content` TEXT,
  `is_read` BOOLEAN DEFAULT FALSE,        -- 新增
  `read_at` TIMESTAMP NULL DEFAULT NULL,  -- 新增
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (from_user_id) REFERENCES users(id),
  FOREIGN KEY (to_user_id) REFERENCES users(id),
  INDEX idx_from_user (from_user_id),
  INDEX idx_to_user (to_user_id),
  INDEX idx_to_user_read (to_user_id, is_read)  -- 新增
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

---

### 2. Proto 定义 (`api/proto/message.proto`)

**修改 1**: 更新 `Message` 结构
```protobuf
message Message {
  string id = 1;
  string from_user_id = 2;
  string to_user_id = 3;
  string content = 4;
  int64 created_at = 5;
  bool is_read = 6;        -- 新增
  int64 read_at = 7;       -- 新增
}
```

**修改 2**: 新增请求/响应消息和 RPC 方法

```protobuf
message MarkMessagesAsReadRequest {
  repeated string message_ids = 1;
}

message MarkMessagesAsReadResponse {
  int32 code = 1;
  string message = 2;
  int32 marked_count = 3;
}

message GetUnreadCountRequest {
}

message GetUnreadCountResponse {
  int32 code = 1;
  string message = 2;
  int32 unread_count = 3;
}

service MessageService {
  rpc SendMessage (SendMessageRequest) returns (SendMessageResponse);
  rpc PullMessages (PullMessagesRequest) returns (PullMessagesResponse);
  rpc MarkMessagesAsRead (MarkMessagesAsReadRequest) returns (MarkMessagesAsReadResponse);  -- 新增
  rpc GetUnreadCount (GetUnreadCountRequest) returns (GetUnreadCountResponse);             -- 新增
}
```

**修改 3**: 修复 `go_package`
```protobuf
option go_package = "ChatIM/api/proto/message";  -- 从 "github.com/your-username/..." 改为相对路径
```

---

### 3. 消息服务处理器 (`internal/message_service/handler/message.go`)

**修改 1**: 更新 `PullMessages` 方法
```go
// 之前的 SELECT 语句:
// SELECT id, from_user_id, to_user_id, content, created_at

// 修改后:
SELECT id, from_user_id, to_user_id, content, is_read, read_at, created_at

// 扫描时添加新字段:
rows.Scan(&msg.Id, &msg.FromUserId, &msg.ToUserId, 
          &msg.Content, &msg.IsRead, &readAtStr, &createdAtStr)
```

**修改 2**: 新增 `MarkMessagesAsRead` 方法 (~55 行)

```go
func (h *MessageHandler) MarkMessagesAsRead(ctx context.Context, 
    req *pb.MarkMessagesAsReadRequest) (*pb.MarkMessagesAsReadResponse, error) {
  // 获取用户ID
  userID, err := auth.GetUserID(ctx)
  
  // 构建批量 UPDATE 查询
  query := `UPDATE messages SET is_read = TRUE, read_at = ? 
            WHERE to_user_id = ? AND id IN (...)`
  
  // 执行更新
  result, err := h.db.ExecContext(ctx, query, ...)
  
  // 返回受影响行数
  rowsAffected, _ := result.RowsAffected()
  return &pb.MarkMessagesAsReadResponse{
    Code: 0,
    Message: "消息已标记为已读",
    MarkedCount: int32(rowsAffected),
  }, nil
}
```

**修改 3**: 新增 `GetUnreadCount` 方法 (~40 行)

```go
func (h *MessageHandler) GetUnreadCount(ctx context.Context, 
    req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
  // 获取用户ID
  userID, err := auth.GetUserID(ctx)
  
  // 查询未读消息数
  query := `SELECT COUNT(*) FROM messages 
            WHERE to_user_id = ? AND is_read = FALSE`
  
  var unreadCount int32
  err = h.db.QueryRowContext(ctx, query, userID).Scan(&unreadCount)
  
  return &pb.GetUnreadCountResponse{
    Code: 0,
    Message: "查询成功",
    UnreadCount: unreadCount,
  }, nil
}
```

---

### 4. API Gateway 处理器 (`internal/api_gateway/handler/handler.go`)

**新增**: 两个 HTTP 处理方法 (~110 行)

```go
// MarkMessagesAsRead - POST /api/v1/messages/read
func (h *UserGatewayHandler) MarkMessagesAsRead(c *gin.Context) {
  var req msgPb.MarkMessagesAsReadRequest
  if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    return
  }
  
  // 提取 token 并转发到 gRPC
  authHeader := c.GetHeader("Authorization")
  md := metadata.New(map[string]string{"authorization": authHeader})
  ctx := metadata.NewOutgoingContext(c.Request.Context(), md)
  
  res, err := h.messageClient.MarkMessagesAsRead(ctx, &req)
  // ... 返回响应
}

// GetUnreadCount - GET /api/v1/messages/unread
func (h *UserGatewayHandler) GetUnreadCount(c *gin.Context) {
  authHeader := c.GetHeader("Authorization")
  md := metadata.New(map[string]string{"authorization": authHeader})
  ctx := metadata.NewOutgoingContext(c.Request.Context(), md)
  
  res, err := h.messageClient.GetUnreadCount(ctx, &msgPb.GetUnreadCountRequest{})
  // ... 返回响应
}
```

---

### 5. API Gateway 路由 (`cmd/api/main.go`)

**修改位置**: `protected` 路由组中

**之前**:
```go
protected := api.Group("/")
protected.Use(middleware.AuthMiddleware())
{
  protected.GET("/users/me", userHandler.GetCurrentUser)
  protected.POST("/messages/send", userHandler.SendMessage)
  protected.GET("/messages", userHandler.PullMessage)
}
```

**之后**:
```go
protected := api.Group("/")
protected.Use(middleware.AuthMiddleware())
{
  protected.GET("/users/me", userHandler.GetCurrentUser)
  protected.POST("/messages/send", userHandler.SendMessage)
  protected.GET("/messages", userHandler.PullMessage)
  protected.POST("/messages/read", userHandler.MarkMessagesAsRead)      -- 新增
  protected.GET("/messages/unread", userHandler.GetUnreadCount)         -- 新增
}
```

---

## 📊 代码变更统计

| 文件 | 新增行数 | 修改行数 | 删除行数 | 说明 |
|------|--------|--------|--------|------|
| `init.sql` | 2 | 1 | 0 | 添加 2 个字段，1 个索引 |
| `api/proto/message.proto` | 20 | 1 | 0 | 新增 4 个消息类型，2 个 RPC 方法 |
| `internal/message_service/handler/message.go` | 95 | 15 | 0 | 新增 2 个方法，更新 PullMessages |
| `internal/api_gateway/handler/handler.go` | 57 | 0 | 0 | 新增 2 个 HTTP 处理函数 |
| `cmd/api/main.go` | 2 | 0 | 0 | 新增 2 个路由 |
| **总计** | **176** | **17** | **0** | 共 193 行变更 |

---

## 🔧 生成 Proto 代码

```bash
cd api/proto
protoc --go_out=./message --go_opt=paths=source_relative \
       --go-grpc_out=./message --go-grpc_opt=paths=source_relative \
       message.proto
```

**生成的文件**:
- `api/proto/message/message.pb.go`
- `api/proto/message/message_grpc.pb.go`

---

## 🧪 验证方法

### 编译验证
```bash
cd internal/message_service
go build -o message-service cmd/message/main.go
# 应该没有错误输出
```

### API 验证
```bash
# 1. 标记消息已读
curl -X POST http://localhost:8080/api/v1/messages/read \
  -H "Authorization: <token>" \
  -H "Content-Type: application/json" \
  -d '{"message_ids": ["msg-1", "msg-2"]}'

# 2. 查询未读消息数
curl -X GET http://localhost:8080/api/v1/messages/unread \
  -H "Authorization: <token>"
```

---

## 📚 文档

- 详细实现指南: `FEATURE_1_READ_CONFIRMATION.md`
- Proto 定义: `api/proto/message.proto`
- 完整代码: 各相关文件

---

## ✅ 完成状态

- [x] 数据库架构更新
- [x] Proto 定义和代码生成
- [x] gRPC 服务实现
- [x] API Gateway 集成
- [x] 路由配置
- [ ] Docker 容器重启（待执行）
- [ ] 集成测试（待执行）
- [ ] 前端集成（下一步）
