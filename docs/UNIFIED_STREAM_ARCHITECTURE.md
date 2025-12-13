# 统一 Stream 架构 - 私聊与群聊消息

## 概述

重构后的消息系统采用**统一 Stream 架构**：私聊和群聊消息都写入用户的个人 Stream (`stream:private:{user_id}`)，通过 `type` 字段区分消息类型。

---

## 设计理念

### 为什么统一架构？

**旧架构问题**：
- 私聊消息：写入 `stream:private:{user_id}`
- 群聊消息：写入 `stream:group:{group_id}`
- 两套独立系统，拉取逻辑复杂，客户端需要分别处理

**统一架构优势**：
1. **简化客户端逻辑** - 只需监听一个 Stream，自动接收私聊和群聊消息
2. **统一拉取接口** - 一个接口获取所有未读消息（无需区分私聊/群聊）
3. **统一已读管理** - 使用相同的已读标记机制
4. **会话列表统一** - 私聊和群聊混合排序，按最新消息时间显示

---

## 架构流程

### 群聊消息发送流程

```
用户A 发送群聊消息到 group_123 (成员: A, B, C, D)
    ↓
Message Service: SendGroupMessage
    ↓
1. 查询群成员: [A, B, C, D]
    ↓
2. 遍历成员（排除发送者A），写入各自的 Stream:
   - stream:private:B ← 消息（type: "group"）
   - stream:private:C ← 消息（type: "group"）
   - stream:private:D ← 消息（type: "group"）
    ↓
3. 异步写入 MySQL group_messages 表
    ↓
用户B/C/D 拉取 stream:private:{自己ID} 时收到消息
```

### 私聊消息发送流程（对比）

```
用户A 发送私聊消息给 用户B
    ↓
Message Service: SendMessage
    ↓
1. 写入 stream:private:B ← 消息（type: "private"）
    ↓
2. 异步写入 MySQL messages 表
    ↓
用户B 拉取 stream:private:B 时收到消息
```

**关键差异**：
- 私聊：写入 **1 个** Stream
- 群聊：写入 **N 个** Stream（N = 群成员数 - 1）

---

## 核心实现

### 1. Stream Operator - 写入层

**文件**: `pkg/stream/operator.go`

```go
// AddGroupMessageToMembers 添加群聊消息到所有成员的个人 Stream
func (so *StreamOperator) AddGroupMessageToMembers(
    ctx context.Context,
    msgID, groupID, fromUserID, content, msgType string,
    memberIDs []string,
) error {
    payload := map[string]interface{}{
        "id":           msgID,
        "group_id":     groupID,
        "from_user_id": fromUserID,
        "content":      content,
        "created_at":   time.Now().Unix(),
        "msg_type":     msgType,
        "is_read":      "false",
        "type":         "group", // 🔑 关键标识
    }

    // 遍历所有成员，写入各自的 stream:private:{user_id}
    for _, memberID := range memberIDs {
        if memberID == fromUserID {
            continue // 跳过发送者
        }
        
        streamKey := fmt.Sprintf("stream:private:%s", memberID)
        so.rdb.XAdd(ctx, &redis.XAddArgs{
            Stream: streamKey,
            Values: payload,
        })
    }
    
    return nil
}
```

**关键点**：
- `type: "group"` - 标识这是群聊消息
- `group_id` - 群组ID，用于前端路由到对应会话
- 跳过发送者本人（可选，取决于产品需求）

---

### 2. Message Service - 业务层

**文件**: `internal/message_service/handler/message.go`

```go
func (h *MessageHandler) SendGroupMessage(ctx, req) {
    // 1. 查询群成员列表（带缓存）
    memberIDs, err := h.getGroupMembers(ctx, req.GroupId)
    
    // 2. 写入所有成员的个人 Stream
    err = h.streamOp.AddGroupMessageToMembers(
        ctx, msgID, req.GroupId, fromUserID, 
        req.Content, "text", memberIDs,
    )
    
    // 3. 异步持久化到 MySQL
    go func() {
        h.db.Exec("INSERT INTO group_messages ...")
    }()
}

// getGroupMembers 查询群成员（带 Redis 缓存）
func (h *MessageHandler) getGroupMembers(ctx, groupID) ([]string, error) {
    // 1. 尝试从缓存读取
    members, hit, _ := h.streamOp.GetCachedGroupMembers(ctx, groupID)
    if hit {
        return members, nil
    }
    
    // 2. 从数据库查询
    rows := h.db.Query("SELECT user_id FROM group_members WHERE group_id = ?", groupID)
    
    // 3. 写入缓存（5分钟 TTL）
    h.streamOp.CacheGroupMembers(ctx, groupID, members)
    
    return members, nil
}
```

