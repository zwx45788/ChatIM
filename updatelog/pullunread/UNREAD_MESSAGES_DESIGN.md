# 未读消息处理设计方案

## 📱 主流聊天软件的最佳实践分析

### 微信方案分析
```
用户上线流程：
1. 用户登录 → 返回 token
2. 前端调用 `/messages/unread` → 获取未读消息数
3. 前端调用 `/messages/pull?unread=true` → 拉取所有未读消息
4. 自动标记所有已读消息为已读
5. 显示红点数字提示（基于未读数）

特点：
✅ 分离关注点（先知道有多少，再拉取）
✅ 用户可以控制加载时机
✅ 减少首屏加载压力
✅ 支持增量同步
```

### QQ 方案分析
```
用户上线流程：
1. 用户登录 → 返回 token + 离线消息统计
2. 直接推送离线消息到前端（如果数量 < 100）
3. 如果离线消息 > 100，提示用户并按需加载
4. 前端滚动到底时继续加载历史消息

特点：
✅ 主动推送（登录返回离线消息统计）
✅ 智能分页（大量离线消息时分页）
✅ 渐进式加载（不一次性加载所有）
✅ 网络优化
```

### Telegram 方案分析
```
用户上线流程：
1. 用户登录 → 返回 token
2. 使用 WebSocket/长轮询建立连接
3. 服务器推送新消息和未读状态
4. 前端渲染并自动标记已读
5. 支持从某个点开始同步（断线重连）

特点：
✅ 实时推送（WebSocket）
✅ 双向通信
✅ 断线重连机制
✅ 消息去重
```

### WhatsApp 方案分析
```
用户上线流程：
1. 用户登录 → 返回 token + 同步令牌
2. 基于同步令牌拉取增量消息
3. 本地 SQLite 数据库存储
4. 自动标记为已读并上报服务器

特点：
✅ 增量同步（基于同步令牌）
✅ 本地优先（离线也能查看）
✅ 可靠性高（多次确认机制）
✅ 隐私保护
```

---

## 🎯 我的建议：分阶段实现

### 第一阶段（现在）- 基础方案（类似微信模式）

**最简单、最可靠，适合现阶段：**

#### 1️⃣ 新增 Proto 方法
```protobuf
message PullUnreadMessagesRequest {
  int64 limit = 1;        // 单次拉取上限（建议 100）
  bool auto_mark = 2;     // 是否自动标记为已读
}

message PullUnreadMessagesResponse {
  int32 code = 1;
  string message = 2;
  repeated Message msgs = 3;
  int32 total_unread = 4; // 拉取前的总未读数
}

service MessageService {
  rpc PullUnreadMessages (PullUnreadMessagesRequest) returns (PullUnreadMessagesResponse);
}
```

#### 2️⃣ 后端实现逻辑
```go
func (h *MessageHandler) PullUnreadMessages(ctx context.Context, 
    req *pb.PullUnreadMessagesRequest) (*pb.PullUnreadMessagesResponse, error) {
    
    userID, _ := auth.GetUserID(ctx)
    
    // 1. 先查询总未读数
    countRes, _ := h.GetUnreadCount(ctx, &pb.GetUnreadCountRequest{})
    totalUnread := countRes.UnreadCount
    
    // 2. 查询未读消息列表
    query := `SELECT id, from_user_id, to_user_id, content, is_read, read_at, created_at
              FROM messages 
              WHERE to_user_id = ? AND is_read = FALSE
              ORDER BY created_at DESC
              LIMIT ?`
    rows, _ := h.db.QueryContext(ctx, query, userID, req.Limit)
    
    // 3. 如果 auto_mark 为 true，自动标记为已读
    if req.AutoMark && len(msgs) > 0 {
        msgIDs := extractIDs(msgs)
        h.MarkMessagesAsRead(ctx, &pb.MarkMessagesAsReadRequest{
            MessageIds: msgIDs,
        })
    }
    
    // 4. 返回消息和统计信息
    return &pb.PullUnreadMessagesResponse{
        Code: 0,
        Msgs: msgs,
        TotalUnread: totalUnread,
    }, nil
}
```

#### 3️⃣ API 端点
```
GET /api/v1/messages/unread/pull?limit=100&auto_mark=true
Authorization: Bearer <token>

响应:
{
  "code": 0,
  "message": "成功",
  "msgs": [...],
  "total_unread": 5
}
```

#### 4️⃣ 前端使用流程
```javascript
// 用户登录后
1. 显示加载动画
2. 调用 GET /api/v1/messages/unread/pull?limit=100&auto_mark=true
3. 获取消息列表并自动标记为已读
4. 渲染消息到界面
5. 隐藏加载动画

优点：
✅ 简单，一次调用解决问题
✅ 自动标记，用户无需额外操作
✅ 支持分页（limit 参数）
```

---

### 第二阶段（下周）- 优化方案（类似 QQ）

**添加离线消息统计到登录响应：**

```protobuf
message LoginResponse {
  int32 code = 1;
  string message = 2;
  string token = 3;
  int32 unread_count = 4;      // ✨ 新增
  int64 last_read_timestamp = 5; // ✨ 新增
}
```

```go
func (h *UserHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
    // ... 验证用户名密码 ...
    
    // 查询未读消息数
    unreadCount, _ := h.getUnreadCount(userID)
    
    // 查询最后已读消息的时间戳
    lastReadTime, _ := h.getLastReadTime(userID)
    
    return &pb.LoginResponse{
        Code: 0,
        Token: tokenString,
        UnreadCount: unreadCount,      // ✨ 直接返回
        LastReadTimestamp: lastReadTime,
    }, nil
}
```

