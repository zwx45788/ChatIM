# WebSocket 实时消息推送测试指南

## 📋 功能说明

系统现在支持通过 WebSocket 实时推送消息，无需轮询。当用户发送消息（私聊或群聊）时，接收方如果在线，将立即通过 WebSocket 收到消息推送。

---

## 🏗️ 架构流程

```
发送者 → SendMessage API
    ↓
1. 写入 Redis Stream (stream:private:{to_user_id})
2. 发布 Redis 通知 (message_notifications)
3. 异步写入数据库
    ↓
WebSocket Subscriber 监听 Redis 通知
    ↓
解析通知数据 → 推送给在线用户
    ↓
接收者的 WebSocket 连接 → 收到消息
```

---

## 🧪 测试准备

### 1. 启动服务

确保以下服务正常运行：

```bash
# 启动 MySQL
docker-compose up -d mysql

# 启动 Redis
docker-compose up -d redis

# 启动 Message Service
cd cmd/message
go run main.go

# 启动 API Gateway (包含 WebSocket)
cd cmd/api
go run main.go
```

### 2. 获取测试用户 Token

```bash
# 用户 A 登录
curl -X POST http://localhost:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user_a",
    "password": "password123"
  }'

# 用户 B 登录
curl -X POST http://localhost:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user_b",
    "password": "password123"
  }'
```

保存返回的 `token` 和 `user_id`。

---

## 🔌 WebSocket 连接测试

### 方法一：使用浏览器控制台

1. 打开浏览器开发者工具（F12）
2. 进入 Console 标签页
3. 粘贴以下代码：

```javascript
// 用户 B 连接 WebSocket
const token = "YOUR_TOKEN_HERE"; // 替换为实际 token
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

ws.onopen = () => {
  console.log("✅ WebSocket 连接成功");
};

ws.onmessage = (event) => {
  console.log("📨 收到新消息:", event.data);
  const message = JSON.parse(event.data);
  console.log("消息详情:", message);
};

ws.onerror = (error) => {
  console.error("❌ WebSocket 错误:", error);
};

ws.onclose = () => {
  console.log("🔌 WebSocket 连接已关闭");
};
```

### 方法二：使用 Node.js 脚本

创建 `test_websocket.js`：

```javascript
const WebSocket = require('ws');

const token = "YOUR_TOKEN_HERE"; // 替换为实际 token
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

ws.on('open', () => {
  console.log('✅ WebSocket 连接成功');
});

ws.on('message', (data) => {
  console.log('📨 收到新消息:', data.toString());
  const message = JSON.parse(data.toString());
  console.log('消息详情:', JSON.stringify(message, null, 2));
});

ws.on('error', (error) => {
  console.error('❌ WebSocket 错误:', error);
});

ws.on('close', () => {
  console.log('🔌 WebSocket 连接已关闭');
});
```

运行：
```bash
npm install ws
node test_websocket.js
```

### 方法三：使用 HTML 页面

创建 `test_websocket.html`：

```html
<!DOCTYPE html>
<html>
<head>
    <title>WebSocket 测试</title>
    <style>
        body { font-family: Arial; padding: 20px; }
        #messages { border: 1px solid #ccc; padding: 10px; height: 300px; overflow-y: auto; margin-top: 10px; }
        .message { margin: 5px 0; padding: 5px; background: #f0f0f0; border-radius: 3px; }
        .private { background: #e3f2fd; }
        .group { background: #fff3e0; }
        input, button { padding: 8px; margin: 5px; }
    </style>
</head>
<body>
    <h2>WebSocket 实时消息测试</h2>
    
    <div>
        <label>Token:</label>
        <input type="text" id="token" placeholder="输入你的 token" style="width: 400px;">
        <button onclick="connect()">连接</button>
        <button onclick="disconnect()">断开</button>
    </div>
    
    <div id="status">状态: 未连接</div>
    
    <div id="messages"></div>

    <script>
        let ws = null;

        function connect() {
            const token = document.getElementById('token').value;
            if (!token) {
                alert('请输入 token');
                return;
            }

            ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

            ws.onopen = () => {
                document.getElementById('status').textContent = '状态: ✅ 已连接';
                addMessage('系统', '✅ WebSocket 连接成功', 'system');
            };

            ws.onmessage = (event) => {
                const message = JSON.parse(event.data);
                addMessage(
                    message.from_user_id, 
                    message.content, 
                    message.type,
                    message
                );
            };

            ws.onerror = (error) => {
                document.getElementById('status').textContent = '状态: ❌ 连接错误';
                console.error('WebSocket 错误:', error);
            };

            ws.onclose = () => {
                document.getElementById('status').textContent = '状态: 🔌 连接已关闭';
                addMessage('系统', '🔌 WebSocket 连接已关闭', 'system');
            };
        }

        function disconnect() {
            if (ws) {
                ws.close();
                ws = null;
            }
        }

        function addMessage(from, content, type, fullMessage) {
            const messagesDiv = document.getElementById('messages');
            const messageDiv = document.createElement('div');
            messageDiv.className = `message ${type}`;
            
            const time = new Date().toLocaleTimeString();
            let details = '';
            if (fullMessage) {
                details = `<br><small>完整数据: ${JSON.stringify(fullMessage)}</small>`;
            }
            
            messageDiv.innerHTML = `
                <strong>[${time}] ${from}:</strong> ${content}
                ${details}
            `;
            messagesDiv.appendChild(messageDiv);
            messagesDiv.scrollTop = messagesDiv.scrollHeight;
        }
    </script>
</body>
</html>
```

