# ChatIM API 参考文档

**Base URL**: `http://localhost:8080/api/v1`

**认证方式**: JWT Token (除注册/登录外的所有接口需要在 Header 中携带 `Authorization: Bearer {token}`)

---

## 1. 用户管理

### 1.1 用户注册
```
POST /users
Content-Type: application/json

Request:
{
  "username": "string",    // 用户名
  "password": "string",    // 密码
  "nickname": "string"     // 昵称
}

Response:
{
  "code": 0,              // 0=成功, 非0=失败
  "message": "string",
  "user_id": "string"     // 用户ID
}
```

### 1.2 用户登录
```
POST /login
Content-Type: application/json

Request:
{
  "username": "string",
  "password": "string"
}

Response:
{
  "code": 0,
  "message": "string",
  "token": "string",       // JWT Token
  "private_unreads": [...],  // 私聊未读消息列表（可能为空）
  "private_unread_count": 0,
  "group_unreads": [...],    // 群聊未读概览（可能为空）
  "group_unread_count": 0,
  "total_unread_count": 0
}
```

### 1.3 获取当前用户信息
```
GET /users/me
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "data": {
    "user_id": "string",
    "username": "string",
    "nickname": "string"
  }
}
```

### 1.4 获取用户详情
```
GET /users/{user_id}

Response:
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": "string",
    "username": "string",
    "nickname": "string"
  }
}
```

### 1.5 检查用户在线状态
```
GET /users/{user_id}/online

Response:
{
  "code": 0,
  "message": "string",
  "is_online": true
}
```

说明：`{user_id}` 既可以传用户ID，也可以直接传 username（服务端会做兼容处理）。

---

## 2. 消息管理

### 2.1 发送私聊消息
```
POST /messages/send
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "to_user_id": "string",     // 接收者ID
  "content": "string"         // 消息内容
}

Response:
{
  "code": 0,
  "message": "string",
  "msg": {
    "id": "string",
    "from_user_id": "string",
    "to_user_id": "string",
    "content": "string",
    "created_at": 0,
    "is_read": false,
    "read_at": 0
  }
}
```

### 2.2 拉取消息（按会话分组，支持私聊 + 群聊）
```
GET /messages?limit=20&auto_mark=false&include_read=false
Authorization: Bearer {token}

Query Parameters:
- limit: 每个会话最多拉取消息数（默认20，最大100）
- auto_mark: 是否自动标记为已读（默认false）
- include_read: 是否包含已读消息（默认false，仅返回未读）

Response:
{
  "code": 0,
  "message": "string",
  "conversations": [
    {
      "conversation_id": "private:{user_id}" | "group:{group_id}",
      "type": "private" | "group",
      "peer_id": "string",
      "peer_name": "string",
      "peer_avatar": "string",
      "unread_count": 0,
      "last_message_time": 0,
      "messages": [
        {
          "id": "string",
          "type": "private" | "group",
          "from_user_id": "string",
          "from_user_name": "string",
          "to_user_id": "string",
          "group_id": "string",
          "content": "string",
          "created_at": 0,
          "is_read": false,
          "stream_id": "string"
        }
      ]
    }
  ],
  "total_unread": 0,
  "conversation_count": 0
}
```

### 2.3 获取未读消息数
```
GET /messages/unread
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "unread_count": 0
}
```

### 2.4 拉取未读消息
```
GET /messages/unread/pull?limit=100&auto_mark=true
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "msgs": [
    {
      "id": "string",
      "from_user_id": "string",
      "to_user_id": "string",
      "content": "string",
      "created_at": 0,
      "is_read": false,
      "read_at": 0
    }
  ],
  "total_unread": 0,
  "has_more": false
}
```

### 2.5 拉取所有未读消息（含群聊）
```
GET /unread/all
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "private_unreads": [...],     // 私聊未读消息
  "group_unreads": {...},       // 群聊未读消息（map 结构，key 为 group_id）
  "total_unread_count": 0       // 总未读数
}
```

---

## 3. 群组管理

### 3.1 创建群组
```
POST /groups
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "name": "string",              // 群名称
  "description": "string",       // 群描述
  "member_ids": ["string"]       // 初始成员ID列表（可选）
}

Response:
{
  "code": 0,
  "message": "string",
  "group_id": "string"
}
```