**优化点**：
- **群成员缓存** - 使用 Redis Set 缓存群成员列表，TTL 5 分钟
- **批量写入** - 遍历成员列表，批量写入各自的 Stream
- **异步持久化** - 不阻塞响应，先写 Stream 再写 MySQL

---

### 3. 客户端拉取消息

**统一拉取接口**（无需区分私聊/群聊）：

```javascript
// 拉取未读消息（私聊 + 群聊统一返回）
async function pullMessages() {
  const response = await fetch('/api/v1/messages/unread/pull', {
    headers: { 'Authorization': `Bearer ${token}` }
  });
  
  const data = await response.json();
  
  data.msgs.forEach(msg => {
    if (msg.type === 'group') {
      console.log('📨 群聊消息:', msg.group_id, msg.content);
      updateGroupConversation(msg.group_id, msg);
    } else {
      console.log('💬 私聊消息:', msg.from_user_id, msg.content);
      updatePrivateConversation(msg.from_user_id, msg);
    }
  });
}
```

**消息结构对比**：

| 字段 | 私聊消息 | 群聊消息 |
|------|---------|---------|
| `id` | ✅ 消息ID | ✅ 消息ID |
| `type` | `"private"` | `"group"` |
| `from_user_id` | ✅ 发送者ID | ✅ 发送者ID |
| `to_user_id` | ✅ 接收者ID | ❌ 无 |
| `group_id` | ❌ 无 | ✅ 群组ID |
| `content` | ✅ 消息内容 | ✅ 消息内容 |
| `created_at` | ✅ 时间戳 | ✅ 时间戳 |

---

## 缓存策略

### 群成员缓存

**Redis 数据结构**: Set  
**Key 格式**: `group:members:{group_id}`  
**TTL**: 5 分钟  
**内容**: 群成员的 `user_id` 列表

**缓存方法**：

```go
// 写入缓存
CacheGroupMembers(ctx, groupID, []string{"user1", "user2", "user3"})

// 读取缓存
members, hit, _ := GetCachedGroupMembers(ctx, groupID)

// 清除缓存（群成员变化时调用）
InvalidateGroupMemberCache(ctx, groupID)
```

**何时清除缓存**：
- 添加群成员
- 移除群成员
- 用户退出群聊

---

## 性能分析

### 写入性能

**私聊消息**：
- 1 次 Redis Stream 写入
- 1 次 MySQL 写入（异步）
- **总耗时**: ~1-3ms

**群聊消息**（100 人群）：
- 99 次 Redis Stream 写入（遍历成员）
- 1 次 MySQL 写入（异步）
- **总耗时**: ~50-150ms（取决于网络延迟）

### 优化方案

#### 1. 批量写入优化

```go
// 使用 Pipeline 批量写入
pipe := so.rdb.Pipeline()
for _, memberID := range memberIDs {
    streamKey := fmt.Sprintf("stream:private:%s", memberID)
    pipe.XAdd(ctx, &redis.XAddArgs{
        Stream: streamKey,
        Values: payload,
    })
}
pipe.Exec(ctx)
```

**优化效果**: 100 人群从 ~150ms 降低到 ~10ms

#### 2. 异步写入

```go
// 主线程立即返回，后台异步写入
go func() {
    so.AddGroupMessageToMembers(...)
}()

return &pb.SendGroupMessageResponse{
    Code: 0,
    Message: "消息已提交发送",
}
```

**优化效果**: API 响应时间 < 5ms

#### 3. 大群消息特殊处理

```go
// 超过 500 人的大群，切换回 stream:group:{group_id} 模式
if len(memberIDs) > 500 {
    return so.AddGroupMessage(ctx, ...)
}
```

---

## 数据一致性

### 问题场景

**场景 1**: 用户发送消息时，部分成员写入失败

```
group_123 成员: [A, B, C, D, E]
写入结果:
  - stream:private:B ✅ 成功
  - stream:private:C ❌ 失败（Redis 连接超时）
  - stream:private:D ✅ 成功
  - stream:private:E ✅ 成功
```

**解决方案**：
1. **记录失败成员** - 日志记录失败的 `memberID`
2. **重试机制** - 失败的成员写入重试队列
3. **最终一致性** - 用户拉取时从 MySQL 补偿

### 场景 2: 群成员变化与消息发送的竞态

```
时刻 T1: 用户A发送消息，查询到成员 [A, B, C]
时刻 T2: 用户D加入群聊
时刻 T3: 消息写入 B 和 C 的 Stream
结果: 用户D 未收到消息
```

**解决方案**：
- **可接受** - 新成员不应看到加入前的历史消息
- **如需解决** - 消息写入后发布事件，新成员上线时补拉历史

---

## 迁移指南

### 从旧架构迁移

如果之前使用 `stream:group:{group_id}` 模式：

