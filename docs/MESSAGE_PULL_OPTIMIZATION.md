# 消息拉取逻辑优化方案

## 🔍 当前问题分析

### ❌ 问题1：`PullMessages` 只查询数据库，忽略 Redis Stream

**当前实现**：
```go
// internal/message_service/handler/message.go:195-272
func (h *MessageHandler) PullMessages(ctx context.Context, req *pb.PullMessagesRequest) (*pb.PullMessagesResponse, error) {
    // ❌ 只从 MySQL 查询
    query := `
        SELECT id, from_user_id, to_user_id, content, is_read, read_at, created_at
        FROM messages
        WHERE to_user_id = ?
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?`
    
    rows, err := h.db.QueryContext(ctx, query, userID, limit, offset)
    // ...
}
```

**问题**：
- ❌ 消息异步写入数据库（5秒延迟），用户拉取不到最新消息
- ❌ 只支持私聊（`WHERE to_user_id = ?`），群聊消息无法拉取
- ❌ 无法按会话ID过滤消息

**用户影响**：
- 发送消息后立即拉取，看不到自己发的消息
- 群聊消息无法通过此接口获取
- 消息混在一起，前端需要自己分组

---

### ❌ 问题2：缺少按会话拉取消息的 API

**当前 API 列表**：
| API | 功能 | 问题 |
|-----|------|------|
| `PullMessages` | 拉取所有私聊消息 | ❌ 无法按会话过滤 |
| `PullUnreadMessages` | 拉取所有未读私聊 | ❌ 无法按会话过滤 |
| `PullGroupMessages` | 拉取群聊消息 | ❌ 未充分利用 Stream |

**缺失功能**：
- ❌ 按 `conversation_id` 拉取消息（如：`private:user_123`）
- ❌ 支持私聊和群聊统一接口
- ❌ 从 Redis Stream 优先读取最新消息

**使用场景**：
```javascript
// 前端点击某个会话时，需要加载该会话的历史消息
loadConversationHistory("private:user_456", 50)
loadConversationHistory("group:group_789", 50)
```

---

### ❌ 问题3：会话列表中的 N+1 查询问题

**当前实现**：
```go
// internal/api_gateway/handler/conversation.go:260-289
func (h *ConversationHandler) getLastMessage(ctx context.Context, userID, conversationID string) string {
    // ❌ 每个会话都读取 20 条消息
    messages, err := h.rdb.XRevRangeN(ctx, streamKey, "+", "-", 20).Result()
    
    // ❌ 遍历所有消息查找匹配的
    for _, msg := range messages {
        if conversationID[:8] == "private:" {
            if msg.Values["to_user_id"] == conversationID[8:] || msg.Values["from_user_id"] == conversationID[8:] {
                return truncateString(content, 50)
            }
        }
    }
}
```

**性能问题**：
- 10 个会话 = 10 次 Redis 查询
- 每次查询读取 20 条消息，实际只用 1 条
- 时间复杂度：O(会话数 × 20)

---

## ✅ 优化方案

### 方案1：新增 `PullConversationMessages` API（推荐）

**核心思路**：
1. **优先从 Redis Stream 读取**（最新消息）
2. **按会话ID过滤**（支持私聊和群聊）
3. **自动回退到数据库**（历史消息）

#### 1.1 Proto 定义

```protobuf
// api/proto/message.proto

// 按会话拉取消息请求
message PullConversationMessagesRequest {
  string conversation_id = 1;  // "private:user_456" 或 "group:group_789"
  int64 limit = 2;             // 拉取数量（默认50）
  string start_id = 3;         // 起始消息ID（用于分页，默认 "+" 表示最新）
  bool use_stream = 4;         // 是否优先使用 Stream（默认 true）
}

// 按会话拉取消息响应
message PullConversationMessagesResponse {
  int32 code = 1;
  string message = 2;
  repeated UnifiedMessage messages = 3; // 统一的消息格式
  bool has_more = 4;                    // 是否还有更多
  string next_start_id = 5;             // 下一页的起始ID
}

// 统一消息格式（支持私聊和群聊）
message UnifiedMessage {
  string id = 1;               // 消息ID
  string type = 2;             // "private" 或 "group"
  string from_user_id = 3;     // 发送者ID
  string to_user_id = 4;       // 接收者ID（私聊）
  string group_id = 5;         // 群组ID（群聊）
  string content = 6;          // 消息内容
  int64 created_at = 7;        // 时间戳
  bool is_read = 8;            // 是否已读
  string stream_id = 9;        // Stream 消息ID（用于分页）
}
```

