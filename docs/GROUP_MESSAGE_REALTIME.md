# 群聊消息实时接收机制

## 概述

群聊消息通过 **Redis Pub/Sub + WebSocket** 实现实时推送。当用户发送群聊消息时，消息会通过 Redis 频道广播给所有在线群成员的 WebSocket 连接。

---

## 架构流程

```
发送者客户端
    ↓ POST /api/v1/groups/messages
API Gateway
    ↓ gRPC SendGroupMessage
Message Service
    ↓ 1. AddGroupMessage (Redis Stream)
    ↓ 2. Publish to Redis (group_msg:{group_id})
    ↓ 3. Save to MySQL (async)
Redis Pub/Sub Channel: group_msg:group_123
    ↓ (Pattern: group_msg:*)
WebSocket Subscriber (subscribeGroupMessages)
    ↓ 查询群成员
    ↓ 构造消息 JSON
    ↓ 推送给所有在线群成员
WebSocket Hub
    ↓ NotifyUser(memberID, messageJSON)
接收者客户端 (WebSocket 连接)
    ↓ 收到群聊消息
前端显示消息
```

---

## 核心组件

### 1. 消息服务发送端

**文件**: `internal/message_service/handler/message.go`

```go
func (h *MessageHandler) SendGroupMessage(ctx, req) {
    // 1. 写入 Redis Stream
    h.streamOp.AddGroupMessage(...)
    
    // 2. 发布通知到 Redis Pub/Sub
    go func() {
        notification := {
            "msg_id": msgID,
            "group_id": groupID,
            "from_user_id": fromUserID,
            "content": content,
            "created_at": timestamp
        }
        h.rdb.Publish(ctx, "group_msg:"+groupID, notificationJSON)
    }()
    
    // 3. 异步写数据库
    go func() {
        h.db.Exec("INSERT INTO group_messages ...")
    }()
}
```

**关键点**:
- Redis Pub/Sub 频道格式: `group_msg:{group_id}`
- 每个群有独立频道（如 `group_msg:group_123`）
- 异步发布，不阻塞响应

---

### 2. WebSocket 订阅端

**文件**: `internal/websocket/subscriber.go`

#### 启动订阅

```go
func StartSubscriber(hub *Hub) {
    // 私聊消息订阅
    go subscribePrivateMessages(hub, rdb, cfg)
    
    // 群聊消息订阅（模式匹配）
    go subscribeGroupMessages(hub, rdb, cfg)
}
```

#### 群聊订阅逻辑

```go
func subscribeGroupMessages(hub, rdb, cfg) {
    // 使用 PSubscribe 订阅模式 "group_msg:*"
    pubsub := rdb.PSubscribe(ctx, "group_msg:*")
    ch := pubsub.Channel()
    
    for msg := range ch {
        // 1. 解析通知
        var notification GroupMessageNotification
        json.Unmarshal(msg.Payload, &notification)
        
        // 2. 查询群成员
        members := fetchGroupMembers(notification.GroupID)
        
        // 3. 构造消息
        groupMsg := GroupMessagePayload{
            ID: notification.MsgID,
            GroupID: notification.GroupID,
            FromUserID: notification.FromUserID,
            Content: notification.Content,
            Type: "group"
        }
        
        // 4. 推送给所有在线成员（除发送者）
        for _, memberID := range members {
            if memberID != notification.FromUserID {
                hub.NotifyUser(memberID, messageJSON)
            }
        }
    }
}
```

**关键特性**:
- 使用 `PSubscribe("group_msg:*")` 模式订阅所有群聊频道
- 自动查询群成员列表
- 排除发送者本人（避免重复推送）
- 只推送给**在线**用户（离线用户通过拉取获取）

---

### 3. WebSocket Hub 推送

**文件**: `internal/websocket/hub.go`

```go
func (h *Hub) NotifyUser(userID string, message []byte) {
    if client, ok := h.clients[userID]; ok {
        client.Send <- message
    }
}
```

**Hub 职责**:
- 维护所有在线用户的 WebSocket 连接映射
- 提供 `NotifyUser` 方法向指定用户推送消息
- 处理连接断开和通道阻塞

---

## 数据结构

### GroupMessageNotification (Redis Pub/Sub 载荷)

```json
{
  "msg_id": "550e8400-e29b-41d4-a716-446655440000",
  "group_id": "group_123",
  "from_user_id": "user_456",
  "content": "Hello everyone!",
  "created_at": "2025-12-13 10:30:45"
}
```

### GroupMessagePayload (推送给客户端)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "group_id": "group_123",
  "from_user_id": "user_456",
  "content": "Hello everyone!",
  "created_at": "2025-12-13 10:30:45",
  "type": "group"
}
```

**`type` 字段用途**:
- 客户端通过 `type` 区分消息类型：
  - `"private"` - 私聊消息
  - `"group"` - 群聊消息
- 前端可据此路由到不同的消息处理逻辑

---

## 客户端 WebSocket 接收示例

### JavaScript

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?token=YOUR_JWT_TOKEN');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  if (message.type === 'group') {
    console.log('📨 收到群聊消息:');
    console.log('  群组ID:', message.group_id);
    console.log('  发送者:', message.from_user_id);
    console.log('  内容:', message.content);
    
    // 更新群聊会话列表
    updateGroupConversation(message.group_id, message);
    
    // 如果当前正在查看该群聊，追加消息
    if (currentGroupId === message.group_id) {
      appendMessageToChat(message);
    }
  } else if (message.type === 'private') {
    console.log('📨 收到私聊消息:', message);
    updatePrivateConversation(message);
  }
};

ws.onerror = (error) => {
  console.error('WebSocket 错误:', error);
};

ws.onclose = () => {
  console.log('WebSocket 连接已关闭，尝试重连...');
  setTimeout(() => reconnect(), 3000);
};
```

