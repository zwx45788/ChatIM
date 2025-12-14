# 结构化拉取消息 API

## 概述

重构了 `PullMessages` API，现在返回**按会话分组的结构化数据**，极大简化前端处理逻辑。

---

## 核心改进

### ✅ 之前的问题
```json
// 旧版 API 返回扁平的消息列表
{
  "code": 0,
  "msgs": [
    {"from_user_id": "user_A", "content": "消息1"},
    {"from_user_id": "user_B", "content": "消息2"},
    {"from_user_id": "user_A", "content": "消息3"}
  ]
}

// ❌ 前端需要自己分组：
// - 遍历所有消息
// - 按 from_user_id 分组
// - 计算每个会话的未读数
// - 获取用户昵称和头像
```

### ✅ 现在的优势
```json
// 新版 API 返回结构化的会话列表
{
  "code": 0,
  "conversations": [
    {
      "conversation_id": "private:user_A",
      "type": "private",
      "peer_name": "张三",
      "peer_avatar": "https://avatar.example.com/user_A.jpg",
      "unread_count": 2,
      "last_message_time": 1702512345678,
      "messages": [
        {"id": "msg_1", "content": "消息1", "is_read": false},
        {"id": "msg_3", "content": "消息3", "is_read": false}
      ]
    },
    {
      "conversation_id": "private:user_B",
      "type": "private",
      "peer_name": "李四",
      "unread_count": 1,
      "messages": [
        {"id": "msg_2", "content": "消息2", "is_read": false}
      ]
    }
  ],
  "total_unread": 3,
  "conversation_count": 2
}

// ✅ 前端直接使用，无需额外处理！
```

---

## API 接口

### 请求

```http
GET /api/v1/messages/pull?limit=20&auto_mark=false&include_read=false
Authorization: Bearer <token>
```

**查询参数**：
| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `limit` | int | 20 | 每个会话最多拉取的消息数 |
| `auto_mark` | bool | false | 是否自动标记为已读 |
| `include_read` | bool | false | 是否包含已读消息（默认只返回未读） |

---

### 响应结构

```typescript
interface PullMessagesResponse {
  code: number;                          // 状态码
  message: string;                       // 响应消息
  conversations: ConversationMessages[]; // 会话列表
  total_unread: number;                  // 总未读消息数
  conversation_count: number;            // 有消息的会话数
}

interface ConversationMessages {
  conversation_id: string;   // 会话ID: "private:user_id" 或 "group:group_id"
  type: string;              // 会话类型: "private" 或 "group"
  peer_id: string;           // 对方用户ID或群组ID
  peer_name: string;         // 对方昵称或群名
  peer_avatar: string;       // 对方头像URL
  unread_count: number;      // 该会话未读消息数
  last_message_time: number; // 最后一条消息时间（毫秒时间戳）
  messages: UnifiedMessage[]; // 该会话的消息列表
}

interface UnifiedMessage {
  id: string;            // 消息ID
  type: string;          // "private" 或 "group"
  from_user_id: string;  // 发送者ID
  from_user_name: string; // 发送者昵称
  to_user_id?: string;   // 接收者ID（私聊）
  group_id?: string;     // 群组ID（群聊）
  content: string;       // 消息内容
  created_at: number;    // 时间戳（秒）
  is_read: boolean;      // 是否已读
  stream_id: string;     // Stream消息ID
}
```

---

## 使用示例

### 场景1：拉取所有未读消息（默认）

```bash
GET /api/v1/messages/pull?limit=20
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "消息拉取成功",
  "conversations": [
    {
      "conversation_id": "private:user_456",
      "type": "private",
      "peer_id": "user_456",
      "peer_name": "张三",
      "peer_avatar": "https://avatar.example.com/user_456.jpg",
      "unread_count": 3,
      "last_message_time": 1702512345,
      "messages": [
        {
          "id": "msg_123",
          "type": "private",
          "from_user_id": "user_456",
          "from_user_name": "张三",
          "to_user_id": "current_user",
          "content": "你好，在吗？",
          "created_at": 1702512345,
          "is_read": false,
          "stream_id": "1702512345000-0"
        },
        {
          "id": "msg_124",
          "type": "private",
          "from_user_id": "user_456",
          "from_user_name": "张三",
          "to_user_id": "current_user",
          "content": "有个问题想请教",
          "created_at": 1702512346,
          "is_read": false,
          "stream_id": "1702512346000-0"
        }
      ]
    },
    {
      "conversation_id": "group:group_789",
      "type": "group",
      "peer_id": "group_789",
      "peer_name": "技术讨论组",
      "peer_avatar": "https://avatar.example.com/group_789.jpg",
      "unread_count": 5,
      "last_message_time": 1702512400,
      "messages": [
        {
          "id": "msg_200",
          "type": "group",
          "from_user_id": "user_111",
          "from_user_name": "李四",
          "group_id": "group_789",
          "content": "@所有人 今天下午开会",
          "created_at": 1702512400,
          "is_read": false,
          "stream_id": "1702512400000-0"
        }
      ]
    }
  ],
  "total_unread": 8,
  "conversation_count": 2
}
```