#### 1.2 Handler 实现

```go
// internal/message_service/handler/message.go

// PullConversationMessages 按会话拉取消息（优先 Stream，自动回退数据库）
func (h *MessageHandler) PullConversationMessages(ctx context.Context, req *pb.PullConversationMessagesRequest) (*pb.PullConversationMessagesResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	// 验证 conversation_id 格式
	if !strings.HasPrefix(req.ConversationId, "private:") && !strings.HasPrefix(req.ConversationId, "group:") {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid conversation_id format")
	}

	// 设置默认值
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	startID := req.StartId
	if startID == "" {
		startID = "+" // 从最新消息开始
	}

	var messages []*pb.UnifiedMessage
	var hasMore bool
	var nextStartID string

	// 优先从 Redis Stream 读取
	if req.UseStream {
		messages, hasMore, nextStartID = h.pullFromStream(ctx, userID, req.ConversationId, startID, limit)
	}

	// 如果 Stream 中消息不足，从数据库补充
	if int64(len(messages)) < limit {
		dbMessages, dbHasMore := h.pullFromDatabase(ctx, userID, req.ConversationId, int64(len(messages)), limit)
		messages = append(messages, dbMessages...)
		hasMore = dbHasMore
	}

	log.Printf("✅ User %s pulled %d messages from conversation %s", userID, len(messages), req.ConversationId)

	return &pb.PullConversationMessagesResponse{
		Code:        0,
		Message:     "Success",
		Messages:    messages,
		HasMore:     hasMore,
		NextStartId: nextStartID,
	}, nil
}

// pullFromStream 从 Redis Stream 读取消息
func (h *MessageHandler) pullFromStream(ctx context.Context, userID, conversationID, startID string, limit int64) ([]*pb.UnifiedMessage, bool, string) {
	streamKey := fmt.Sprintf("stream:private:%s", userID)

	// 使用 XREVRANGE 逆序读取（从新到旧）
	var messages []redis.XMessage
	var err error

	if startID == "+" {
		// 从最新开始
		messages, err = h.rdb.XRevRangeN(ctx, streamKey, "+", "-", limit).Result()
	} else {
		// 从指定ID开始（不包含该ID）
		messages, err = h.rdb.XRevRangeN(ctx, streamKey, fmt.Sprintf("(%s", startID), "-", limit).Result()
	}

	if err != nil {
		log.Printf("Failed to read from stream: %v", err)
		return nil, false, ""
	}

	// 过滤出该会话的消息
	var result []*pb.UnifiedMessage
	var lastStreamID string

	for _, msg := range messages {
		msgType, ok := msg.Values["type"].(string)
		if !ok {
			continue
		}

		// 匹配会话ID
		matched := false
		if strings.HasPrefix(conversationID, "private:") {
			peerID := conversationID[8:]
			if msgType == "private" {
				fromUserID := getString(msg.Values["from_user_id"])
				toUserID := getString(msg.Values["to_user_id"])
				matched = (fromUserID == peerID && toUserID == userID) || (fromUserID == userID && toUserID == peerID)
			}
		} else if strings.HasPrefix(conversationID, "group:") {
			groupID := conversationID[6:]
			if msgType == "group" && getString(msg.Values["group_id"]) == groupID {
				matched = true
			}
		}

		if matched {
			unifiedMsg := &pb.UnifiedMessage{
				Id:         getString(msg.Values["msg_id"]),
				Type:       msgType,
				FromUserId: getString(msg.Values["from_user_id"]),
				Content:    getString(msg.Values["content"]),
				CreatedAt:  getInt64(msg.Values["created_at"]),
				StreamId:   msg.ID,
			}

			if msgType == "private" {
				unifiedMsg.ToUserId = getString(msg.Values["to_user_id"])
			} else if msgType == "group" {
				unifiedMsg.GroupId = getString(msg.Values["group_id"])
			}

			result = append(result, unifiedMsg)
			lastStreamID = msg.ID

			if int64(len(result)) >= limit {
				break
			}
		}
	}

	// 判断是否还有更多
	hasMore := len(messages) == int(limit) && lastStreamID != ""

	return result, hasMore, lastStreamID
}

// pullFromDatabase 从数据库读取历史消息
func (h *MessageHandler) pullFromDatabase(ctx context.Context, userID, conversationID string, currentCount, limit int64) ([]*pb.UnifiedMessage, bool) {
	remaining := limit - currentCount
	if remaining <= 0 {
		return nil, false
	}

	var messages []*pb.UnifiedMessage

	if strings.HasPrefix(conversationID, "private:") {
		// 私聊消息
		peerID := conversationID[8:]
		query := `
			SELECT id, from_user_id, to_user_id, content, created_at, is_read
			FROM messages
			WHERE (from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)
			ORDER BY created_at DESC
			LIMIT ?`

		rows, err := h.db.QueryContext(ctx, query, userID, peerID, peerID, userID, remaining)
		if err != nil {
			log.Printf("Failed to query private messages: %v", err)
			return nil, false
		}
		defer rows.Close()

		for rows.Next() {
			var msg pb.UnifiedMessage
			msg.Type = "private"
			rows.Scan(&msg.Id, &msg.FromUserId, &msg.ToUserId, &msg.Content, &msg.CreatedAt, &msg.IsRead)
			messages = append(messages, &msg)
		}

	} else if strings.HasPrefix(conversationID, "group:") {
		// 群聊消息
		groupID := conversationID[6:]
		query := `
			SELECT id, from_user_id, group_id, content, created_at
			FROM group_messages
			WHERE group_id = ?
			ORDER BY created_at DESC
			LIMIT ?`

		rows, err := h.db.QueryContext(ctx, query, groupID, remaining)
		if err != nil {
			log.Printf("Failed to query group messages: %v", err)
			return nil, false
		}
		defer rows.Close()

		for rows.Next() {
			var msg pb.UnifiedMessage
			msg.Type = "group"
			rows.Scan(&msg.Id, &msg.FromUserId, &msg.GroupId, &msg.Content, &msg.CreatedAt)
			messages = append(messages, &msg)
		}
	}

	hasMore := int64(len(messages)) == remaining

	return messages, hasMore
}

// 辅助函数
func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	}
	return 0
}
```