### 3.2 获取群组信息
```
GET /groups/{group_id}
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "group": {
    "id": "string",
    "name": "string",
    "description": "string",
    "creator_id": "string",
    "created_at": 0,
    "member_count": 0
  }
}
```

### 3.3 获取我的群组列表
```
GET /groups
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "groups": [
    {
      "id": "string",
      "name": "string",
      "description": "string",
      "creator_id": "string",
      "created_at": 0,
      "member_count": 0
    }
  ],
  "total": 0
}
```

### 3.4 添加群成员
```
POST /groups/{group_id}/members
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "user_ids": ["string"]      // 用户ID数组
}

Response:
{
  "code": 0,
  "message": "string",
  "added_count": 0
}
```

### 3.5 移除群成员
```
DELETE /groups/{group_id}/members
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "group_id": "string",
  "user_ids": ["string"]
}

Response:
{
  "code": 0,
  "message": "string",
  "removed_count": 0
}
```

### 3.6 退出群组
```
DELETE /groups/{group_id}
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 3.7 发送群消息
```
POST /groups/messages
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "group_id": "string",
  "content": "string"
}

Response:
{
  "code": 0,
  "message": "string",
  "msg": {
    "id": "string",
    "group_id": "string",
    "from_user_id": "string",
    "content": "string",
    "created_at": 0
  }
}
```

### 3.8 获取群成员列表
```
GET /groups/{group_id}/members?limit=50&offset=0
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "members": [
    {
      "user_id": "string",
      "username": "string",
      "nickname": "string",
      "role": "admin" | "member",
      "joined_at": 0
    }
  ],
  "total": 0
}
```

---

## 4. 群加入请求

### 4.1 发送加群申请
```
POST /groups/join-requests
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "group_id": "string",
  "message": "string"         // 申请理由
}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 4.2 处理加群申请
```
POST /groups/join-requests/handle
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "request_id": "string",
  "action": 1                  // 1=接受, 2=拒绝
}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 4.3 获取群的加入申请列表（管理员）
```
GET /groups/{group_id}/join-requests?status=0&limit=20&offset=0
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "requests": [
    {
      "id": "string",
      "group_id": "string",
      "from_user_id": "string",
      "from_username": "string",
      "message": "string",
      "status": "pending",      // pending/accepted/rejected/cancelled
      "reviewed_by": "string",
      "created_at": 0,
      "processed_at": 0
    }
  ],
  "total": 0
}
```

### 4.4 获取我的加入申请列表
```
GET /groups/join-requests/my?status=0&limit=20&offset=0
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "requests": [...],
  "total": 0
}
```

---

## 5. 群组管理功能

### 5.1 修改群信息
```
PUT /groups/{group_id}/info
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "name": "string",           // 可选
  "description": "string",    // 可选
  "avatar": "string"          // 可选
}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 5.2 转让群主
```
POST /groups/{group_id}/transfer
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "new_owner_id": "string"
}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 5.3 解散群组
```
POST /groups/{group_id}/dismiss
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 5.4 设置/取消管理员
```
POST /groups/{group_id}/admin
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "user_id": "string",
  "is_admin": true            // true=设置为管理员, false=取消管理员
}

Response:
{
  "code": 0,
  "message": "string"
}
```

---

## 6. 搜索功能

### 6.1 搜索用户
```
GET /search/users?keyword={keyword}&limit=20&offset=0
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "users": [
    {
      "id": "string",
      "username": "string",
      "nickname": "string",
      "avatar": "string"
    }
  ],
  "total": 0
}
```

### 6.2 搜索群组
```
GET /search/groups?keyword={keyword}&limit=20&offset=0
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "groups": [
    {
      "id": "string",
      "name": "string",
      "description": "string",
      "avatar": "string",
      "member_count": 0
    }
  ],
  "total": 0
}
```

---

## 7. 文件上传

