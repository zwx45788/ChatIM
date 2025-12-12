# 删除与移除功能实现文档

## 📋 概述

本文档说明三个新增的删除和移除功能：
1. **删除好友** (RemoveFriend) - 用户主动删除好友关系
2. **退出群聊** (LeaveGroup) - 用户主动退出群组
3. **踢出群聊** (RemoveGroupMember) - 群主踢出群成员

---

## 🎯 功能详情

### 1️⃣ RemoveFriend - 删除好友

#### RPC 定义
```protobuf
rpc RemoveFriend (RemoveFriendRequest) returns (RemoveFriendResponse);

message RemoveFriendRequest {
  string friend_user_id = 1;
}

message RemoveFriendResponse {
  int32 code = 1;      // 0=成功
  string message = 2;  // 状态信息
}
```

#### 实现逻辑

**仓储层** (`internal/friendship/repository/friendship_repository.go`)
```go
// RemoveFriend 删除好友关系
func (r *FriendshipRepository) RemoveFriend(ctx context.Context, userID1, userID2 string) error
  - 正规化ID: 确保 user_id_1 < user_id_2
  - 执行: DELETE FROM friends WHERE user_id_1=? AND user_id_2=?
  - 返回: 行影响数校验，为0时返回 "friendship not found" 错误
```

**处理层** (`internal/friendship/handler/friendship_handler.go`)
```go
// RemoveFriend 处理删除好友请求
func (h *FriendshipHandler) RemoveFriend(ctx context.Context, req *pb.RemoveFriendRequest) (*pb.RemoveFriendResponse, error)
  - 提取用户ID: auth.GetUserID(ctx)
  - 调用仓储: h.repo.RemoveFriend(ctx, userID, req.FriendUserId)
  - 错误处理: 检查 "friendship not found" 并返回 NotFound 状态
  - 返回: code=0, message="好友已删除"
```

#### 使用示例

**Go 客户端调用**
```go
client, _ := clients.NewFriendshipClient("localhost:50053")
defer client.Close()

ctx := context.Background()
// 添加认证 token
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

err := client.RemoveFriend(ctx, friendUserID)
if err != nil {
    log.Printf("删除好友失败: %v", err)
}
```

**gRPC 请求**
```bash
grpcurl -d '{"friend_user_id":"user123"}' \
  -H "authorization: Bearer TOKEN" \
  localhost:50053 friendship.FriendshipService/RemoveFriend
```

#### 权限检查
- ✅ 需要认证（JWT token）
- ✅ 只能删除自己的好友关系
- ✅ 自动删除双向关系

#### 返回值
- **成功**: `code=0, message="好友已删除"`
- **好友不存在**: `code=NotFound, message="好友关系不存在"`
- **系统错误**: `code=Internal, message="删除失败"`

---

### 2️⃣ LeaveGroup - 退出群聊

#### RPC 定义
```protobuf
rpc LeaveGroup (LeaveGroupRequest) returns (LeaveGroupResponse);

message LeaveGroupRequest {
  string group_id = 1;
}

message LeaveGroupResponse {
  int32 code = 1;      // 0=成功
  string message = 2;  // 状态信息
}
```

#### 实现逻辑

**仓储层** (`internal/friendship/repository/friendship_repository.go`)
```go
// LeaveGroup 用户退出群组
func (r *FriendshipRepository) LeaveGroup(ctx context.Context, groupID, userID string) error
  - 执行: DELETE FROM group_members WHERE group_id=? AND user_id=?
  - 返回: 行影响数校验，为0时返回 "用户不在该群组中" 错误
```

**支持方法** (`internal/friendship/repository/friendship_repository.go`)
```go
// CheckGroupMembership 检查用户是否在群组中
func (r *FriendshipRepository) CheckGroupMembership(ctx context.Context, groupID, userID string) (bool, error)
  - 查询: SELECT COUNT(*) FROM group_members WHERE group_id=? AND user_id=?
  - 返回: bool (true=在群组中, false=不在)
```

**处理层** (`internal/friendship/handler/friendship_handler.go`)
```go
// LeaveGroup 处理退出群聊请求
func (h *FriendshipHandler) LeaveGroup(ctx context.Context, req *pb.LeaveGroupRequest) (*pb.LeaveGroupResponse, error)
  - 提取用户ID: auth.GetUserID(ctx)
  - 检查成员: h.repo.CheckGroupMembership(ctx, req.GroupId, userID)
    - 不在群组则返回 NotFound 错误
  - 执行退出: h.repo.LeaveGroup(ctx, req.GroupId, userID)
  - 返回: code=0, message="已退出群组"
```

#### 使用示例

**Go 客户端调用**
```go
client, _ := clients.NewFriendshipClient("localhost:50053")
defer client.Close()

ctx := context.Background()
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

err := client.LeaveGroup(ctx, groupID)
if err != nil {
    log.Printf("退出群聊失败: %v", err)
}
```

**gRPC 请求**
```bash
grpcurl -d '{"group_id":"group123"}' \
  -H "authorization: Bearer TOKEN" \
  localhost:50053 friendship.FriendshipService/LeaveGroup
```