**前端优化：**
```javascript
// 用户登录后
1. 获取响应中的 unread_count
2. 如果 unread_count > 0，显示红点和数字
3. 用户点击时再调用 /messages/unread/pull
4. 不强制加载（用户控制）

优点：
✅ 减少启动压力（不立即加载所有消息）
✅ 用户知道有多少未读
✅ 可以先处理其他事情再看消息
```

---

### 第三阶段（本月）- 实时推送（类似 Telegram）

**使用 WebSocket 推送新消息：**

```go
// WebSocket 消息类型
type WSMessage struct {
    Type string                 // "new_message" | "user_online" | "user_typing"
    Data interface{}
}

// 新消息事件
type NewMessageEvent struct {
    MessageID string
    FromUserID string
    Content string
    Timestamp int64
}

func (h *MessageHandler) SendMessage(...) {
    // ... 保存消息到数据库 ...
    
    // ✨ 推送给接收者（如果在线）
    event := NewMessageEvent{...}
    h.publishToUser(req.ToUserId, event)
}
```

**前端实时处理：**
```javascript
// 建立 WebSocket 连接
ws = new WebSocket('ws://localhost:8080/ws?token=xxx')

ws.onmessage = (event) => {
    const message = JSON.parse(event.data)
    
    if (message.type === 'new_message') {
        // 实时显示新消息
        addMessageToUI(message.data)
        // 自动标记为已读
        markAsRead([message.data.id])
    }
}
```

---

## 📊 方案对比

| 方案 | 实现难度 | 用户体验 | 实时性 | 建议使用时间 |
|------|--------|--------|-------|-----------|
| **第一阶段** (Pull模式) | ⭐ 简单 | ⭐⭐⭐ | ⭐⭐ 秒级 | 现在 |
| **第二阶段** (登录推送) | ⭐⭐ 中等 | ⭐⭐⭐⭐ | ⭐⭐ 秒级 | 1-2周后 |
| **第三阶段** (WebSocket) | ⭐⭐⭐ 复杂 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ 毫秒级 | 1个月后 |

---

## 🎯 我的强烈推荐：先做第一阶段

### 原因：

1. **开发成本小** - 只需添加一个方法，2-3 小时完成
2. **测试简单** - 易于验证和调试
3. **用户体验足够好** - 满足 90% 的使用场景
4. **为后续优化打基础** - 第二第三阶段可以基于此扩展
5. **稳定可靠** - HTTP 长连接比 WebSocket 更稳定

### 实现步骤（5 步）：

```
1️⃣  更新 Proto 定义          (10 分钟)
2️⃣  实现 PullUnreadMessages  (30 分钟)
3️⃣  添加 HTTP 路由           (10 分钟)
4️⃣  生成 Proto 代码          (5 分钟)
5️⃣  测试验证                 (15 分钟)

总计：70 分钟 🚀
```

---

## 💡 核心设计建议

### 1. 消息限流
```go
// 一次最多拉取 100 条未读消息
if req.Limit > 100 {
    req.Limit = 100
}
```

### 2. 自动标记可选
```go
// 让客户端决定是否自动标记
if req.AutoMark {
    // 标记为已读
    h.MarkMessagesAsRead(ctx, ...)
}
```

### 3. 返回统计信息
```protobuf
message PullUnreadMessagesResponse {
    repeated Message msgs = 1;
    int32 total_unread = 2;     // 还有多少未读
    bool has_more = 3;          // 是否有更多消息
}
```

### 4. 支持分页
```
GET /api/v1/messages/unread/pull?limit=100&offset=0
```

### 5. 异常处理
```go
// 如果自动标记失败，也要返回消息
if err := h.MarkMessagesAsRead(...); err != nil {
    log.Printf("Warning: failed to mark as read: %v", err)
    // 仍然返回消息列表
}
```

---

## 🔒 安全性考虑

### 权限验证
```go
func (h *MessageHandler) PullUnreadMessages(...) {
    userID, err := auth.GetUserID(ctx)
    if err != nil {
        return nil, status.Errorf(codes.Unauthenticated, "unauthorized")
    }
    
    // 只能查看自己的未读消息
    // 不需要额外检查，因为查询条件已经是 to_user_id = ?
}
```

### 速率限制（可选）
```go
// 防止同一用户频繁调用
key := "rate_limit:" + userID
count, _ := h.redis.Incr(ctx, key).Result()
if count > 10 {  // 每秒最多 10 次
    return nil, status.Errorf(codes.ResourceExhausted, "too many requests")
}
h.redis.Expire(ctx, key, time.Second)
```

---

## 📈 性能指标预期

| 指标 | 数值 |
|------|-----|
| 响应时间 | 50-150ms |
| 支持并发 | 1000+ |
| 数据库查询 | 1-2 个 SQL |
| 内存占用 | ~1MB（100条消息） |

---

## ✅ 立即行动建议

我建议你现在就开始第一阶段的实现：

```bash
# 估计时间：1-2 小时

1. 我帮你更新 Proto 定义
2. 我帮你实现 PullUnreadMessages 方法
3. 我帮你添加 HTTP 路由
4. 生成 Proto 代码
5. 一起运行测试验证

要开始吗？💪
```

---

## 🎓 总结

| 方面 | 建议 |
|------|-----|
| **即刻实现** | 第一阶段（Pull + 自动标记） |
| **消息限制** | 单次 100 条，支持分页 |
| **自动标记** | 是的，可选参数 |
| **返回信息** | 消息列表 + 总未读数 + 是否有更多 |
| **API 设计** | `GET /api/v1/messages/unread/pull` |
| **下一步** | 第二周开始第二阶段（登录返回未读数） |
| **远期目标** | 第四周开始第三阶段（WebSocket 实时推送） |

---

**现在就开始第一阶段？还是先了解更多细节？** 📱
