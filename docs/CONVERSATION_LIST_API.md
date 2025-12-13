# 会话列表按时间排序 + 置顶功能

## 概述

基于 **Redis Sorted Set** 实现的会话列表管理，支持自动按时间排序和会话置顶功能。

---

## 功能特性

✅ **自动时间排序** - 最新消息的会话自动排在前面  
✅ **会话置顶** - 重要会话固定在列表顶部  
✅ **混合排序** - 置顶会话在前，非置顶会话按时间降序  
✅ **自动更新** - 收发消息时自动更新会话时间  
✅ **高性能** - Redis Sorted Set O(log N) 复杂度  
✅ **支持分页** - 适合大量会话场景  

---

## API 接口

### 1. 获取会话列表

```http
GET /api/v1/conversations?offset=0&limit=20
Authorization: Bearer <token>
```

**响应示例**：
```json
{
  "code": 0,
  "message": "success",
  "conversations": [
    {
      "conversation_id": "private:user_456",
      "type": "private",
      "peer_id": "user_456",
      "title": "张三",
      "avatar": "https://avatar.example.com/user_456.jpg",
      "last_message": "好的，明天见",
      "last_message_time": 1702512345678,
      "unread_count": 5,
      "is_pinned": true
    },
    {
      "conversation_id": "group:group_789",
      "type": "group",
      "peer_id": "group_789",
      "title": "项目讨论组",
      "avatar": "https://avatar.example.com/group_789.jpg",
      "last_message": "会议记录已上传",
      "last_message_time": 1702512234567,
      "unread_count": 2,
      "is_pinned": false
    }
  ],
  "total": 2,
  "has_more": false
}
```

**字段说明**：
- `conversation_id` - 会话唯一ID，格式：`private:{user_id}` 或 `group:{group_id}`
- `type` - 会话类型：`private`（私聊）或 `group`（群聊）
- `peer_id` - 对方用户ID或群组ID
- `title` - 显示名称（用户昵称或群名）
- `last_message_time` - 毫秒时间戳
- `is_pinned` - 是否置顶

---

### 2. 置顶会话

```http
POST /api/v1/conversations/:conversation_id/pin
Authorization: Bearer <token>
```

**示例**：
```bash
curl -X POST http://localhost:8080/api/v1/conversations/private:user_456/pin \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**响应**：
```json
{
  "code": 0,
  "message": "Conversation pinned successfully"
}
```

---

### 3. 取消置顶

```http
DELETE /api/v1/conversations/:conversation_id/pin
Authorization: Bearer <token>
```

**示例**：
```bash
curl -X DELETE http://localhost:8080/api/v1/conversations/private:user_456/pin \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

### 4. 删除会话

```http
DELETE /api/v1/conversations/:conversation_id
Authorization: Bearer <token>
```

**注意**：删除会话只是从列表中移除，不删除历史消息。

---

## 前端集成示例

### Vue.js 示例