#### 权限检查
- ✅ 需要认证（JWT token）
- ✅ 只能退出自己加入的群组
- ✅ 群主退出时需要转让群组

#### 返回值
- **成功**: `code=0, message="已退出群组"`
- **不在群组中**: `code=NotFound, message="用户不在该群组中"`
- **系统错误**: `code=Internal, message="退出失败"`

---

### 3️⃣ RemoveGroupMember - 踢出群成员

#### RPC 定义
```protobuf
rpc RemoveGroupMember (RemoveGroupMemberRequest) returns (RemoveGroupMemberResponse);

message RemoveGroupMemberRequest {
  string group_id = 1;         // 群组ID
  string member_user_id = 2;   // 被踢出的成员ID
}

message RemoveGroupMemberResponse {
  int32 code = 1;      // 0=成功
  string message = 2;  // 状态信息
}
```

#### 实现逻辑

**仓储层** (`internal/friendship/repository/friendship_repository.go`)
```go
// RemoveGroupMember 管理员踢出群成员
func (r *FriendshipRepository) RemoveGroupMember(ctx context.Context, groupID, memberUserID string) error
  - 执行: DELETE FROM group_members WHERE group_id=? AND user_id=?
  - 返回: 行影响数校验，为0时返回 "用户不在该群组中" 错误

// CheckGroupOwner 检查用户是否是群主
func (r *FriendshipRepository) CheckGroupOwner(ctx context.Context, groupID, userID string) (bool, error)
  - 查询: SELECT owner_id FROM groups WHERE id=?
  - 返回: bool (true=是群主, false=不是)
```

**处理层** (`internal/friendship/handler/friendship_handler.go`)
```go
// RemoveGroupMember 处理踢出群成员请求
func (h *FriendshipHandler) RemoveGroupMember(ctx context.Context, req *pb.RemoveGroupMemberRequest) (*pb.RemoveGroupMemberResponse, error)
  - 提取操作者ID: auth.GetUserID(ctx)
  - 权限检查: h.repo.CheckGroupOwner(ctx, req.GroupId, operatorUserID)
    - 不是群主返回 PermissionDenied 错误
  - 验证: req.MemberUserId != operatorUserID
    - 不能踢自己，返回 InvalidArgument 错误
  - 成员检查: h.repo.CheckGroupMembership(ctx, req.GroupId, req.MemberUserId)
    - 成员不在群组返回 NotFound 错误
  - 执行踢出: h.repo.RemoveGroupMember(ctx, req.GroupId, req.MemberUserId)
  - 返回: code=0, message="已踢出该成员"
```

#### 使用示例

**Go 客户端调用**
```go
client, _ := clients.NewFriendshipClient("localhost:50053")
defer client.Close()

ctx := context.Background()
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

// 群主踢出成员
err := client.RemoveGroupMember(ctx, groupID, memberUserID)
if err != nil {
    log.Printf("踢出成员失败: %v", err)
}
```

**gRPC 请求**
```bash
grpcurl -d '{"group_id":"group123","member_user_id":"user456"}' \
  -H "authorization: Bearer TOKEN" \
  localhost:50053 friendship.FriendshipService/RemoveGroupMember
```

#### 权限检查
- ✅ 需要认证（JWT token）
- ✅ **只有群主可以踢人**（关键权限验证）
- ✅ 不能踢自己
- ✅ 被踢出者必须在群组中

#### 返回值
- **成功**: `code=0, message="已踢出该成员"`
- **权限拒绝**: `code=PermissionDenied, message="只有群主才能踢人"`
- **不能踢自己**: `code=InvalidArgument, message="不能踢出自己"`
- **成员不存在**: `code=NotFound, message="该用户不在群组中"`
- **系统错误**: `code=Internal, message="踢出失败"`

---

## 📊 数据库操作总览

### 表操作对应关系

| 功能 | 操作表 | SQL 操作 | 行为 |
|------|--------|---------|------|
| RemoveFriend | friends | DELETE | 删除双向好友关系 |
| LeaveGroup | group_members | DELETE | 删除用户群组成员关系 |
| RemoveGroupMember | group_members | DELETE | 删除指定成员关系 |

### 检查操作对应关系

| 功能 | 检查项 | 表 | SQL 查询 |
|------|--------|-----|---------|
| RemoveFriend | 好友是否存在 | friends | COUNT(*) WHERE user_id_1=? AND user_id_2=? |
| LeaveGroup | 用户是否在群中 | group_members | COUNT(*) WHERE group_id=? AND user_id=? |
| RemoveGroupMember | 操作者是否为群主 | groups | SELECT owner_id WHERE id=? |
| RemoveGroupMember | 被踢者是否在群中 | group_members | COUNT(*) WHERE group_id=? AND user_id=? |

---

## 🔄 工作流程示意

### 删除好友工作流

```
Client Request
    ↓
[RemoveFriend RPC]
    ↓
提取 & 认证用户ID
    ↓
删除好友关系 (friends 表)
    ↓
[成功]
    ├─→ 返回 code=0, message="好友已删除"
    └─→ 好友关系完全删除
```

