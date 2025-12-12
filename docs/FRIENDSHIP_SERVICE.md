# 友谊服务 (Friendship Service) 使用指南

## 概述

友谊服务（Friendship Service）是 ChatIM 系统中用于管理好友关系和群组加入申请的微服务。它提供以下核心功能：

1. **好友管理**：发送好友请求、接受/拒绝、查看好友列表、删除好友
2. **群组加入申请**：申请加入群组、管理员审批申请、查看申请列表

## 架构

### 服务端口
- **gRPC 端口**: `:50053`（可通过配置修改）

### 依赖服务
- **MySQL**: 存储好友关系和申请数据
- **User Service**: 用户认证和验证

## 数据模型

### 好友相关表

#### friend_requests 表（好友申请）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(36) | 申请ID（UUID）|
| from_user_id | VARCHAR(36) | 申请者ID |
| to_user_id | VARCHAR(36) | 接收者ID |
| message | TEXT | 申请信息 |
| status | ENUM | 状态：pending/accepted/rejected/cancelled |
| created_at | TIMESTAMP | 创建时间 |
| processed_at | TIMESTAMP | 处理时间 |

#### friends 表（好友关系）
| 字段 | 类型 | 说明 |
|------|------|------|
| user_id_1 | VARCHAR(36) | 用户ID（较小）|
| user_id_2 | VARCHAR(36) | 用户ID（较大）|
| created_at | TIMESTAMP | 创建时间 |

*注*：user_id_1 和 user_id_2 通过规范化（取较小和较大的ID）形成复合主键，确保关系的唯一性。

### 群组相关表

#### group_join_requests 表（群申请）
| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(36) | 申请ID（UUID）|
| group_id | VARCHAR(36) | 群组ID |
| from_user_id | VARCHAR(36) | 申请者ID |
| message | TEXT | 申请信息 |
| status | ENUM | 状态：pending/accepted/rejected/cancelled |
| reviewed_by | VARCHAR(36) | 审批人ID |
| created_at | TIMESTAMP | 创建时间 |
| processed_at | TIMESTAMP | 处理时间 |

## gRPC API 方法

### 好友相关接口

#### 1. 发送好友请求
```protobuf
rpc SendFriendRequest(SendFriendRequestRequest) returns (SendFriendRequestResponse);

message SendFriendRequestRequest {
  string to_user_id = 1;  // 目标用户ID
  string message = 2;      // 申请信息
}

message SendFriendRequestResponse {
  int32 code = 1;
  string message = 2;
  string request_id = 3;   // 返回的申请ID
}
```

#### 2. 获取好友请求列表
```protobuf
rpc GetFriendRequests(GetFriendRequestsRequest) returns (GetFriendRequestsResponse);

message GetFriendRequestsRequest {
  int32 status = 1;    // 状态：0=pending, 1=accepted, 2=rejected, 3=cancelled
  int32 limit = 2;     // 分页大小（默认20，最大100）
  int32 offset = 3;    // 偏移量
}

message GetFriendRequestsResponse {
  int32 code = 1;
  string message = 2;
  repeated FriendRequest requests = 3;
  int32 total = 4;     // 总数
}

message FriendRequest {
  string id = 1;
  string from_user_id = 2;
  string from_username = 3;
  string from_nickname = 4;
  string message = 5;
  int32 status = 6;
  int64 created_at = 7;
}
```

#### 3. 处理好友请求
```protobuf
rpc ProcessFriendRequest(ProcessFriendRequestRequest) returns (ProcessFriendRequestResponse);

message ProcessFriendRequestRequest {
  string request_id = 1;   // 申请ID
  bool accept = 2;         // true=接受，false=拒绝
}

message ProcessFriendRequestResponse {
  int32 code = 1;
  string message = 2;
}
```

#### 4. 获取好友列表
```protobuf
rpc GetFriends(GetFriendsRequest) returns (GetFriendsResponse);

message GetFriendsRequest {
  int32 limit = 1;     // 分页大小（默认20，最大100）
  int32 offset = 2;    // 偏移量
}

message GetFriendsResponse {
  int32 code = 1;
  string message = 2;
  repeated Friend friends = 3;
  int32 total = 4;
}

message Friend {
  string user_id = 1;
  string username = 2;
  string nickname = 3;
  int64 created_at = 4;
}
```

#### 5. 删除好友
```protobuf
rpc RemoveFriend(RemoveFriendRequest) returns (RemoveFriendResponse);

message RemoveFriendRequest {
  string friend_user_id = 1;   // 好友ID
}

message RemoveFriendResponse {
  int32 code = 1;
  string message = 2;
}
```

### 群组申请相关接口

#### 1. 申请加入群组
```protobuf
rpc SendGroupJoinRequest(SendGroupJoinRequestRequest) returns (SendGroupJoinRequestResponse);

message SendGroupJoinRequestRequest {
  string group_id = 1;    // 群组ID
  string message = 2;      // 申请信息
}

message SendGroupJoinRequestResponse {
  int32 code = 1;
  string message = 2;
  string request_id = 3;   // 返回的申请ID
}
```

