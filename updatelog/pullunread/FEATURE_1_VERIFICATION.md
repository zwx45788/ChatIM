# ✅ 功能 1 验证清单

## 📋 代码验证

### 1. 数据库脚本验证

**文件**: `d:\\git-demo\\ChatIM\\init.sql`

**检查项**:
- [ ] `messages` 表包含 `is_read BOOLEAN DEFAULT FALSE` 字段
- [ ] `messages` 表包含 `read_at TIMESTAMP NULL` 字段  
- [ ] 存在索引 `idx_to_user_read (to_user_id, is_read)`

**验证命令**:
```bash
docker exec chatim-db mysql -u root -p -e "DESC chatim.messages;"
```

---

### 2. Proto 定义验证

**文件**: `d:\\git-demo\\ChatIM\\api\\proto\\message.proto`

**检查项**:
- [ ] `Message` 消息体有 `bool is_read = 6` 字段
- [ ] `Message` 消息体有 `int64 read_at = 7` 字段
- [ ] 存在 `MarkMessagesAsReadRequest` 消息类型
- [ ] 存在 `MarkMessagesAsReadResponse` 消息类型
- [ ] 存在 `GetUnreadCountRequest` 消息类型
- [ ] 存在 `GetUnreadCountResponse` 消息类型
- [ ] `MessageService` 有 `rpc MarkMessagesAsRead` 方法
- [ ] `MessageService` 有 `rpc GetUnreadCount` 方法
- [ ] `go_package` 正确设置为 `"ChatIM/api/proto/message"`

**验证命令**:
```bash
grep -E "is_read|read_at|MarkMessagesAsRead|GetUnreadCount" api/proto/message.proto
```

---

### 3. gRPC 代码生成验证

**文件**: 
- `d:\\git-demo\\ChatIM\\api\\proto\\message\\message.pb.go`
- `d:\\git-demo\\ChatIM\\api\\proto\\message\\message_grpc.pb.go`

**检查项**:
- [ ] `message.pb.go` 文件存在
- [ ] `message_grpc.pb.go` 文件存在
- [ ] 文件大小 > 0（正常生成）

**验证命令**:
```bash
ls -lh api/proto/message/
# 应该显示:
# -rw-r--r-- ... message.pb.go
# -rw-r--r-- ... message_grpc.pb.go
```

---

### 4. 消息服务实现验证

**文件**: `d:\\git-demo\\ChatIM\\internal\\message_service\\handler\\message.go`

**检查项**:
- [ ] 包含函数 `func (h *MessageHandler) MarkMessagesAsRead(...)`
- [ ] 包含函数 `func (h *MessageHandler) GetUnreadCount(...)`
- [ ] `PullMessages` 方法查询包括 `is_read, read_at` 字段
- [ ] `PullMessages` 方法扫描包括 `&msg.IsRead, &readAtStr` 变量
- [ ] 没有编译错误

**验证命令**:
```bash
cd internal/message_service/handler
go build -o test.exe
# 应该编译成功，no errors
```

---

### 5. API Gateway 处理器验证

**文件**: `d:\\git-demo\\ChatIM\\internal\\api_gateway\\handler\\handler.go`

**检查项**:
- [ ] 包含函数 `func (h *UserGatewayHandler) MarkMessagesAsRead(...)`
- [ ] 包含函数 `func (h *UserGatewayHandler) GetUnreadCount(...)`
- [ ] 两个函数都使用 `metadata.New` 传递 Authorization
- [ ] 两个函数都调用 `h.messageClient` 的对应 gRPC 方法
- [ ] 正确处理 HTTP 响应状态码

**验证命令**:
```bash
grep -n "func (h \*UserGatewayHandler) MarkMessagesAsRead\|func (h \*UserGatewayHandler) GetUnreadCount" internal/api_gateway/handler/handler.go
# 应该显示两行，表示两个函数都存在
```

---

### 6. API 路由验证

**文件**: `d:\\git-demo\\ChatIM\\cmd\\api\\main.go`

**检查项**:
- [ ] `protected.POST("/messages/read", userHandler.MarkMessagesAsRead)` 存在
- [ ] `protected.GET("/messages/unread", userHandler.GetUnreadCount)` 存在
- [ ] 两个路由都在 `protected` 组中（有认证中间件）