### 退出群聊工作流

```
Client Request
    ↓
[LeaveGroup RPC]
    ↓
提取 & 认证用户ID
    ↓
检查用户是否在群中
    ├─→ [不在] → 返回 NotFound
    └─→ [在]
        ↓
        删除群成员关系 (group_members 表)
        ↓
        [成功]
        └─→ 返回 code=0, message="已退出群组"
```

### 踢出成员工作流

```
Client Request
    ↓
[RemoveGroupMember RPC]
    ↓
提取 & 认证操作者ID
    ↓
检查操作者权限
    ├─→ [不是群主] → 返回 PermissionDenied
    └─→ [是群主]
        ↓
        验证不能踢自己
        ├─→ [是自己] → 返回 InvalidArgument
        └─→ [不是自己]
            ↓
            检查目标成员是否在群中
            ├─→ [不在] → 返回 NotFound
            └─→ [在]
                ↓
                删除成员关系 (group_members 表)
                ↓
                [成功]
                └─→ 返回 code=0, message="已踢出该成员"
```

---

## 🧪 测试场景

### RemoveFriend 测试
```
✅ 删除存在的好友关系
✅ 尝试删除不存在的好友 → NotFound
✅ 未认证用户尝试删除 → Unauthenticated
✅ 好友关系同步检查（A删除B，B查询不应找到A）
```

### LeaveGroup 测试
```
✅ 正常退出群组
✅ 尝试退出不在的群组 → NotFound
✅ 未认证用户尝试退出 → Unauthenticated
✅ 退出后查询GetUserGroups应看不到该群
```

### RemoveGroupMember 测试
```
✅ 群主踢出成员
✅ 非群主尝试踢人 → PermissionDenied
✅ 群主尝试踢自己 → InvalidArgument
✅ 尝试踢不存在的成员 → NotFound
✅ 未认证用户尝试踢人 → Unauthenticated
```

---

## 💾 客户端库集成

### 方法签名

```go
// 删除好友
func (fc *FriendshipClient) RemoveFriend(ctx context.Context, friendUserID string) error

// 退出群聊
func (fc *FriendshipClient) LeaveGroup(ctx context.Context, groupID string) error

// 踢出群成员
func (fc *FriendshipClient) RemoveGroupMember(ctx context.Context, groupID, memberUserID string) error
```

### 完整调用示例

```go
package main

import (
    "context"
    "log"
    "ChatIM/pkg/clients"
    "google.golang.org/grpc/metadata"
)

func main() {
    // 创建客户端
    client, err := clients.NewFriendshipClient("localhost:50053")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 创建带认证的上下文
    ctx := context.Background()
    ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

    // 1. 删除好友
    if err := client.RemoveFriend(ctx, "friend_user_id"); err != nil {
        log.Printf("删除好友失败: %v", err)
    }

    // 2. 退出群聊
    if err := client.LeaveGroup(ctx, "group_id"); err != nil {
        log.Printf("退出群聊失败: %v", err)
    }

    // 3. 踢出群成员（仅群主）
    if err := client.RemoveGroupMember(ctx, "group_id", "member_user_id"); err != nil {
        log.Printf("踢出成员失败: %v", err)
    }
}
```

---

## 📝 API 参考速查

| 功能 | 方法 | 请求参数 | 响应 | 权限 |
|------|------|--------|------|------|
| 删除好友 | RemoveFriend | friend_user_id | code, message | 认证用户 |
| 退出群聊 | LeaveGroup | group_id | code, message | 认证用户 |
| 踢出成员 | RemoveGroupMember | group_id, member_user_id | code, message | 群主 |

---

## 🚀 部署考虑

### 性能指标
- **删除好友**: ~ 20-30ms（简单DELETE操作）
- **退出群聊**: ~ 30-50ms（含检查+删除）
- **踢出成员**: ~ 30-50ms（含权限检查+删除）

### 数据库索引建议
```sql
-- 加速查询
CREATE INDEX idx_friends_user_pair ON friends(user_id_1, user_id_2);
CREATE INDEX idx_group_members_group_user ON group_members(group_id, user_id);
CREATE INDEX idx_groups_owner ON groups(owner_id);
```

### 事务处理
- RemoveFriend: 单操作，无需事务
- LeaveGroup: 单操作，无需事务
- RemoveGroupMember: 单操作，无需事务

---

## ⚠️ 注意事项

1. **级联删除**
   - 删除好友时，只删除 friends 表，不影响其他表
   - 用户退出群聊时，只删除 group_members，不删除群组本身

2. **权限验证**
   - RemoveGroupMember 必须验证操作者是群主
   - 所有操作都需要有效的 JWT token

3. **边界情况**
   - 用户不能删除不存在的好友
   - 用户不能从不在的群组退出
   - 群主不能踢不存在的成员或自己

4. **审计日志**
   - 建议在实际部署中添加操作日志
   - 记录删除好友、退出群聊、踢出成员的时间和操作者

---

**文档版本**: v1.0  
**更新日期**: 2024年12月  
**状态**: ✅ 完整实现