在浏览器中打开此文件。

---

## 📤 发送测试消息

### 私聊消息测试

在另一个终端，使用用户 A 的 token 发送消息给用户 B：

```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Authorization: Bearer USER_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "to_user_id": "USER_B_ID",
    "content": "你好，这是一条测试消息！"
  }'
```

**预期结果**：
- 用户 B 的 WebSocket 连接立即收到消息
- 消息格式：
```json
{
  "type": "private",
  "id": "message-uuid",
  "from_user_id": "user_a_id",
  "to_user_id": "user_b_id",
  "content": "你好，这是一条测试消息！",
  "created_at": 1702540800
}
```

### 群聊消息测试

```bash
curl -X POST http://localhost:8080/api/v1/groups/GROUP_ID/messages \
  -H "Authorization: Bearer USER_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "大家好，这是一条群聊消息！"
  }'
```

**预期结果**：
- 群内所有在线成员（除了发送者）的 WebSocket 连接立即收到消息
- 消息格式：
```json
{
  "type": "group",
  "id": "message-uuid",
  "group_id": "group_id",
  "from_user_id": "user_a_id",
  "content": "大家好，这是一条群聊消息！",
  "created_at": 1702540800
}
```

---

## 🐛 故障排查

### 1. WebSocket 连接失败

**检查项**：
- ✅ API Gateway 是否正常运行
- ✅ Token 是否有效且未过期
- ✅ WebSocket URL 是否正确（ws://localhost:8080/ws）

**查看日志**：
```bash
# API Gateway 日志
tail -f logs/api_gateway.log

# 应该看到类似：
# Client {user_id} connected
```

### 2. 收不到消息推送

**检查项**：
- ✅ WebSocket 是否已连接
- ✅ Redis 是否正常运行
- ✅ Subscriber 是否正常启动

**查看日志**：
```bash
# Message Service 日志
# 发送消息时应该看到：
# ✅ Notification published for message {msg_id} to user {user_id}

# API Gateway 日志
# 应该看到：
# 📨 Message notification: {...}
# ✅ Message pushed to user {user_id} via WebSocket
```

**手动测试 Redis**：
```bash
# 连接到 Redis
redis-cli

# 订阅消息通知频道
SUBSCRIBE message_notifications

# 在另一个终端发送消息，应该能看到通知
```

### 3. 消息格式错误

**检查点**：
- 确认 `internal/message_service/handler/message.go` 中的通知格式
- 确认 `internal/websocket/subscriber.go` 中的解析逻辑

### 4. 用户离线时的行为

**正常行为**：
- 离线用户不会收到 WebSocket 推送（正常）
- 消息已保存到 Redis Stream 和数据库
- 用户上线后可以通过 `PullMessages` 拉取历史消息

---

## ✅ 测试检查清单

- [ ] 用户 A 和用户 B 都能成功连接 WebSocket
- [ ] 用户 A 发送消息，用户 B 立即收到推送
- [ ] 用户 B 发送消息，用户 A 立即收到推送
- [ ] 群聊消息能推送给所有在线成员
- [ ] 发送者不会收到自己的消息推送
- [ ] 离线用户不影响在线用户接收消息
- [ ] 消息格式正确，包含所有必要字段
- [ ] WebSocket 断开重连后仍能正常接收消息
- [ ] 日志中显示正确的推送成功信息

---

## 📊 性能测试

### 并发连接测试

使用 `goroutine` 或其他工具模拟多个并发 WebSocket 连接：

```go
package main

import (
    "fmt"
    "log"
    "time"
    "github.com/gorilla/websocket"
)

func main() {
    tokens := []string{
        "token1", "token2", "token3", // ... 添加更多 token
    }

    for i, token := range tokens {
        go func(idx int, tk string) {
            url := fmt.Sprintf("ws://localhost:8080/ws?token=%s", tk)
            ws, _, err := websocket.DefaultDialer.Dial(url, nil)
            if err != nil {
                log.Printf("User %d failed to connect: %v", idx, err)
                return
            }
            defer ws.Close()

            log.Printf("User %d connected", idx)

            for {
                _, message, err := ws.ReadMessage()
                if err != nil {
                    log.Printf("User %d read error: %v", idx, err)
                    return
                }
                log.Printf("User %d received: %s", idx, string(message))
            }
        }(i, token)
    }

    // 保持主线程运行
    select {}
}
```

---

## 🎯 预期性能指标

- WebSocket 连接延迟：< 100ms
- 消息推送延迟：< 50ms
- 单服务器支持并发连接：1000+ （取决于服务器配置）
- Redis 通知发布延迟：< 10ms

---

## 📝 注意事项

1. **Token 过期处理**：Token 过期时 WebSocket 会自动断开，客户端需要重新登录并连接

2. **重连机制**：客户端应实现自动重连逻辑，处理网络波动

3. **消息去重**：客户端应基于 `message_id` 进行消息去重

4. **离线消息**：WebSocket 只推送实时消息，历史消息需要通过 HTTP API 拉取

5. **心跳检测**：建议客户端每 30 秒发送一次 ping 保持连接活跃

---

**最后更新**：2025年12月14日
