# 群加入请求功能 API 文档

## 功能概述

群加入请求功能允许用户申请加入群组，群管理员可以接受或拒绝申请。

## 数据库表结构

### group_join_requests 表

```sql
CREATE TABLE group_join_requests (
    id VARCHAR(36) PRIMARY KEY,
    group_id VARCHAR(36) NOT NULL,
    from_user_id VARCHAR(36) NOT NULL,
    message TEXT,
    status ENUM('pending', 'accepted', 'rejected', 'cancelled') DEFAULT 'pending',
    reviewed_by VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP NULL,
    FOREIGN KEY (group_id) REFERENCES groups(id),
    FOREIGN KEY (from_user_id) REFERENCES users(id),
    FOREIGN KEY (reviewed_by) REFERENCES users(id),
    INDEX idx_group_status (group_id, status),
    INDEX idx_from_user (from_user_id)
);
```

## API 接口

### 1. 发送加群申请

**接口**: `POST /api/v1/groups/join-requests`

**权限**: 需要登录

**请求体**:
```json
{
  "group_id": "群组ID",
  "message": "申请理由（可选）"
}
```

**响应**:
```json
{
  "code": 0,
  "message": "加群申请已发送",
  "request_id": "申请ID"
}
```

**错误码**:
- `AlreadyExists`: 已经是群成员或已有待处理申请
- `NotFound`: 群组不存在

**示例**:
```bash
curl -X POST http://localhost:8080/api/v1/groups/join-requests \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "group_id": "group-123",
    "message": "希望加入贵群学习交流"
  }'
```

---

### 2. 处理加群申请（管理员）

**接口**: `POST /api/v1/groups/join-requests/handle`

**权限**: 需要登录，且必须是群管理员

**请求体**:
```json
{
  "request_id": "申请ID",
  "action": 1  // 1: 接受, 2: 拒绝
}
```

**响应**:
```json
{
  "code": 0,
  "message": "申请已接受"  // 或 "申请已拒绝"
}
```

**错误码**:
- `NotFound`: 申请不存在
- `PermissionDenied`: 不是管理员或不是群成员
- `FailedPrecondition`: 申请已被处理

**示例**:
```bash
# 接受申请
curl -X POST http://localhost:8080/api/v1/groups/join-requests/handle \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "request_id": "req-123",
    "action": 1
  }'

# 拒绝申请
curl -X POST http://localhost:8080/api/v1/groups/join-requests/handle \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "request_id": "req-123",
    "action": 2
  }'
```

---

### 3. 获取群的加入申请列表（管理员）

**接口**: `GET /api/v1/groups/:group_id/join-requests`

**权限**: 需要登录，且必须是群管理员

**查询参数**:
- `status`: 申请状态筛选（可选）
  - `0`: 全部（默认）
  - `1`: 待处理
  - `2`: 已接受
  - `3`: 已拒绝
- `limit`: 每页数量，默认 20
- `offset`: 偏移量，默认 0

**响应**:
```json
{
  "code": 0,
  "message": "查询成功",
  "requests": [
    {
      "id": "申请ID",
      "group_id": "群组ID",
      "from_user_id": "申请人ID",
      "from_username": "申请人用户名",
      "message": "申请理由",
      "status": "pending",  // pending/accepted/rejected
      "reviewed_by": "审核人ID（可选）",
      "created_at": 1234567890,
      "processed_at": 1234567890  // 处理时间（可选）
    }
  ],
  "total": 5
}
```

**示例**:
```bash
# 查询所有申请
curl -X GET "http://localhost:8080/api/v1/groups/group-123/join-requests" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 只查询待处理申请
curl -X GET "http://localhost:8080/api/v1/groups/group-123/join-requests?status=1" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

### 4. 获取我的加入申请列表

**接口**: `GET /api/v1/groups/join-requests/my`

**权限**: 需要登录

**查询参数**:
- `status`: 申请状态筛选（可选）
  - `0`: 全部（默认）
  - `1`: 待处理
  - `2`: 已接受
  - `3`: 已拒绝
- `limit`: 每页数量，默认 20
- `offset`: 偏移量，默认 0

**响应**:
```json
{
  "code": 0,
  "message": "查询成功",
  "requests": [
    {
      "id": "申请ID",
      "group_id": "群组ID",
      "from_username": "群名称（复用字段）",
      "from_user_id": "申请人ID",
      "message": "申请理由",
      "status": "pending",
      "reviewed_by": "审核人ID（可选）",
      "created_at": 1234567890,
      "processed_at": 1234567890
    }
  ],
  "total": 3
}
```

**示例**:
```bash
# 查询我的所有申请
curl -X GET "http://localhost:8080/api/v1/groups/join-requests/my" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 只查询待处理的申请
curl -X GET "http://localhost:8080/api/v1/groups/join-requests/my?status=1" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## 业务逻辑说明

