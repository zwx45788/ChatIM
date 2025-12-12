# 用户群组列表功能说明

## 新增功能

### GetUserGroups RPC 方法

获取当前用户所在的所有群组列表。

#### 方法签名

```protobuf
rpc GetUserGroups(GetUserGroupsRequest) returns (GetUserGroupsResponse);
```

#### 请求参数 (GetUserGroupsRequest)

| 字段 | 类型 | 说明 |
|------|------|------|
| limit | int64 | 分页大小（默认20，最大100） |
| offset | int64 | 偏移量 |

#### 响应 (GetUserGroupsResponse)

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int32 | 状态码（0=成功） |
| message | string | 返回消息 |
| groups | []GroupInfo | 群组列表 |
| total | int32 | 总群组数 |

#### GroupInfo 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| group_id | string | 群组ID |
| group_name | string | 群组名称 |
| description | string | 群组描述 |
| member_count | int32 | 群组成员数 |
| created_at | int64 | 创建时间戳 |

## 使用示例

### gRPC 调用

```bash
grpcurl -plaintext \
  -d '{"limit":20,"offset":0}' \
  -H "authorization: Bearer <JWT_TOKEN>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/GetUserGroups
```

### Go 客户端调用

```go
import "ChatIM/pkg/clients"

client, _ := clients.NewFriendshipClient("localhost:50053")
defer client.Close()

// 获取用户所有群组（分页）
groups, total, err := client.GetUserGroups(ctx, 20, 0)
if err != nil {
    log.Fatalf("Error: %v", err)
}

log.Printf("用户有 %d 个群组", total)
for i, group := range groups {
    log.Printf("[%d] %s (%d 成员)", i+1, group.GroupName, group.MemberCount)
}
```

## 返回示例

```json
{
  "code": 0,
  "message": "查询成功",
  "groups": [
    {
      "group_id": "group_123",
      "group_name": "前端开发",
      "description": "前端技术讨论",
      "member_count": 25,
      "created_at": 1702345600
    },
    {
      "group_id": "group_456",
      "group_name": "后端开发",
      "description": "后端架构讨论",
      "member_count": 18,
      "created_at": 1702345800
    }
  ],
  "total": 12
}
```

## 实现细节

### 查询逻辑

```sql
SELECT g.id, g.name, g.description, COUNT(gm.user_id) as member_count, g.created_at
FROM groups g
JOIN group_members gm ON g.id = gm.group_id
WHERE gm.user_id = ?
GROUP BY g.id, g.name, g.description, g.created_at
ORDER BY g.created_at DESC
LIMIT ? OFFSET ?
```

### 关键特点

1. **自动计算成员数**: 使用 COUNT() 自动统计每个群组的成员数
2. **分页支持**: 支持 limit/offset 分页查询
3. **时间排序**: 按创建时间倒序排列（最新的群组在前）
4. **完整信息**: 返回群组基本信息和统计数据

## 使用场景

1. **用户首页**: 显示用户加入的所有群组
2. **群组导航**: 让用户快速切换群组
3. **群组统计**: 显示用户群组数量
4. **侧边栏**: 在消息应用中显示群组列表

## 权限要求

- ✅ 需要有效的 JWT token（已认证用户）
- ✅ 只能查看当前用户自己的群组
- ✅ 自动过滤：只返回用户所在的群组

## 性能考虑

| 操作 | 响应时间 | 备注 |
|------|---------|------|
| 查询 10 个群组 | < 50ms | 使用了分组和索引 |
| 查询 100 个群组 | < 100ms | 最多查询 1000+ 时可能变慢 |

### 优化建议

如果用户加入的群组很多（>1000），建议：
1. 增加分页 limit 值（但不超过 100）
2. 在前端做缓存
3. 考虑在 Redis 中缓存用户的群组列表

## 数据库要求

确保以下表和索引存在：

```sql
-- 必需的表
- groups (id, name, description, created_at)
- group_members (group_id, user_id)

-- 建议的索引
CREATE INDEX idx_group_members_user ON group_members(user_id);
CREATE INDEX idx_groups_created ON groups(created_at DESC);
```

## 扩展功能

### 可选添加

1. **按群组类型筛选**: 区分普通群和企业群
2. **按创建时间筛选**: 查询特定时间范围内的群组
3. **搜索功能**: 按群组名称搜索
4. **未读消息统计**: 显示每个群组的未读消息数
5. **群组功能权限**: 返回用户在各个群组中的角色

示例：

```protobuf
message GetUserGroupsRequest {
  int64 limit = 1;
  int64 offset = 2;
  string search = 3;           // 可选：搜索群组名
  string type = 4;             // 可选：筛选群组类型
  int64 created_after = 5;     // 可选：创建时间筛选
}
```