**验证命令**:
```bash
grep "/messages/read\|/messages/unread" cmd/api/main.go
# 应该显示两行
```

---

## 🔧 编译验证

### 步骤 1: Proto 代码生成

```bash
cd d:\git-demo\ChatIM\api\proto
protoc --go_out=./message --go_opt=paths=source_relative \
       --go-grpc_out=./message --go-grpc_opt=paths=source_relative \
       message.proto
```

**预期输出**:
```
# 无错误输出，生成以下文件:
# - message/message.pb.go (大小 > 10KB)
# - message/message_grpc.pb.go (大小 > 5KB)
```

### 步骤 2: 消息服务编译

```bash
cd d:\git-demo\ChatIM\internal\message_service
go build cmd/message/main.go
```

**预期输出**:
```
# 无错误
# 生成 main.exe (如果在 Windows)
```

### 步骤 3: API Gateway 编译

```bash
cd d:\git-demo\ChatIM\cmd\api
go build -o api-gateway.exe main.go
```

**预期输出**:
```
# 无错误
# 生成 api-gateway.exe
```

---

## 🧪 功能测试

### 前置条件

```bash
# 1. 启动 Docker
docker-compose down -v
docker-compose up -d

# 2. 等待容器启动
sleep 30

# 3. 验证容器状态
docker ps
# 应该显示 5 个 running 容器:
# - chatim-db (MySQL)
# - chatim-redis (Redis)
# - chatim-user-service
# - chatim-message-service
# - chatim-api-gateway
```

### 测试 1: 用户登录

```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "password": "password123"
  }'
```

**预期响应**:
```json
{
  "code": 0,
  "message": "登录成功",
  "token": "eyJ..."
}
```

**验证项**:
- [ ] HTTP 状态码 200
- [ ] 返回 token
- [ ] 保存 token 供后续测试使用

---

### 测试 2: 获取未读消息数

```bash
curl -X GET http://localhost:8080/api/v1/messages/unread \
  -H "Authorization: Bearer <你的token>"
```

**预期响应**:
```json
{
  "code": 0,
  "message": "查询成功",
  "unread_count": 0
}
```

**验证项**:
- [ ] HTTP 状态码 200
- [ ] 返回 unread_count 字段
- [ ] 值为非负整数

---

### 测试 3: 拉取消息（检查新字段）

```bash
curl -X GET "http://localhost:8080/api/v1/messages?limit=5" \
  -H "Authorization: Bearer <你的token>" | jq
```

**预期响应**:
```json
{
  "code": 0,
  "message": "消息拉取成功",
  "msgs": [
    {
      "id": "msg-uuid",
      "from_user_id": "sender-id",
      "to_user_id": "receiver-id",
      "content": "Hello",
      "created_at": 1234567890,
      "is_read": false,
      "read_at": 0
    }
  ]
}
```

**验证项**:
- [ ] HTTP 状态码 200
- [ ] msgs 数组非空（如果有消息）
- [ ] 每条消息包含 `is_read` 字段
- [ ] 每条消息包含 `read_at` 字段
- [ ] is_read 值为 true 或 false
- [ ] read_at 值为整数或 0

---

### 测试 4: 标记消息为已读

```bash
# 首先获取消息 ID
MSG_IDS=$(curl -s "http://localhost:8080/api/v1/messages?limit=1" \
  -H "Authorization: Bearer <你的token>" | jq -r '.msgs[0].id')

# 标记为已读
curl -X POST http://localhost:8080/api/v1/messages/read \
  -H "Authorization: Bearer <你的token>" \
  -H "Content-Type: application/json" \
  -d "{
    \"message_ids\": [\"$MSG_IDS\"]
  }"
```

**预期响应**:
```json
{
  "code": 0,
  "message": "消息已标记为已读",
  "marked_count": 1
}
```

**验证项**:
- [ ] HTTP 状态码 200
- [ ] marked_count 为 1（或实际标记的数量）

---

### 测试 5: 验证标记后的状态