### 发送申请时的检查

1. ✅ 验证群组是否存在
2. ✅ 检查用户是否已经是群成员
3. ✅ 检查是否已有待处理的申请
4. ✅ 创建申请记录（状态为 pending）

### 处理申请时的检查

1. ✅ 验证申请是否存在
2. ✅ 验证申请状态是否为 pending
3. ✅ 验证处理者是否是群管理员
4. ✅ 更新申请状态和审核信息
5. ✅ 如果接受，添加用户到群组

### 权限控制

- **发送申请**: 任何登录用户
- **处理申请**: 只有群管理员（role = 'admin'）
- **查看群申请列表**: 只有群管理员
- **查看个人申请列表**: 只能查看自己的申请

---

## 实现文件

### 后端实现

1. **Proto 定义**: `api/proto/group/group.proto`
   - `SendGroupJoinRequestRequest/Response`
   - `HandleGroupJoinRequestRequest/Response`
   - `GetGroupJoinRequestsRequest/Response`
   - `GetMyGroupJoinRequestsRequest/Response`
   - `GroupJoinRequest` 消息类型

2. **gRPC 服务**: `internal/group_service/handler/group.go`
   - `SendGroupJoinRequest()`
   - `HandleGroupJoinRequest()`
   - `GetGroupJoinRequests()`
   - `GetMyGroupJoinRequests()`

3. **API 网关**: `internal/api_gateway/handler/handler.go`
   - `SendGroupJoinRequest()`
   - `HandleGroupJoinRequest()`
   - `GetGroupJoinRequests()`
   - `GetMyGroupJoinRequests()`

4. **路由配置**: `cmd/api/main.go`
   - 添加了 4 个新的受保护路由

---

## 测试场景

### 场景 1: 正常申请流程

1. **用户 A 申请加入群组**
   ```bash
   POST /api/v1/groups/join-requests
   {
     "group_id": "group-001",
     "message": "我想加入这个群"
   }
   ```
   预期: 返回申请 ID，状态为 pending

2. **管理员查看申请列表**
   ```bash
   GET /api/v1/groups/group-001/join-requests?status=1
   ```
   预期: 返回待处理申请列表

3. **管理员接受申请**
   ```bash
   POST /api/v1/groups/join-requests/handle
   {
     "request_id": "req-xxx",
     "action": 1
   }
   ```
   预期: 申请状态变为 accepted，用户 A 成为群成员

4. **用户 A 查看自己的申请**
   ```bash
   GET /api/v1/groups/join-requests/my
   ```
   预期: 显示申请已被接受

---

### 场景 2: 重复申请检测

1. 用户 A 发送加群申请（pending）
2. 用户 A 再次申请同一个群
   
   预期: 返回错误 "已发送过申请，请等待处理"

---

### 场景 3: 权限控制

1. 普通成员尝试处理申请
   
   预期: 返回错误 "只有管理员才能处理申请"

2. 非群成员尝试查看群的申请列表
   
   预期: 返回错误 "您不是群成员"

---

## 状态流转图

```
      [用户申请]
          ↓
      pending (待处理)
          ↓
    ┌─────┴─────┐
    ↓           ↓
accepted    rejected
(已接受)     (已拒绝)
    ↓
[加入群组]
```

---

## 注意事项

1. ⚠️ 申请一旦处理（accepted/rejected），就无法再修改
2. ⚠️ 只有管理员（role='admin'）才能处理申请
3. ⚠️ 同一个用户对同一个群只能有一个 pending 状态的申请
4. ⚠️ 接受申请后，用户会自动以 'member' 角色加入群组
5. ⚠️ 申请被处理后会记录 `reviewed_by` 和 `processed_at`

---

## 未来优化建议

1. 🔔 **实时通知**: 当申请被处理时，通过 WebSocket 通知申请人
2. 🔔 **管理员通知**: 有新申请时通知群管理员
3. ⏰ **自动过期**: pending 状态的申请超过一定时间（如 7 天）自动标记为 cancelled
4. 📊 **统计功能**: 统计群的申请通过率、待处理数量等
5. 🚫 **黑名单**: 支持管理员拉黑某些用户，禁止其申请加入

---

## 版本历史

- **v1.0** (2024-01-XX): 初始实现
  - ✅ 发送加群申请
  - ✅ 处理加群申请
  - ✅ 查询申请列表
  - ✅ 权限控制
