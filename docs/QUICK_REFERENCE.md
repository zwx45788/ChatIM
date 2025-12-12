# 友谊服务 - 快速参考卡

## 📌 服务信息

| 项目 | 值 |
|------|-----|
| **服务名** | FriendshipService |
| **gRPC 端口** | 50053 |
| **Proto 包** | ChatIM.friendship |
| **启动文件** | cmd/friendship/main.go |
| **配置字段** | friendship_grpc_port / friendship_grpc_addr |

## 🔌 API 方法速查

### 好友相关 (5 个方法)

```protobuf
// 发送好友请求
SendFriendRequest(
  to_user_id: string,
  message: string
) → request_id: string

// 获取好友请求列表
GetFriendRequests(
  status: int32,        // 0=pending, 1=accepted, 2=rejected, 3=cancelled
  limit: int64,         // 分页大小，默认20，最大100
  offset: int64         // 偏移量
) → [FriendRequest], total: int32

// 处理好友请求（接受/拒绝）
ProcessFriendRequest(
  request_id: string,
  accept: bool          // true=接受，false=拒绝
) → code: int32, message: string

// 获取好友列表
GetFriends(
  limit: int64,
  offset: int64
) → [Friend], total: int32

// 删除好友 ⭐ 新
RemoveFriend(
  friend_user_id: string
) → code: int32, message: string
```

### 群组相关 (6 个方法)

```protobuf
// 申请加入群组
SendGroupJoinRequest(
  group_id: string,
  message: string
) → request_id: string

// 获取群申请列表（仅群主/管理员）
GetGroupJoinRequests(
  group_id: string,
  status: int32,        // 0=pending, 1=accepted, 2=rejected, 3=cancelled
  limit: int64,
  offset: int64
) → [GroupJoinRequest], total: int32

// 处理群申请（仅群主/管理员）
ProcessGroupJoinRequest(
  request_id: string,
  accept: bool
) → code: int32, message: string

// 获取用户所在的所有群组 ⭐ 新
GetUserGroups(
  limit: int64,         // 分页大小，默认20，最大100
  offset: int64         // 偏移量
) → [GroupInfo], total: int32

// 退出群聊 ⭐ 新
LeaveGroup(
  group_id: string
) → code: int32, message: string

// 踢出群成员（仅群主）⭐ 新
RemoveGroupMember(
  group_id: string,
  member_user_id: string
) → code: int32, message: string
```

## 📦 关键数据结构

```go
// 好友请求
type FriendRequest struct {
    ID           string    // UUID
    FromUserID   string
    FromUsername string
    FromNickname string
    Message      string
    Status       string    // pending/accepted/rejected/cancelled
    CreatedAt    time.Time
}

// 好友关系
type Friend struct {
    UserID    string
    Username  string
    Nickname  string
    CreatedAt time.Time
}

// 群加入申请
type GroupJoinRequest struct {
    ID           string    // UUID
    GroupID      string
    FromUserID   string
    FromUsername string
    FromNickname string
    Message      string
    Status       string
    ReviewedBy   *string   // 审批者ID
    CreatedAt    time.Time
}

// 用户群组信息
type GroupInfo struct {
    GroupID     string
    GroupName   string
    Description string
    MemberCount int32     // 群组成员数
    CreatedAt   int64     // 创建时间戳
}
```

## 🗄️ 数据库表

### friend_requests
```sql
-- 好友申请记录
id (PK) | from_user_id (FK) | to_user_id (FK) | message | status | created_at | processed_at | updated_at
-- 索引: idx_to_user_status, idx_from_user, idx_created_at
-- 唯一约束: (from_user_id, to_user_id)
```

### friends
```sql
-- 好友关系（双向）
user_id_1 (PK, FK) | user_id_2 (PK, FK) | created_at
-- 索引: idx_user1, idx_user2
-- 约束: user_id_1 < user_id_2（规范化）
```

### group_join_requests
```sql
-- 群加入申请
id (PK) | group_id (FK) | from_user_id (FK) | message | status | reviewed_by (FK) | created_at | processed_at | updated_at
-- 索引: idx_group_status, idx_from_user, idx_created_at
-- 唯一约束: (group_id, from_user_id)
```

## 🔐 权限要求

| 操作 | 权限要求 |
|------|--------|
| SendFriendRequest | 已认证用户 |
| GetFriendRequests | 已认证用户（查看自己的请求） |
| ProcessFriendRequest | 请求接收者 |
| GetFriends | 已认证用户 |
| RemoveFriend | 已认证用户 |
| SendGroupJoinRequest | 已认证用户（非成员） |
| GetGroupJoinRequests | **群主/管理员** |
| ProcessGroupJoinRequest | **群主/管理员** |
| GetUserGroups | 已认证用户 |
| LeaveGroup | 已认证用户（必须在群中） |
| RemoveGroupMember | **群主专属** |

## 📝 状态转换

### 好友申请状态流转
```
pending → accepted ✓  (仅接收者)
       → rejected  ✓  (仅接收者)
       → cancelled ✓  (申请者)
```