1. **保留旧 Stream** - 不删除旧数据，兼容历史消息
2. **双写过渡** - 新消息同时写入新旧 Stream
3. **客户端适配** - 支持读取两种格式
4. **逐步切换** - 验证新架构稳定后，停止写入旧 Stream

---

## 测试用例

### 单元测试

```go
func TestAddGroupMessageToMembers(t *testing.T) {
    // 1. 准备测试数据
    members := []string{"user1", "user2", "user3"}
    
    // 2. 发送消息
    err := streamOp.AddGroupMessageToMembers(
        ctx, "msg123", "group1", "sender", "Hello", "text", members,
    )
    
    // 3. 验证写入
    for _, memberID := range members {
        if memberID == "sender" {
            continue
        }
        
        streamKey := fmt.Sprintf("stream:private:%s", memberID)
        msgs := rdb.XRange(ctx, streamKey, "-", "+").Val()
        
        assert.NotEmpty(t, msgs, "Member %s should receive message", memberID)
        assert.Equal(t, "group", msgs[0].Values["type"])
    }
}
```

### 集成测试

```bash
# 1. 创建群聊
POST /api/v1/groups
{"name": "测试群"}
# 返回: {"group_id": "group_123"}

# 2. 添加成员
POST /api/v1/groups/group_123/members
{"user_ids": ["user_B", "user_C"]}

# 3. 发送群聊消息
POST /api/v1/groups/messages
{"group_id": "group_123", "content": "Hello"}

# 4. 验证成员B收到消息
GET /api/v1/messages/unread/pull
# 返回: {"msgs": [{"type": "group", "group_id": "group_123", ...}]}
```

---

## 监控指标

### 关键指标

| 指标 | 说明 | 告警阈值 |
|------|------|---------|
| **群聊消息写入成功率** | `成功写入成员数 / 总成员数` | < 95% |
| **群聊消息发送延迟** | `发送完成时间 - 请求时间` | > 500ms |
| **缓存命中率** | `缓存命中次数 / 总查询次数` | < 80% |
| **Stream 长度** | `stream:private:{user_id}` 条目数 | > 10000 |

### 监控实现

```go
// 埋点示例
func (so *StreamOperator) AddGroupMessageToMembers(...) error {
    start := time.Now()
    successCount := 0
    
    for _, memberID := range memberIDs {
        err := so.rdb.XAdd(...)
        if err == nil {
            successCount++
        }
    }
    
    // 上报指标
    metrics.GroupMessageWriteRate.Observe(float64(successCount) / float64(len(memberIDs)))
    metrics.GroupMessageLatency.Observe(time.Since(start).Seconds())
}
```

---

## FAQ

### Q1: 发送者会收到自己的消息吗？

**A**: 默认不会。代码中通过 `if memberID == fromUserID { continue }` 跳过发送者。如需发送者也收到（如显示"已发送"状态），可移除此判断。

### Q2: 群聊消息占用更多 Redis 存储吗？

**A**: 是的。100 人群的一条消息会写入 99 个 Stream，存储空间是单条的 99 倍。但考虑：
- Redis Stream 使用压缩存储，实际占用比预期小
- 设置合理的 TTL 或 MAXLEN 限制
- 超大群（>500人）可切换回集中式存储

### Q3: 如何处理超大群（5000+ 人）？

**A**: 建议策略：
1. **分层架构** - 大群使用 `stream:group:{group_id}`，小群使用个人 Stream
2. **延迟推送** - 消息先入队，后台异步分发
3. **分批处理** - 每批 100 人，避免单次遍历阻塞

### Q4: 群成员列表缓存失效怎么办？

**A**: 自动降级到数据库查询：
```go
members, hit, _ := GetCachedGroupMembers(ctx, groupID)
if !hit {
    members = queryFromDB(groupID) // 自动查库并回写缓存
}
```

---

## 相关文件

- **Stream 操作**: `pkg/stream/operator.go`
- **消息服务**: `internal/message_service/handler/message.go`
- **缓存实现**: `pkg/stream/operator.go` (CacheGroupMembers)
- **订阅器**: `internal/websocket/subscriber.go`

---

## 总结

统一 Stream 架构的核心优势：

✅ **简化客户端** - 一个拉取接口，自动获取私聊和群聊  
✅ **统一会话列表** - 按时间混合排序，体验一致  
✅ **易于扩展** - 新增消息类型（如系统通知）只需加 `type` 字段  
✅ **降低复杂度** - 无需维护两套独立的消息系统  

权衡：
- ⚠️ 群聊消息写入放大（N 倍）
- ⚠️ 需要群成员缓存优化
- ⚠️ 超大群需要特殊处理

整体来说，对于中小规模应用（群聊 < 500 人），统一架构是更优选择。