---

### 场景2：拉取并自动标记为已读

```bash
GET /api/v1/messages/pull?limit=20&auto_mark=true
Authorization: Bearer <token>
```

**效果**：
- 返回未读消息
- 后台自动标记这些消息为已读
- 适合用户打开应用时一次性同步

---

### 场景3：拉取包含已读消息

```bash
GET /api/v1/messages/pull?limit=50&include_read=true
Authorization: Bearer <token>
```

**效果**：
- 返回每个会话的最近 50 条消息（包括已读和未读）
- 适合查看历史消息

---

## 前端集成示例

### Vue.js 示例

```vue
<template>
  <div class="message-page">
    <!-- 会话列表 -->
    <div class="conversation-list">
      <div 
        v-for="conv in conversations" 
        :key="conv.conversation_id"
        class="conversation-item"
        @click="openConversation(conv)"
      >
        <!-- 头像 -->
        <img :src="conv.peer_avatar" class="avatar" />
        
        <!-- 内容 -->
        <div class="content">
          <div class="header">
            <span class="name">{{ conv.peer_name }}</span>
            <span class="time">{{ formatTime(conv.last_message_time) }}</span>
          </div>
          <div class="footer">
            <span class="last-message">
              {{ getLastMessage(conv) }}
            </span>
            <span v-if="conv.unread_count > 0" class="unread-badge">
              {{ conv.unread_count }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- 总未读提示 -->
    <div v-if="totalUnread > 0" class="unread-summary">
      共 {{ conversationCount }} 个会话，{{ totalUnread }} 条未读消息
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      conversations: [],
      totalUnread: 0,
      conversationCount: 0,
    };
  },
  
  mounted() {
    this.loadMessages();
  },
  
  methods: {
    // 加载消息
    async loadMessages() {
      try {
        const res = await fetch('/api/v1/messages/pull?limit=20', {
          headers: {
            'Authorization': `Bearer ${this.token}`
          }
        });
        
        const data = await res.json();
        
        if (data.code === 0) {
          this.conversations = data.conversations || [];
          this.totalUnread = data.total_unread || 0;
          this.conversationCount = data.conversation_count || 0;
        }
      } catch (error) {
        console.error('Failed to load messages:', error);
      }
    },
    
    // 获取最后一条消息文本
    getLastMessage(conv) {
      if (conv.messages && conv.messages.length > 0) {
        const lastMsg = conv.messages[conv.messages.length - 1];
        const prefix = conv.type === 'group' ? `${lastMsg.from_user_name}: ` : '';
        return prefix + lastMsg.content;
      }
      return '';
    },
    
    // 打开会话
    openConversation(conv) {
      this.$router.push({
        path: '/chat',
        query: {
          conversation_id: conv.conversation_id
        }
      });
    },
    
    // 格式化时间
    formatTime(timestamp) {
      const date = new Date(timestamp * 1000);
      const now = new Date();
      
      if (date.toDateString() === now.toDateString()) {
        return date.toLocaleTimeString('zh-CN', { 
          hour: '2-digit', 
          minute: '2-digit' 
        });
      } else {
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
  background: white;
}

.conversation-item {
  display: flex;
  padding: 12px;
  border-bottom: 1px solid #eee;
  cursor: pointer;
}

.conversation-item:hover {
  background: #f5f5f5;
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

.name {
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

.unread-summary {
  padding: 12px;
  text-align: center;
  color: #666;
  font-size: 13px;
  background: #f9f9f9;
}
</style>
```

---

### React 示例

```jsx
import React, { useState, useEffect } from 'react';

function MessagePage() {
  const [conversations, setConversations] = useState([]);
  const [totalUnread, setTotalUnread] = useState(0);
  const [conversationCount, setConversationCount] = useState(0);

  useEffect(() => {
    loadMessages();
  }, []);

  const loadMessages = async () => {
    try {
      const res = await fetch('/api/v1/messages/pull?limit=20', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      });
      
      const data = await res.json();
      
      if (data.code === 0) {
        setConversations(data.conversations || []);
        setTotalUnread(data.total_unread || 0);
        setConversationCount(data.conversation_count || 0);
      }
    } catch (error) {
      console.error('Failed to load messages:', error);
    }
  };

  const getLastMessage = (conv) => {
    if (conv.messages && conv.messages.length > 0) {
      const lastMsg = conv.messages[conv.messages.length - 1];
      const prefix = conv.type === 'group' ? `${lastMsg.from_user_name}: ` : '';
      return prefix + lastMsg.content;
    }
    return '';
  };

  return (
    <div className="message-page">
      {/* 会话列表 */}
      {conversations.map(conv => (
        <div key={conv.conversation_id} className="conversation-item">
          <img src={conv.peer_avatar} className="avatar" />
          <div className="content">
            <div className="header">
              <span className="name">{conv.peer_name}</span>
              <span className="time">{formatTime(conv.last_message_time)}</span>
            </div>
            <div className="footer">
              <span className="last-message">{getLastMessage(conv)}</span>
              {conv.unread_count > 0 && (
                <span className="unread-badge">{conv.unread_count}</span>
              )}
            </div>
          </div>
        </div>
      ))}

      {/* 总未读提示 */}
      {totalUnread > 0 && (
        <div className="unread-summary">
          共 {conversationCount} 个会话，{totalUnread} 条未读消息
        </div>
      )}
    </div>
  );
}

export default MessagePage;
```