---

## 离线消息处理

用户离线期间的群聊消息通过以下方式获取：

1. **登录时拉取**
   - 接口: `GET /api/v1/unread/all`
   - 返回包含 `group_unreads` 字段（所有群的未读消息）

2. **进入群聊时拉取历史**
   - 接口: `GET /api/v1/groups/:group_id/messages`
   - 分页获取历史消息

3. **已读状态同步**
   - 接口: `POST /api/v1/messages/group/read`
   - 标记群聊消息为已读

---

## Redis 频道设计

| 频道模式 | 用途 | 订阅方式 | 示例 |
|---------|------|---------|------|
| `message_notifications` | 私聊消息通知 | `Subscribe` | 单一频道 |
| `group_msg:*` | 群聊消息通知 | `PSubscribe` | `group_msg:group_123` |

**为什么用模式订阅**:
- 群组数量动态变化，无法预先订阅所有频道
- `PSubscribe("group_msg:*")` 自动匹配所有群聊频道
- 无需在群创建/删除时手动管理订阅

---

## 性能优化

### 1. 群成员缓存

当前每次消息推送都查询数据库获取群成员列表。优化方案：

```go
// 使用 Redis 缓存群成员列表
func fetchGroupMembersWithCache(groupID string, rdb *redis.Client) []string {
    cacheKey := "group_members:" + groupID
    
    // 1. 尝试从 Redis 读取
    members, err := rdb.SMembers(ctx, cacheKey).Result()
    if err == nil && len(members) > 0 {
        return members
    }
    
    // 2. 从数据库查询
    members = fetchGroupMembersFromDB(groupID)
    
    // 3. 写入 Redis（TTL 5分钟）
    rdb.SAdd(ctx, cacheKey, members)
    rdb.Expire(ctx, cacheKey, 5*time.Minute)
    
    return members
}
```

### 2. 批量推送优化

如果群成员过多（如 1000+ 人），可以：
- 分批推送（每批 100 人）
- 使用 goroutine 并发推送
- 限制并发数（如 10 个 worker）

### 3. 连接池复用

当前每次查询都创建新的数据库连接。优化：
- 使用全局数据库连接池
- 在 `StartSubscriber` 初始化时创建，避免频繁 `InitDB`

---

## 测试场景

### 场景 1: 三人群聊

1. 用户 A、B、C 都在线，连接 WebSocket
2. 用户 A 发送群聊消息："大家好！"
3. 验证：
   - ✅ 用户 B 和 C 实时收到消息
   - ✅ 用户 A 不会收到自己的消息（避免重复）
   - ✅ 消息类型为 `"group"`

### 场景 2: 部分离线

1. 用户 A、B 在线，用户 C 离线
2. 用户 A 发送群聊消息
3. 验证：
   - ✅ 用户 B 实时收到
   - ✅ 用户 C 离线，未收到推送
   - ✅ 用户 C 上线后调用 `/unread/all` 能拉取到该消息

### 场景 3: 跨群消息隔离

1. 用户 A 同时在 group_1 和 group_2
2. 用户 B 在 group_1 发送消息
3. 验证：
   - ✅ 用户 A 只收到 group_1 的消息
   - ✅ group_2 的成员不受影响

---

## 故障排查

### 问题 1: 群聊消息收不到

**检查步骤**:
1. 确认 WebSocket 订阅器已启动：查看日志 `"Subscribed to Redis pattern 'group_msg:*'"`
2. 确认消息发送时有发布通知：查看日志 `"Published group message to channel group_msg:xxx"`
3. 确认用户在群成员列表中：查询 `group_members` 表
4. 确认 WebSocket 连接正常：检查 Hub 中的 `clients` 映射

### 问题 2: 收到重复消息

**可能原因**:
- 发送者也收到了推送（未排除）
- 多个订阅器实例重复订阅

**解决方案**:
- 确认 `if memberID != notification.FromUserID` 逻辑存在
- 确保只启动一个 `StartSubscriber` 实例

### 问题 3: 高延迟

**排查**:
- Redis 响应时间：使用 `redis-cli --latency`
- 数据库查询慢：添加索引 `CREATE INDEX idx_group_members ON group_members(group_id)`
- 群成员过多：实施缓存优化

---

## 下一步扩展

1. **在线状态同步**
   - 用户上线时发布 `user_online:{user_id}` 事件
   - 前端显示群成员在线状态

2. **消息已读回执**
   - 用户阅读消息后发送已读确认
   - 发送者看到"已读 3/5"统计

3. **@提及通知**
   - 解析消息内容中的 `@username`
   - 被提及的用户收到特殊通知

4. **消息撤回同步**
   - 发布 `group_msg_recall:{group_id}` 事件
   - 所有在线用户同步删除该消息

5. **打字状态指示器**
   - 用户输入时发布 `group_typing:{group_id}` 事件
   - 其他成员看到"某某正在输入..."

---

## 相关文件

- **订阅实现**: `internal/websocket/subscriber.go`
- **Hub 实现**: `internal/websocket/hub.go`
- **消息服务**: `internal/message_service/handler/message.go`
- **启动入口**: `cmd/api/main.go` (调用 `StartSubscriber(hub)`)