```bash
# 拉取刚才标记的消息
curl -X GET "http://localhost:8080/api/v1/messages?limit=1" \
  -H "Authorization: Bearer <你的token>" | jq '.msgs[0] | {id, is_read, read_at}'
```

**预期响应**:
```json
{
  "id": "msg-uuid",
  "is_read": true,
  "read_at": 1701939600  // 当前 Unix 时间戳
}
```

**验证项**:
- [ ] is_read 现在是 true
- [ ] read_at 现在是有效的时间戳（> 0）

---

## 📊 性能测试

### 测试场景 1: 快速计数

```bash
# 测试 GetUnreadCount 的性能
time curl -X GET http://localhost:8080/api/v1/messages/unread \
  -H "Authorization: Bearer <token>" > /dev/null
```

**预期结果**:
- 响应时间 < 50ms
- 内存占用稳定

---

### 测试场景 2: 批量标记

```bash
# 模拟标记 100 条消息
MSG_IDS=$(for i in {1..100}; do echo "\"msg-$i\""; done | tr '\n' ',' | sed 's/,$//')

time curl -X POST http://localhost:8080/api/v1/messages/read \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d "{\"message_ids\": [$MSG_IDS]}" > /dev/null
```

**预期结果**:
- 响应时间 < 200ms
- 返回 marked_count = 100

---

## 🐛 故障排查

### 问题 1: 编译错误 "undefined: pb.MarkMessagesAsReadRequest"

**原因**: Proto 代码未生成或生成失败

**解决方案**:
```bash
# 重新生成 Proto 代码
cd api/proto
rm -rf message/*.pb.go
protoc --go_out=./message --go_opt=paths=source_relative \
       --go-grpc_out=./message --go-grpc_opt=paths=source_relative \
       message.proto
```

---

### 问题 2: 数据库错误 "table messages doesn't exist"

**原因**: init.sql 未执行

**解决方案**:
```bash
docker-compose down -v
docker-compose up -d
sleep 30
```

---

### 问题 3: HTTP 404 错误访问新端点

**原因**: API Gateway 路由未配置

**解决方案**:
1. 检查 `cmd/api/main.go` 中是否有路由定义
2. 重启 API Gateway 容器
3. 检查日志：`docker logs chatim-api-gateway`

---

### 问题 4: 返回 "message_ids is empty"

**原因**: 请求体中 message_ids 为空数组

**解决方案**:
```bash
# 正确的请求:
curl -X POST http://localhost:8080/api/v1/messages/read \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"message_ids": ["msg-1", "msg-2"]}'
```

---

## 📝 最终检查清单

部署前的最后验证：

### 代码部分
- [ ] `init.sql` 包含新的字段和索引
- [ ] `message.proto` 包含新的消息和 RPC 方法
- [ ] Proto 代码已生成（check `message.pb.go` 文件）
- [ ] `message.go` 实现了两个新方法
- [ ] `handler.go` 包含两个新的 HTTP 处理函数
- [ ] `main.go` 配置了两个新的路由
- [ ] 所有文件编译无错误

### 部署部分
- [ ] Docker 容器已启动
- [ ] MySQL 数据库初始化完成
- [ ] 所有 5 个服务容器 running
- [ ] 数据库表包含新字段

### 测试部分
- [ ] 用户能成功登录
- [ ] 能查询未读消息数（成功率 > 90%）
- [ ] 能查看消息列表中的新字段
- [ ] 能标记消息为已读
- [ ] 标记后消息状态更新正确
- [ ] 性能满足 < 100ms 要求

---

## 🎉 完成标志

当所有检查项都打勾后，功能 1 就部署成功了！

```
✅ 代码实现完成
✅ Proto 定义完成
✅ gRPC 服务实现完成
✅ API Gateway 集成完成
✅ 数据库架构更新完成
✅ 编译测试通过
✅ 功能测试通过
✅ 性能测试通过

🎉 功能 1：已读确认 - 完全就绪！
```

---

## 📚 后续步骤

- [ ] 提交代码到 Git (git commit + git push)
- [ ] 通知团队新功能已部署
- [ ] 开始功能 2 的开发（多媒体消息）
- [ ] 考虑添加 Redis 缓存优化
- [ ] 监控生产环境性能