---

## 技术实现细节

### 1. 数据源优先级

```
1️⃣ Redis Stream（最新消息，包含未持久化的）
    ↓
2️⃣ 按会话分组
    ↓
3️⃣ 补充用户/群组信息
    ↓
4️⃣ 按最后消息时间排序
    ↓
5️⃣ 返回结构化数据
```

### 2. 分组算法

```go
// 遍历 Stream 中的消息
for _, msg := range streamMessages {
    // 确定会话ID
    if msg.type == "private" {
        conversationID = "private:" + peerUserID
    } else if msg.type == "group" {
        conversationID = "group:" + groupID
    }
    
    // 添加到对应会话
    conversationMap[conversationID].messages.append(msg)
    
    // 更新未读计数
    if !msg.is_read {
        conversationMap[conversationID].unread_count++
    }
}
```

### 3. 信息补充

```go
// 查询用户/群组基本信息
SELECT username, avatar FROM users WHERE id = ?
SELECT name, avatar FROM groups WHERE id = ?

// 查询发送者昵称
SELECT username FROM users WHERE id = msg.from_user_id
```

### 4. 自动标记已读（可选）

```go
if req.AutoMark {
    // 异步标记消息为已读
    go func() {
        UPDATE messages SET is_read = TRUE WHERE id IN (...)
    }()
}
```

---

## 性能优化

### 优化1：限制 Stream 读取数量

```go
// 只读取 Stream 中的最近 500 条消息
messages, _ := rdb.XRevRangeN(ctx, streamKey, "+", "-", 500)
```

### 优化2：限制每个会话的消息数

```go
// 每个会话最多返回 limit 条消息（默认20）
if len(conv.Messages) >= limit {
    continue
}
```

### 优化3：异步标记已读

```go
// 不阻塞响应，后台异步执行
if req.AutoMark {
    go h.autoMarkConversationsAsRead(ctx, userID, conversations)
}
```

---

## 与会话列表 API 的区别

| 特性 | `GET /api/v1/conversations` | `GET /api/v1/messages/pull` |
|------|----------------------------|---------------------------|
| **数据源** | Redis Sorted Set（会话列表） | Redis Stream（消息流） |
| **用途** | 会话列表首页展示 | 拉取具体消息内容 |
| **返回内容** | 会话基本信息 + 最后一条消息 | 会话 + 完整消息列表 |
| **分页** | offset/limit | 每个会话的消息 limit |
| **未读计数** | 从 Stream 统计 | 实时计算 |

**推荐使用场景**：
- **会话列表首页** → 使用 `GET /api/v1/conversations`（轻量级）
- **拉取未读消息** → 使用 `GET /api/v1/messages/pull`（含消息内容）

---

## 常见问题

### Q1: 为什么要按会话分组？

**A**: 极大简化前端逻辑，前端无需：
- 遍历消息列表
- 手动分组
- 计算未读数
- 查询用户信息

### Q2: 已读消息会返回吗？

**A**: 默认不返回（`include_read=false`）。如需查看历史消息，设置 `include_read=true`。

### Q3: 群聊消息也支持吗？

**A**: ✅ 完全支持！私聊和群聊统一在一个接口返回。

### Q4: 性能如何？

**A**: 
- 从 Redis Stream 读取，极快
- 限制读取数量（500条）
- 限制每个会话消息数（20条）
- 异步标记已读，不阻塞响应

---

## 总结

✅ **结构化返回**：按会话分组，前端直接使用  
✅ **信息完整**：包含用户昵称、头像、未读数  
✅ **统一支持**：私聊和群聊统一处理  
✅ **实时性高**：优先从 Redis Stream 读取  
✅ **自动标记**：可选自动标记已读  

**API 调用**：
```bash
# 拉取未读消息（默认）
GET /api/v1/messages/pull?limit=20

# 拉取并自动标记已读
GET /api/v1/messages/pull?limit=20&auto_mark=true

# 拉取包含已读消息
GET /api/v1/messages/pull?limit=50&include_read=true
```

开始使用，享受结构化数据带来的便利！🚀
