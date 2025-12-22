# Zap 结构化日志使用指南

## 概述

本项目已从标准库 `log` 包迁移至 [Uber Zap](https://github.com/uber-go/zap) 结构化日志系统，提供高性能、类型安全的日志记录能力。

## 为什么使用 Zap？

### 优势

1. **高性能**：比标准库日志快 4-10 倍，零内存分配
2. **结构化**：日志字段结构化，便于解析和分析
3. **类型安全**：编译时类型检查，避免运行时错误
4. **灵活配置**：支持多种输出格式（JSON、Console）、日志级别、文件轮转
5. **生产就绪**：广泛应用于企业级项目

### 对比

```go
// 旧方式 (标准库 log)
log.Printf("User %s failed to send message: %v", userID, err)

// 新方式 (Zap)
logger.Error("User failed to send message",
    zap.String("user_id", userID),
    zap.Error(err))
```

## 快速开始

### 1. 初始化日志系统

在 `main.go` 中初始化：

```go
import "ChatIM/pkg/logger"

func main() {
    // 初始化日志（根据配置自动选择模式）
    logger.InitLogger(logger.Config{
        Mode:       "production",  // "development" 或 "production"
        Level:      "info",        // "debug", "info", "warn", "error"
        OutputPath: "logs/app.log",
        MaxSize:    100,           // MB
        MaxBackups: 3,
        MaxAge:     30,            // days
        Compress:   true,
    })
    defer logger.Sync() // 刷新缓冲区
}
```

### 2. 使用日志

#### 基本用法

```go
import (
    "ChatIM/pkg/logger"
    "go.uber.org/zap"
)

// Debug 级别 - 用于开发调试
logger.Debug("Debugging user query", zap.String("user_id", userID))

// Info 级别 - 正常流程信息
logger.Info("User login successful", 
    zap.String("user_id", userID),
    zap.String("ip", clientIP))

// Warn 级别 - 警告但不影响主流程
logger.Warn("Redis cache miss, using database",
    zap.String("key", cacheKey))

// Error 级别 - 错误需要关注
logger.Error("Failed to send message",
    zap.String("msg_id", msgID),
    zap.Error(err))

// Fatal 级别 - 致命错误，记录后程序退出
logger.Fatal("Failed to connect to database",
    zap.String("addr", dbAddr),
    zap.Error(err))
```

#### 结构化字段

Zap 提供强类型字段方法：

```go
// 字符串
zap.String("user_id", "12345")
zap.Strings("tags", []string{"admin", "active"})

// 数字
zap.Int("count", 100)
zap.Int32("unread_count", unreadCount)
zap.Int64("timestamp", time.Now().Unix())
zap.Float64("score", 98.5)

// 布尔值
zap.Bool("is_admin", true)

// 时间
zap.Time("created_at", time.Now())
zap.Duration("elapsed", time.Since(start))

// 错误
zap.Error(err)  // 特殊处理，键名为 "error"

// 任意类型（会使用 reflection，性能略低）
zap.Any("data", complexObject)

// 嵌套对象
zap.Object("user", userObject)  // 需要实现 zapcore.ObjectMarshaler

// 跳过堆栈帧（用于包装函数）
zap.Skip()
```

#### 格式化日志（Sugar Logger）

对于简单场景，可以使用 Sugar Logger（性能略低，但更方便）：

```go
import "ChatIM/pkg/logger"

// Printf 风格
logger.Debugf("User %s is sending message to %s", fromUser, toUser)
logger.Infof("Received %d messages", count)
logger.Warnf("Cache miss for key: %s", key)
logger.Errorf("Failed to process request: %v", err)

// 键值对风格
logger.Debugw("User login",
    "user_id", userID,
    "ip", clientIP,
    "device", deviceType)
```

## 最佳实践

### 1. 日志级别选择

| 级别  | 使用场景 | 示例 |
|-------|---------|------|
| Debug | 详细的调试信息，仅开发环境 | 请求参数、中间变量 |
| Info  | 重要的业务流程信息 | 用户登录、消息发送成功 |
| Warn  | 警告但不影响主流程 | 缓存未命中、降级处理 |
| Error | 需要关注的错误 | 数据库查询失败、第三方API错误 |
| Fatal | 致命错误，程序无法继续 | 配置文件缺失、核心服务连接失败 |

### 2. 字段命名规范

使用 **snake_case** 风格，保持一致性：

```go
// ✅ 推荐
logger.Info("User action",
    zap.String("user_id", userID),
    zap.String("action_type", "login"),
    zap.Int64("timestamp", timestamp))

// ❌ 避免
logger.Info("User action",
    zap.String("UserID", userID),
    zap.String("ActionType", "login"))
```

### 3. 常用字段名

| 字段名 | 含义 | 示例 |
|--------|------|------|
| `user_id` | 用户ID | "12345" |
| `msg_id` | 消息ID | "msg_abc123" |
| `group_id` | 群组ID | "group_xyz789" |
| `request_id` | 请求ID（链路追踪） | "req_001" |
| `error` | 错误对象 | err |
| `latency` | 延迟 | 150ms |
| `method` | HTTP方法 | "POST" |
| `path` | 请求路径 | "/api/v1/messages" |
| `status_code` | HTTP状态码 | 200 |
| `client_ip` | 客户端IP | "192.168.1.100" |

### 4. 错误日志记录

```go
// ❌ 避免：信息不足
logger.Error("Failed to send message", zap.Error(err))

// ✅ 推荐：包含上下文信息
logger.Error("Failed to send private message",
    zap.String("msg_id", msgID),
    zap.String("from_user_id", fromUserID),
    zap.String("to_user_id", toUserID),
    zap.String("content_preview", content[:50]),
    zap.Error(err))
```

### 5. 性能优化

```go
// 避免在日志语句中进行复杂计算
// ❌ 避免
logger.Debug("Processing users", zap.Int("count", len(fetchAllUsers())))

// ✅ 推荐
if logger.Core().Enabled(zap.DebugLevel) {
    users := fetchAllUsers()
    logger.Debug("Processing users", zap.Int("count", len(users)))
}

// 或使用延迟计算
logger.Debug("Processing users", zap.Lazy(func() zap.Field {
    return zap.Int("count", len(fetchAllUsers()))
}))
```

### 6. 避免敏感信息泄露

```go
// ❌ 避免记录敏感信息
logger.Info("User login", zap.String("password", password))

// ✅ 记录脱敏后的信息
logger.Info("User login",
    zap.String("user_id", userID),
    zap.String("ip", clientIP))

// 如果必须记录，使用脱敏
logger.Debug("User credential",
    zap.String("password_hash", hashPassword(password)[:10]+"..."))
```

## 配置详解

### 开发模式 vs 生产模式

**开发模式（Development）**:
- 输出格式：彩色 Console，易读
- 日志级别：默认 Debug
- 堆栈跟踪：Error 及以上级别
- 适用：本地开发、调试

**生产模式（Production）**:
- 输出格式：JSON，便于日志收集和分析
- 日志级别：默认 Info
- 堆栈跟踪：仅 Fatal 级别
- 适用：线上环境、日志聚合系统（ELK、Loki）

### 配置示例

```yaml
# config.yml
log:
  mode: production        # development | production
  level: info             # debug | info | warn | error
  output_path: logs/app.log
  max_size: 100           # MB
  max_backups: 3
  max_age: 30             # days
  compress: true
```

## 实际案例

### 案例 1: HTTP 请求日志

```go
func (h *MessageHandler) SendMessage(c *gin.Context) {
    start := time.Now()
    var req SendMessageRequest
    
    if err := c.ShouldBindJSON(&req); err != nil {
        logger.Warn("Invalid request format",
            zap.String("method", c.Request.Method),
            zap.String("path", c.Request.URL.Path),
            zap.Error(err))
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }
    
    userID, _ := middleware.GetUserIDFromContext(c)
    
    logger.Info("Processing send message request",
        zap.String("user_id", userID),
        zap.String("to_user_id", req.ToUserID),
        zap.String("content_type", req.ContentType))
    
    // ... 业务逻辑 ...
    
    logger.Info("Send message completed",
        zap.String("user_id", userID),
        zap.String("msg_id", msgID),
        zap.Duration("latency", time.Since(start)))
}
```

### 案例 2: 数据库操作日志

```go
func (r *MessageRepository) GetMessageByID(ctx context.Context, msgID string) (*Message, error) {
    start := time.Now()
    
    var msg Message
    err := r.db.QueryRowContext(ctx,
        "SELECT * FROM messages WHERE id = ?", msgID).Scan(&msg)
    
    if err != nil {
        if err == sql.ErrNoRows {
            logger.Warn("Message not found",
                zap.String("msg_id", msgID),
                zap.Duration("query_time", time.Since(start)))
            return nil, ErrNotFound
        }
        
        logger.Error("Database query failed",
            zap.String("msg_id", msgID),
            zap.Duration("query_time", time.Since(start)),
            zap.Error(err))
        return nil, err
    }
    
    logger.Debug("Message fetched successfully",
        zap.String("msg_id", msgID),
        zap.Duration("query_time", time.Since(start)))
    
    return &msg, nil
}
```

### 案例 3: Redis 操作日志

```go
func (s *StreamOperator) AddPrivateMessage(ctx context.Context, msgID, fromUserID, toUserID string, payload MessagePayload) error {
    start := time.Now()
    streamKey := fmt.Sprintf("stream:private:%s", toUserID)
    
    data, err := json.Marshal(payload)
    if err != nil {
        logger.Error("Failed to marshal message payload",
            zap.String("msg_id", msgID),
            zap.Error(err))
        return err
    }
    
    _, err = s.rdb.XAdd(ctx, &redis.XAddArgs{
        Stream: streamKey,
        Values: map[string]interface{}{
            "msg_id":       msgID,
            "from_user_id": fromUserID,
            "data":         data,
        },
    }).Result()
    
    if err != nil {
        logger.Error("Failed to add message to Redis Stream",
            zap.String("msg_id", msgID),
            zap.String("stream_key", streamKey),
            zap.Duration("latency", time.Since(start)),
            zap.Error(err))
        return err
    }
    
    logger.Info("Message added to stream",
        zap.String("msg_id", msgID),
        zap.String("stream_key", streamKey),
        zap.Duration("latency", time.Since(start)))
    
    return nil
}
```

### 案例 4: gRPC 服务日志

```go
func (s *UserService) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
    start := time.Now()
    
    logger.Info("GetUserInfo request received",
        zap.String("user_id", req.UserId),
        zap.String("request_id", getRequestID(ctx)))
    
    user, err := s.repo.FindUserByID(ctx, req.UserId)
    if err != nil {
        logger.Error("Failed to get user info",
            zap.String("user_id", req.UserId),
            zap.Duration("latency", time.Since(start)),
            zap.Error(err))
        return nil, status.Error(codes.NotFound, "User not found")
    }
    
    logger.Info("GetUserInfo request completed",
        zap.String("user_id", req.UserId),
        zap.Duration("latency", time.Since(start)))
    
    return &pb.GetUserInfoResponse{
        UserId:   user.ID,
        Username: user.Username,
        // ...
    }, nil
}
```

## 日志收集与分析

### 1. 日志格式

生产模式下，Zap 输出 JSON 格式日志，便于解析：

```json
{
  "level": "info",
  "ts": 1735000000.123456,
  "caller": "handler/message.go:123",
  "msg": "Message sent successfully",
  "msg_id": "msg_abc123",
  "from_user_id": "user_001",
  "to_user_id": "user_002",
  "latency": 0.15
}
```

### 2. ELK Stack 集成

使用 Filebeat 收集日志发送到 Elasticsearch：

```yaml
# filebeat.yml
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - /var/log/chatim/*.log
    json.keys_under_root: true
    json.add_error_key: true

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "chatim-%{+yyyy.MM.dd}"
```

### 3. Grafana Loki 集成

使用 Promtail 采集日志：

```yaml
# promtail.yml
clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: chatim
    static_configs:
      - targets:
          - localhost
        labels:
          job: chatim
          __path__: /var/log/chatim/*.log
    pipeline_stages:
      - json:
          expressions:
            level: level
            msg: msg
            user_id: user_id
```

### 4. 常用查询

**Elasticsearch (Kibana)**:
```
# 查询特定用户的错误日志
level: error AND user_id: "12345"

# 查询慢请求（延迟 > 1s）
latency: >1.0

# 查询 API 调用失败
method: POST AND status_code: [500 TO 599]
```

**Loki (LogQL)**:
```
# 查询错误日志
{job="chatim"} | json | level="error"

# 统计每分钟错误数
sum(rate({job="chatim"} | json | level="error"[1m]))

# 查询特定用户的日志
{job="chatim"} | json | user_id="12345"
```

## 迁移指南

如果你需要将旧代码从 `log` 包迁移到 Zap：

### 迁移步骤

1. **导入包**:
```go
// 删除
import "log"

// 添加
import (
    "ChatIM/pkg/logger"
    "go.uber.org/zap"
)
```

2. **替换基本调用**:
```go
// log.Println -> logger.Info
log.Println("User logged in")
logger.Info("User logged in")

// log.Printf -> logger.Infof (或使用结构化字段)
log.Printf("User %s logged in", userID)
logger.Infof("User %s logged in", userID)
// 或（推荐）
logger.Info("User logged in", zap.String("user_id", userID))

// log.Fatal -> logger.Fatal
log.Fatal("Failed to start server")
logger.Fatal("Failed to start server")
```

3. **添加结构化字段**:
```go
// 旧
log.Printf("User %s sent message %s to %s", fromUser, msgID, toUser)

// 新
logger.Info("Message sent",
    zap.String("from_user_id", fromUser),
    zap.String("msg_id", msgID),
    zap.String("to_user_id", toUser))
```

### 已迁移文件

- ✅ `cmd/api/main.go` - API Gateway 入口
- ✅ `internal/message_service/handler/message.go` - 消息服务处理器
- ✅ `internal/api_gateway/handler/conversation.go` - 会话处理器
- ✅ `internal/api_gateway/middleware/auth.go` - 认证中间件
- ✅ `pkg/profiling/profiling.go` - 性能分析
- ✅ `pkg/stream/operator.go` - Stream 操作
- ✅ `pkg/run.go` - 服务运行器

### 待迁移文件

- ⏳ `internal/group_service/handler/group.go`
- ⏳ `internal/friendship/repository/friendship_repository.go`
- ⏳ `internal/friendship/handler/*.go`

## 常见问题

### Q1: 如何在单元测试中使用日志？

```go
import (
    "ChatIM/pkg/logger"
    "go.uber.org/zap"
    "go.uber.org/zap/zaptest"
    "testing"
)

func TestSendMessage(t *testing.T) {
    // 使用 zaptest 创建测试专用 logger
    testLogger := zaptest.NewLogger(t)
    logger.SetLogger(testLogger)  // 需要在 logger 包中添加此方法
    
    // 测试代码...
}
```

### Q2: 如何动态调整日志级别？

```go
// 方式 1: 通过配置文件重新加载
logger.InitLogger(newConfig)

// 方式 2: 暴露 HTTP 接口调整（生产环境谨慎使用）
// GET /admin/log/level?level=debug
func SetLogLevel(level string) {
    logger.SetLevel(level)
}
```

### Q3: 日志文件太大怎么办？

配置文件轮转：

```go
logger.InitLogger(logger.Config{
    OutputPath: "logs/app.log",
    MaxSize:    100,  // 100MB 后轮转
    MaxBackups: 5,    // 保留 5 个备份
    MaxAge:     30,   // 30 天后删除
    Compress:   true, // 压缩旧文件
})
```

### Q4: 如何在日志中添加请求追踪ID？

使用上下文传递：

```go
// middleware
func RequestIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        requestID := uuid.New().String()
        c.Set("request_id", requestID)
        
        // 添加到日志上下文
        ctx := context.WithValue(c.Request.Context(), "request_id", requestID)
        c.Request = c.Request.WithContext(ctx)
        
        c.Next()
    }
}

// handler
func (h *Handler) Process(c *gin.Context) {
    requestID := c.GetString("request_id")
    logger.Info("Processing request",
        zap.String("request_id", requestID),
        // ...
    )
}
```

### Q5: 如何记录 panic 堆栈？

使用 recovery 中间件：

```go
func RecoveryMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                logger.Error("Panic recovered",
                    zap.Any("error", err),
                    zap.Stack("stack"))
                c.AbortWithStatus(500)
            }
        }()
        c.Next()
    }
}
```

## 参考资料

- [Zap 官方文档](https://pkg.go.dev/go.uber.org/zap)
- [Zap GitHub](https://github.com/uber-go/zap)
- [日志最佳实践](https://github.com/uber-go/guide/blob/master/style.md#logging)

---

**更新时间**: 2024-12-23  
**作者**: ChatIM Team