```vue
<template>
  <div class="conversation-list">
    <!-- 会话列表 -->
    <div 
      v-for="conv in conversations" 
      :key="conv.conversation_id"
      class="conversation-item"
      :class="{ pinned: conv.is_pinned }"
      @click="openConversation(conv)"
    >
      <!-- 头像 -->
      <img :src="conv.avatar" class="avatar" />
      
      <!-- 内容区 -->
      <div class="content">
        <div class="header">
          <span class="title">{{ conv.title }}</span>
          <span class="time">{{ formatTime(conv.last_message_time) }}</span>
        </div>
        <div class="footer">
          <span class="last-message">{{ conv.last_message }}</span>
          <span v-if="conv.unread_count > 0" class="unread-badge">
            {{ conv.unread_count }}
          </span>
        </div>
      </div>
      
      <!-- 置顶图标 -->
      <div v-if="conv.is_pinned" class="pin-icon">📌</div>
      
      <!-- 右键菜单 -->
      <div class="actions">
        <button @click.stop="togglePin(conv)">
          {{ conv.is_pinned ? '取消置顶' : '置顶' }}
        </button>
        <button @click.stop="deleteConversation(conv)">删除</button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      conversations: [],
      offset: 0,
      limit: 20,
    };
  },
  
  mounted() {
    this.loadConversations();
  },
  
  methods: {
    // 加载会话列表
    async loadConversations() {
      const res = await fetch(
        `/api/v1/conversations?offset=${this.offset}&limit=${this.limit}`,
        {
          headers: {
            'Authorization': `Bearer ${this.token}`
          }
        }
      );
      
      const data = await res.json();
      this.conversations = data.conversations;
    },
    
    // 置顶/取消置顶
    async togglePin(conv) {
      const method = conv.is_pinned ? 'DELETE' : 'POST';
      const url = `/api/v1/conversations/${conv.conversation_id}/pin`;
      
      await fetch(url, {
        method,
        headers: {
          'Authorization': `Bearer ${this.token}`
        }
      });
      
      // 刷新列表
      this.loadConversations();
    },
    
    // 删除会话
    async deleteConversation(conv) {
      if (!confirm('确认删除该会话？')) return;
      
      await fetch(`/api/v1/conversations/${conv.conversation_id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${this.token}`
        }
      });
      
      // 从列表中移除
      this.conversations = this.conversations.filter(
        c => c.conversation_id !== conv.conversation_id
      );
    },
    
    // 打开会话
    openConversation(conv) {
      this.$router.push(`/chat/${conv.conversation_id}`);
    },
    
    // 格式化时间
    formatTime(timestamp) {
      const date = new Date(timestamp);
      const now = new Date();
      
      if (date.toDateString() === now.toDateString()) {
        // 今天：显示时分
        return date.toLocaleTimeString('zh-CN', { 
          hour: '2-digit', 
          minute: '2-digit' 
        });
      } else {
        // 其他：显示月日
        return date.toLocaleDateString('zh-CN', { 
          month: '2-digit', 
          day: '2-digit' 
        });
      }
    }
  }
};
</script>

<style scoped>
.conversation-list {
  max-width: 400px;
}

.conversation-item {
  display: flex;
  padding: 12px;
  border-bottom: 1px solid #eee;
  cursor: pointer;
  position: relative;
}

.conversation-item:hover {
  background: #f5f5f5;
}

.conversation-item.pinned {
  background: #fff9e6;
}

.avatar {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  margin-right: 12px;
}

.content {
  flex: 1;
}

.header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 4px;
}

.title {
  font-weight: bold;
  font-size: 15px;
}

.time {
  font-size: 12px;
  color: #999;
}

.footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.last-message {
  font-size: 13px;
  color: #666;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 200px;
}

.unread-badge {
  background: #ff4d4f;
  color: white;
  border-radius: 10px;
  padding: 2px 6px;
  font-size: 12px;
  min-width: 18px;
  text-align: center;
}

.pin-icon {
  position: absolute;
  top: 8px;
  right: 8px;
  font-size: 12px;
}
</style>
```

---

## 技术实现细节

### Redis 数据结构

```redis
# Key: conversation:list:{user_id}
# Type: Sorted Set
# Score: 时间戳（毫秒）或置顶标记

# 普通会话
ZADD conversation:list:user_123 1702512000000 "private:user_456"
ZADD conversation:list:user_123 1702513000000 "group:group_789"

# 置顶会话（score = 10^13 + 原时间戳）
ZADD conversation:list:user_123 10001702512000000 "private:user_abc"
```

### Score 规则

| 状态 | Score 值 | 说明 |
|------|---------|------|
| **普通会话** | 当前时间毫秒 | 例如：1702512000000 |
| **置顶会话** | 10^13 + 时间毫秒 | 例如：10001702512000000 |

**排序规则**：
- Redis `ZREVRANGE` 按 score 降序排列
- 置顶会话的 score > 10^13，永远排在普通会话前面
- 置顶会话之间按置顶时间排序
- 普通会话之间按最新消息时间排序

---

## 自动更新机制

### 收到消息时自动更新

**私聊消息**：
```go
// 发送方和接收方的会话列表都更新
h.streamOp.UpdateConversationTime(ctx, fromUserID, "private:toUserID")
h.streamOp.UpdateConversationTime(ctx, toUserID, "private:fromUserID")
```

**群聊消息**：
```go
// 所有群成员的会话列表都更新
for _, memberID := range memberIDs {
    h.streamOp.UpdateConversationTime(ctx, memberID, "group:groupID")
}
```

### 置顶状态保持

```go
// 更新时检查是否已置顶
currentScore := rdb.ZScore(ctx, key, conversationID).Val()
if currentScore > 10000000000000 {
    // 已置顶，保持置顶状态
    score = 10000000000000 + float64(time.Now().UnixMilli())
}
```

---

## 性能优化

### 1. 缓存过期策略

```go
// 会话列表 30 天过期
so.rdb.Expire(ctx, key, 30*24*time.Hour)
```

### 2. 分页查询

```go
// ZREVRANGE 支持高效分页
results := rdb.ZRevRangeWithScores(ctx, key, offset, offset+limit-1)
```

### 3. 批量更新

```go
// 使用 Pipeline 批量更新会话
pipe := rdb.Pipeline()
for _, memberID := range memberIDs {
    pipe.ZAdd(ctx, fmt.Sprintf("conversation:list:%s", memberID), ...)
}
pipe.Exec(ctx)
```

---

## 测试用例

### 场景 1: 基本排序

```bash
# 1. 用户发送两条消息
POST /api/v1/messages/send
{"to_user_id": "user_B", "content": "消息1"}  # 10:00

POST /api/v1/messages/send
{"to_user_id": "user_C", "content": "消息2"}  # 10:05

# 2. 获取会话列表
GET /api/v1/conversations

# 预期结果：user_C 在前（时间更新）
[
  {"conversation_id": "private:user_C", "last_message_time": 10:05},
  {"conversation_id": "private:user_B", "last_message_time": 10:00}
]
```

### 场景 2: 置顶功能

```bash
# 1. 置顶 user_B 的会话
POST /api/v1/conversations/private:user_B/pin

# 2. 获取会话列表
GET /api/v1/conversations

# 预期结果：user_B 置顶在前
[
  {"conversation_id": "private:user_B", "is_pinned": true},
  {"conversation_id": "private:user_C", "is_pinned": false}
]
```

### 场景 3: 置顶会话收到新消息

```bash
# 1. user_C 已置顶
# 2. user_B 发来新消息（时间更晚）

# 预期结果：user_C 仍在前（置顶优先）
[
  {"conversation_id": "private:user_C", "is_pinned": true},
  {"conversation_id": "private:user_B", "is_pinned": false}
]
```

---

## 常见问题

### Q1: 会话列表为什么是空的？

**A**: 会话列表是在收发消息时自动创建的。如果从未发送过消息，列表为空是正常的。

**解决方案**：
- 发送一条测试消息
- 或者手动初始化会话列表

### Q2: 置顶后时间还会更新吗？

**A**: 会更新，但置顶状态保持不变。置顶会话之间按最新消息时间排序。

### Q3: 删除会话后历史消息还在吗？

**A**: 在。删除会话只是从列表中移除，不影响 Stream 中的历史消息。重新发送消息后会话会重新出现。

### Q4: 支持多端同步吗？

**A**: 支持。会话列表存储在 Redis 中，用户ID作为 Key，所有设备共享同一份数据。

---

## 扩展功能

### 1. 会话草稿

```go
// 保存草稿
func SaveDraft(ctx context.Context, rdb *redis.Client, userID, conversationID, draft string) {
    key := fmt.Sprintf("conversation:draft:%s:%s", userID, conversationID)
    rdb.Set(ctx, key, draft, 7*24*time.Hour)
}
```

### 2. 会话免打扰

```go
// 设置免打扰
func MuteConversation(ctx context.Context, rdb *redis.Client, userID, conversationID string) {
    key := fmt.Sprintf("conversation:mute:%s", userID)
    rdb.SAdd(ctx, key, conversationID)
}
```

### 3. 会话标签

```go
// 添加标签
func TagConversation(ctx context.Context, rdb *redis.Client, userID, conversationID, tag string) {
    key := fmt.Sprintf("conversation:tags:%s:%s", userID, conversationID)
    rdb.SAdd(ctx, key, tag)
}
```

---

## 相关文件

- **Stream Operator**: `pkg/stream/operator.go`
- **Conversation Handler**: `internal/api_gateway/handler/conversation.go`
- **Message Handler**: `internal/message_service/handler/message.go`
- **API Routes**: `cmd/api/main.go`

---

## 总结

✅ **实现完成**：会话列表自动排序 + 置顶功能  
✅ **技术方案**：Redis Sorted Set（高性能）  
✅ **自动更新**：收发消息时自动维护会话时间  
✅ **用户体验**：置顶在前，最新消息优先显示  
✅ **易于集成**：RESTful API，前端友好  

开始使用：发送消息后，会话列表自动出现并按时间排序！📱