#### 1.3 API Gateway 路由

```go
// cmd/api/main.go

protected := r.Group("/api/v1")
protected.Use(authMiddleware)
{
    // 新增：按会话拉取消息
    protected.GET("/conversations/:conversation_id/messages", userHandler.PullConversationMessages)
}
```

```go
// internal/api_gateway/handler/handler.go

// PullConversationMessages 按会话拉取消息
func (h *UserGatewayHandler) PullConversationMessages(c *gin.Context) {
	conversationID := c.Param("conversation_id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.ParseInt(limitStr, 10, 64)

	startID := c.DefaultQuery("start_id", "+")
	useStream := c.DefaultQuery("use_stream", "true") == "true"

	authHeader := c.GetHeader("Authorization")
	md := metadata.New(map[string]string{"authorization": authHeader})
	ctx := metadata.NewOutgoingContext(c.Request.Context(), md)

	req := &msgPb.PullConversationMessagesRequest{
		ConversationId: conversationID,
		Limit:          limit,
		StartId:        startID,
		UseStream:      useStream,
	}

	res, err := h.messageClient.PullConversationMessages(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
```

---

### 方案2：优化现有 `PullMessages` API

如果不想新增接口，可以优化现有 `PullMessages`：

```go
// 修改现有 PullMessages，增加 Stream 支持
func (h *MessageHandler) PullMessages(ctx context.Context, req *pb.PullMessagesRequest) (*pb.PullMessagesResponse, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}

	// 1. 先从 Redis Stream 读取最新消息（如果 offset = 0）
	var streamMessages []*pb.Message
	if req.Offset == 0 {
		streamKey := fmt.Sprintf("stream:private:%s", userID)
		messages, _ := h.rdb.XRevRangeN(ctx, streamKey, "+", "-", 50).Result()

		for _, msg := range messages {
			if msgType := getString(msg.Values["type"]); msgType == "private" {
				streamMessages = append(streamMessages, &pb.Message{
					Id:         getString(msg.Values["msg_id"]),
					FromUserId: getString(msg.Values["from_user_id"]),
					ToUserId:   getString(msg.Values["to_user_id"]),
					Content:    getString(msg.Values["content"]),
					CreatedAt:  getInt64(msg.Values["created_at"]),
				})
			}
		}
	}

	// 2. 从数据库读取历史消息
	query := `...` // 原有逻辑
	
	// 3. 合并去重
	messages := mergeAndDeduplicate(streamMessages, dbMessages)
	
	return &pb.PullMessagesResponse{
		Code:    0,
		Message: "Success",
		Msgs:    messages,
	}, nil
}
```