### 群申请状态流转
```
pending → accepted ✓  (群主/管理员)
       → rejected  ✓  (群主/管理员)
       → cancelled ✓  (申请者)
```

## 🧪 测试命令

### 发送好友请求
```bash
grpcurl -plaintext \
  -d '{"to_user_id":"user_2","message":"加个好友？"}' \
  -H "authorization: Bearer <JWT_TOKEN>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/SendFriendRequest
```

### 获取好友请求
```bash
grpcurl -plaintext \
  -d '{"status":0,"limit":20,"offset":0}' \
  -H "authorization: Bearer <JWT_TOKEN>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/GetFriendRequests
```

### 处理好友请求
```bash
grpcurl -plaintext \
  -d '{"request_id":"<uuid>","accept":true}' \
  -H "authorization: Bearer <JWT_TOKEN>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/ProcessFriendRequest
```

### 删除好友 ⭐ 新
```bash
grpcurl -plaintext \
  -d '{"friend_user_id":"user_2"}' \
  -H "authorization: Bearer <JWT_TOKEN>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/RemoveFriend
```

### 退出群聊 ⭐ 新
```bash
grpcurl -plaintext \
  -d '{"group_id":"group_123"}' \
  -H "authorization: Bearer <JWT_TOKEN>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/LeaveGroup
```

### 踢出成员 ⭐ 新
```bash
grpcurl -plaintext \
  -d '{"group_id":"group_123","member_user_id":"user_to_remove"}' \
  -H "authorization: Bearer <JWT_TOKEN>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/RemoveGroupMember
```

## 🚀 启动服务

```bash
# 方式1：Windows 启动脚本
cd cmd
start.bat

# 方式2：直接运行
cd cmd/friendship
go run main.go

# 方式3：运行编译的二进制
cd cmd/friendship
./friendship.exe
```

## 📝 常见错误响应

| 错误 | 原因 | 解决方案 |
|------|------|--------|
| Unauthenticated | 缺少 JWT token | 检查 Authorization header |
| InvalidArgument | 参数无效或格式错误 | 检查请求参数 |
| AlreadyExists | 资源已存在 | 确保未重复操作 |
| NotFound | 资源不存在 | 检查 ID 是否正确 |
| PermissionDenied | 无权执行操作 | 检查是否为群主/管理员 |
| Internal | 服务器错误 | 查看服务日志 |

## 🔗 调用示例 (Go 客户端)

```go
import "ChatIM/pkg/clients"

// 初始化客户端
client, _ := clients.NewFriendshipClient("localhost:50053")
defer client.Close()

// 发送好友请求
requestID, _ := client.SendFriendRequest(ctx, "user_2", "加好友")

// 获取好友请求
requests, total, _ := client.GetFriendRequests(
    ctx, 
    0,      // status: pending
    20,     // limit
    0,      // offset
)

// 接受好友请求
client.ProcessFriendRequest(ctx, requestID, true)

// 获取好友列表
friends, total, _ := client.GetFriends(ctx, 20, 0)

// 删除好友 ⭐ 新
client.RemoveFriend(ctx, "user_2")

// 获取用户的群组 ⭐ 新
groups, total, _ := client.GetUserGroups(ctx, 20, 0)

// 退出群聊 ⭐ 新
client.LeaveGroup(ctx, "group_123")

// 踢出群成员（仅群主）⭐ 新
client.RemoveGroupMember(ctx, "group_123", "user_to_remove")
```

## 📚 文档导航

| 文档 | 位置 | 内容 |
|------|------|------|
| **API 文档** | docs/FRIENDSHIP_SERVICE.md | 完整 API 说明、工作流程 |
| **部署指南** | docs/FRIENDSHIP_DEPLOYMENT.md | 部署、监控、故障排除 |
| **实现总结** | docs/IMPLEMENTATION_SUMMARY.md | 架构、性能、安全 |
| **完成清单** | docs/COMPLETION_CHECKLIST.md | 项目完成状态 |

## 🎯 关键特性

- ✅ **事务支持**: 接受请求时原子更新状态和关系
- ✅ **权限验证**: 完整的认证和授权检查
- ✅ **分页查询**: 所有列表查询都支持分页
- ✅ **防重复**: 检查重复申请和自己加自己
- ✅ **关系规范化**: 好友关系 ID 自动规范化避免重复
- ✅ **时间戳**: 完整的创建和处理时间记录
- ✅ **日志记录**: 所有操作都有详细日志

## 🔍 性能指标（预期）

| 操作 | 响应时间 | 备注 |
|------|---------|------|
| 发送请求 | < 50ms | UUID 生成 + 单条插入 |
| 获取列表 | < 50ms | 索引查询 + 分页 |
| 接受请求 | < 100ms | 事务处理 + 2 条 SQL |
| 删除好友 | < 50ms | 索引删除 |
| 退出群聊 | < 50ms | 成员检查 + 删除 |
| 踢出成员 | < 50ms | 权限检查 + 删除 |

---

**最后更新**: 2024年12月
**状态**: ✅ 生产就绪
**RPC 方法总数**: 11 个（含 3 个新增删除/移除功能）