### 7.1 获取上传签名
```
GET /upload/signature?type=image
Authorization: Bearer {token}

Query Parameters:
- type: image 或 file

Response:
{
  "code": 0,
  "message": "string",
  "data": {
    "accessKeyId": "string",
    "policy": "string",
    "signature": "string",
    "host": "string",          // OSS地址
    "key": "string",           // 文件路径
    "expire": 1234567890       // 过期时间戳
  }
}

使用方式:
1. 获取签名后，构建FormData:
   - key: data.key
   - OSSAccessKeyId: data.accessKeyId
   - policy: data.policy
   - signature: data.signature
   - file: 文件对象
2. POST到 data.host
3. 上传成功后，文件URL为: data.host + '/' + data.key
```

---

## 8. 好友管理

### 8.1 发送好友请求
```
POST /friends/requests
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "to_user_id": "string",
  "message": "string"      // 可选备注
}

Response:
{
  "code": 0,
  "message": "string",
  "request_id": "string"
}
```

### 8.2 获取好友请求列表
```
GET /friends/requests?status=pending&limit=20&offset=0
Authorization: Bearer {token}

Query Parameters:
- status: pending | approved | rejected

Response:
{
  "code": 0,
  "message": "string",
  "requests": [
    {
      "id": "string",
      "from_user_id": "string",
      "from_username": "string",
      "from_nickname": "string",
      "message": "string",
      "status": 0,
      "created_at": 0
    }
  ],
  "total": 0
}
```

### 8.3 处理好友请求
```
POST /friends/requests/handle
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "request_id": "string",
  "accept": true
}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 8.4 获取好友列表
```
GET /friends
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "data": [
    {
      "user_id": "string",
      "username": "string",
      "nickname": "string",
      "created_at": 0
    }
  ]
}
```

---

## 9. 会话管理

### 9.1 获取会话列表
```
GET /conversations
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string",
  "conversations": [
    {
      "conversation_id": "private:{user_id}" | "group:{group_id}",
      "type": "private" | "group",
      "peer_id": "string",
      "title": "string",
      "avatar": "string",
      "last_message": "string",
      "last_message_time": 0,
      "unread_count": 0,
      "is_pinned": false
    }
  ],
  "total": 0,
  "has_more": false
}
```

### 9.2 置顶会话
```
POST /conversations/{conversation_id}/pin
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 9.3 取消置顶
```
DELETE /conversations/{conversation_id}/pin
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 9.4 删除会话
```
DELETE /conversations/{conversation_id}
Authorization: Bearer {token}

Response:
{
  "code": 0,
  "message": "string"
}
```

### 9.5 创建会话
```
POST /conversations
Authorization: Bearer {token}
Content-Type: application/json

Request:
{
  "conversation_id": "string"   // 格式: "private:{user_id}" 或 "group:{group_id}"
}

Response:
{
  "code": 0,
  "message": "string"
}
```

---

## 10. WebSocket 实时通信

### 10.1 连接 WebSocket
```
WebSocket URL: ws://localhost:8080/ws?token={token}

连接建立后会收到实时消息推送:
{
  "type": "private" | "group",
  "id": "string",
  "from_user_id": "string",
  "to_user_id": "string",      // private消息
  "group_id": "string",        // group消息
  "content": "string",
  "created_at": "string"
}
```

---

## 11. 运维与调试

### 11.1 Prometheus Metrics
```
GET http://localhost:9090/metrics
```

### 11.2 CPU 压力测试
```
GET /debug/cpu-burn?seconds=10&workers=0

Query Parameters:
- seconds: 持续秒数（默认10）
- workers: 并发 worker 数，0 表示使用 CPU 核数

Response:
{
  "workers": 0,
  "seconds": 10,
  "ops": 0,
  "gomaxprocs": 0,
  "num_cpu": 0,
  "finished_at": "2025-01-01T00:00:00Z"
}
```

---

## 通用说明

### 响应状态码
- `code: 0` - 成功
- `code: 1001` - 参数错误
- `code: 1002` - 数据库错误
- `code: 1003` - 权限不足
- `code: 1004` - 资源不存在

### 分页参数
- `limit` - 每页数量，默认20，最大100
- `offset` - 偏移量，默认0

### 时间格式
所有时间字段使用 ISO 8601 格式或 Unix 时间戳