---

### 方案3：优化会话列表的 `getLastMessage`

#### 3.1 使用 Pipeline 批量查询

```go
// internal/api_gateway/handler/conversation.go

func (h *ConversationHandler) GetConversationList(c *gin.Context) {
	// ... 获取会话列表 ...
	
	// ✅ 使用 Pipeline 批量查询最后消息
	lastMessages := h.batchGetLastMessages(ctx, userID, conversations)
	
	for i, conv := range conversations {
		response := h.enrichConversationInfo(ctx, userID, conv)
		response.LastMessage = lastMessages[i]
		// ...
	}
}

// batchGetLastMessages 批量获取最后一条消息
func (h *ConversationHandler) batchGetLastMessages(ctx context.Context, userID string, convs []stream.ConversationItem) []string {
	streamKey := fmt.Sprintf("stream:private:%s", userID)
	
	// 一次性读取用户的所有消息（缓存）
	messages, err := h.rdb.XRevRangeN(ctx, streamKey, "+", "-", 100).Result()
	if err != nil {
		return make([]string, len(convs))
	}
	
	// 构建会话ID -> 最后消息的映射
	conversationMsgs := make(map[string]string)
	
	for _, msg := range messages {
		var conversationID string
		msgType := getString(msg.Values["type"])
		
		if msgType == "private" {
			peerID := getString(msg.Values["from_user_id"])
			if peerID == userID {
				peerID = getString(msg.Values["to_user_id"])
			}
			conversationID = fmt.Sprintf("private:%s", peerID)
		} else if msgType == "group" {
			conversationID = fmt.Sprintf("group:%s", getString(msg.Values["group_id"]))
		}
		
		// 记录该会话的第一条消息（即最新消息）
		if _, exists := conversationMsgs[conversationID]; !exists {
			conversationMsgs[conversationID] = truncateString(getString(msg.Values["content"]), 50)
		}
	}
	
	// 按顺序返回结果
	result := make([]string, len(convs))
	for i, conv := range convs {
		if lastMsg, ok := conversationMsgs[conv.ConversationID]; ok {
			result[i] = lastMsg
		} else {
			result[i] = ""
		}
	}
	
	return result
}
```

**优化效果**：
- ❌ 原来：10 个会话 = 10 次 Redis 查询（每次 20 条）
- ✅ 优化后：1 次 Redis 查询（100 条），内存中过滤

---

## 📊 性能对比

| 场景 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| **拉取会话消息** | 从数据库（5秒延迟） | 从 Stream（实时） | 🚀 100% 实时性 |
| **会话列表加载** | 10 次 Redis × 20 条 | 1 次 Redis × 100 条 | 🚀 90% 查询减少 |
| **群聊消息支持** | ❌ 不支持 | ✅ 支持 | ✅ 新功能 |
| **按会话过滤** | ❌ 前端自己过滤 | ✅ 后端直接过滤 | ✅ 网络传输减少 |