#### 2. 获取群申请列表（群主/管理员）
```protobuf
rpc GetGroupJoinRequests(GetGroupJoinRequestsRequest) returns (GetGroupJoinRequestsResponse);

message GetGroupJoinRequestsRequest {
  string group_id = 1;     // 群组ID
  int32 status = 2;        // 状态：0=pending, 1=accepted, 2=rejected, 3=cancelled
  int32 limit = 3;         // 分页大小（默认20，最大100）
  int32 offset = 4;        // 偏移量
}

message GetGroupJoinRequestsResponse {
  int32 code = 1;
  string message = 2;
  repeated GroupJoinRequest requests = 3;
  int32 total = 4;
}

message GroupJoinRequest {
  string id = 1;
  string group_id = 2;
  string from_user_id = 3;
  string from_username = 4;
  string from_nickname = 5;
  string message = 6;
  int32 status = 7;
  int64 created_at = 8;
}
```

#### 3. 处理群申请（群主/管理员）
```protobuf
rpc ProcessGroupJoinRequest(ProcessGroupJoinRequestRequest) returns (ProcessGroupJoinRequestResponse);

message ProcessGroupJoinRequestRequest {
  string request_id = 1;   // 申请ID
  bool accept = 2;         // true=接受，false=拒绝
}

message ProcessGroupJoinRequestResponse {
  int32 code = 1;
  string message = 2;
}
```

## 状态码约定

| 状态值 | 含义 |
|------|------|
| 0 | pending（待处理）|
| 1 | accepted（已接受）|
| 2 | rejected（已拒绝）|
| 3 | cancelled（已取消）|

## 启动服务

### 方式一：使用启动脚本
```bash
cd cmd
start.bat
```

### 方式二：直接运行
```bash
cd cmd/friendship
go run main.go
```

## 配置

在 `config.yaml` 中配置（需要的字段）：

```yaml
server:
  friendship_grpc_port: ":50053"
  friendship_grpc_addr: "localhost:50053"
```

## 错误处理

### 常见错误码

| gRPC Status Code | 含义 |
|------|------|
| Unauthenticated | 用户未认证 |
| InvalidArgument | 参数无效 |
| NotFound | 资源不存在 |
| AlreadyExists | 资源已存在 |
| PermissionDenied | 无权限 |
| Internal | 服务器内部错误 |

## 工作流程

### 好友请求流程

```
1. 用户 A 调用 SendFriendRequest(user_b_id)
   → 检查：不能是自己、不能已是好友、不能有待处理申请
   → 创建好友请求记录，状态为 pending

2. 用户 B 收到请求，查看待处理申请
   → 调用 GetFriendRequests(status=pending)

3. 用户 B 接受申请
   → 调用 ProcessFriendRequest(request_id, accept=true)
   → 更新申请状态为 accepted
   → 同时在 friends 表中插入双向关系（user_a_id, user_b_id）

4. 双方都可以通过 GetFriends 看到彼此
   → 调用 GetFriends(limit=20, offset=0)

5. 删除好友
   → 调用 RemoveFriend(friend_id)
   → 删除 friends 表中的关系
```

### 群申请流程

```
1. 用户 A 调用 SendGroupJoinRequest(group_id)
   → 检查：用户不是群成员、没有待处理申请
   → 创建群申请记录，状态为 pending

2. 群主/管理员 查看待申请
   → 调用 GetGroupJoinRequests(group_id, status=pending)
   → 需要权限验证（必须是群主或管理员）

3. 群主/管理员 审批申请
   → 调用 ProcessGroupJoinRequest(request_id, accept=true)
   → 更新申请状态为 accepted
   → reviewed_by 字段记录审批人
   → 同时向 group_members 表添加成员

4. 若拒绝
   → 调用 ProcessGroupJoinRequest(request_id, accept=false)
   → 只更新申请状态为 rejected
   → 不向 group_members 添加
```

## 数据库迁移

服务启动时会自动执行数据库迁移，创建必要的表：
- `friend_requests`
- `friends`
- `group_join_requests`

如需手动执行，参考 `migrations/004_friend_and_group_requests.sql`

## 注意事项

1. **好友关系的规范化**: friends 表中的 user_id_1 总是小于 user_id_2，确保关系的唯一性
2. **权限验证**: 处理群申请必须验证用户是否为群主或管理员
3. **事务支持**: 接受申请时，会在一个事务中同时更新申请状态和添加关系
4. **缓存考虑**: 若后续需要优化，可在 Redis 中缓存热点好友列表

## 扩展计划

- [ ] 好友分组管理
- [ ] 黑名单功能
- [ ] 好友备注名称
- [ ] 群组邀请（不需要申请）
- [ ] 申请通知推送