---

## 🚀 实施步骤

### Phase 1: 新增按会话拉取消息（1-2小时）
1. ✅ 修改 `message.proto`，新增 `PullConversationMessages` RPC
2. ✅ 生成 proto 文件：`cd api/proto && build.bat`
3. ✅ 实现 `PullConversationMessages` Handler
4. ✅ 添加 API Gateway 路由
5. ✅ 测试接口

### Phase 2: 优化会话列表查询（30分钟）
1. ✅ 修改 `batchGetLastMessages` 方法
2. ✅ 测试性能提升

### Phase 3: 优化现有 `PullMessages`（可选，1小时）
1. ✅ 增加 Redis Stream 支持
2. ✅ 实现消息合并去重

---

## 🧪 测试用例

### 测试1：拉取私聊会话消息

```bash
GET /api/v1/conversations/private:user_456/messages?limit=50&start_id=+
Authorization: Bearer <token>

# 预期响应
{
  "code": 0,
  "messages": [
    {
      "id": "msg_123",
      "type": "private",
      "from_user_id": "user_789",
      "to_user_id": "user_456",
      "content": "你好",
      "created_at": 1702512000000,
      "stream_id": "1702512000000-0"
    }
  ],
  "has_more": true,
  "next_start_id": "1702511000000-0"
}
```

### 测试2：拉取群聊会话消息

```bash
GET /api/v1/conversations/group:group_789/messages?limit=50
Authorization: Bearer <token>

# 预期响应
{
  "code": 0,
  "messages": [
    {
      "id": "msg_456",
      "type": "group",
      "from_user_id": "user_123",
      "group_id": "group_789",
      "content": "大家好",
      "created_at": 1702512000000
    }
  ],
  "has_more": false
}
```

### 测试3：验证实时性

```bash
# 1. 发送消息
POST /api/v1/messages/send
{"to_user_id": "user_456", "content": "测试消息"}

# 2. 立即拉取（不等待数据库写入）
GET /api/v1/conversations/private:user_456/messages?limit=1

# ✅ 预期：能立即看到刚发送的消息（从 Stream 读取）
```

---

## ⚠️ 注意事项

### 1. Stream 消息过期问题
Redis Stream 中的消息会定期清理（如 7 天），需要：
- ✅ 数据库作为永久存储
- ✅ 优先读取 Stream，自动回退数据库
- ✅ 前端分页时使用 `start_id` 或 `offset` 组合策略

### 2. 消息去重
Stream 和数据库可能有重复消息，需要：
- ✅ 使用 `msg_id` 去重
- ✅ 优先使用 Stream 中的消息（更新）

### 3. 分页策略
- **Stream 分页**：使用 `start_id`（Stream ID）
- **数据库分页**：使用 `offset`
- **混合分页**：先用完 Stream，再切换到数据库

---

## 📝 总结

| 优化项 | 优先级 | 难度 | 影响 |
|--------|--------|------|------|
| 新增按会话拉取消息 | 🔴 高 | ⭐⭐ | 🚀 实时性 + 功能完整性 |
| 优化会话列表查询 | 🟡 中 | ⭐ | 🚀 性能提升 90% |
| 优化现有 PullMessages | 🟢 低 | ⭐⭐ | 🚀 向后兼容 |

**推荐实施顺序**：
1️⃣ 新增 `PullConversationMessages` API（最重要）  
2️⃣ 优化会话列表的 `batchGetLastMessages`（最快见效）  
3️⃣ 优化现有 `PullMessages`（可选）

---

## 🎯 下一步行动

你希望我：
1. ✅ **立即实现方案1**（新增按会话拉取消息 API）
2. ✅ **优化方案2**（批量查询最后消息）
3. ❓ **只提供代码示例**（你自己实现）
4. ❓ **先测试现有逻辑**（找出更多问题）

请告诉我你的选择！🚀
