# 搜索功能 API 文档

## 1. 搜索用户

搜索用户名或昵称匹配的用户。

### 接口信息

- **URL**: `/api/v1/search/users`
- **Method**: `GET`
- **认证**: 需要 JWT Token

### 请求参数

| 参数名 | 类型 | 必填 | 说明 | 默认值 |
|--------|------|------|------|--------|
| keyword | string | 是 | 搜索关键词 | - |
| limit | int64 | 否 | 每页数量（1-100） | 20 |
| offset | int64 | 否 | 偏移量 | 0 |

### 请求示例

```bash
curl -X GET "http://localhost:8080/api/v1/search/users?keyword=张三&limit=10&offset=0" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 响应参数

| 参数名 | 类型 | 说明 |
|--------|------|------|
| code | int32 | 状态码（0=成功） |
| message | string | 响应消息 |
| results | array | 用户搜索结果列表 |
| results[].id | int64 | 用户ID |
| results[].username | string | 用户名 |
| results[].nickname | string | 昵称 |
| results[].avatar | string | 头像URL |
| total | int64 | 符合条件的总数 |

### 响应示例

```json
{
  "code": 0,
  "message": "搜索用户成功",
  "results": [
    {
      "id": 123,
      "username": "zhangsan",
      "nickname": "张三",
      "avatar": "https://example.com/avatar/123.jpg"
    },
    {
      "id": 456,
      "username": "zhangsan2",
      "nickname": "张三丰",
      "avatar": "https://example.com/avatar/456.jpg"
    }
  ],
  "total": 25
}
```

### 搜索规则

1. 关键词匹配：用户名或昵称包含关键词即可
2. 排序规则（优先级从高到低）：
   - 完全匹配（用户名或昵称等于关键词）
   - 前缀匹配（用户名或昵称以关键词开头）
   - 包含匹配（用户名或昵称包含关键词）
3. 大小写不敏感

### 错误码

| 错误码 | 说明 |
|--------|------|
| 1001 | 参数错误 |
| 1002 | 数据库查询失败 |

---

## 2. 搜索群组

搜索群名称或描述匹配的群组。

### 接口信息

- **URL**: `/api/v1/search/groups`
- **Method**: `GET`
- **认证**: 需要 JWT Token

### 请求参数

| 参数名 | 类型 | 必填 | 说明 | 默认值 |
|--------|------|------|------|--------|
| keyword | string | 是 | 搜索关键词 | - |
| limit | int64 | 否 | 每页数量（1-100） | 20 |
| offset | int64 | 否 | 偏移量 | 0 |

### 请求示例

```bash
curl -X GET "http://localhost:8080/api/v1/search/groups?keyword=技术交流&limit=10&offset=0" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 响应参数

| 参数名 | 类型 | 说明 |
|--------|------|------|
| code | int32 | 状态码（0=成功） |
| message | string | 响应消息 |
| results | array | 群组搜索结果列表 |
| results[].id | int64 | 群组ID |
| results[].name | string | 群名称 |
| results[].avatar | string | 群头像URL |
| results[].description | string | 群描述 |
| results[].member_count | int32 | 群成员数量 |
| total | int64 | 符合条件的总数 |

### 响应示例

```json
{
  "code": 0,
  "message": "搜索群组成功",
  "results": [
    {
      "id": 789,
      "name": "技术交流群",
      "avatar": "https://example.com/group/789.jpg",
      "description": "技术讨论与分享",
      "member_count": 150
    },
    {
      "id": 101,
      "name": "前端技术交流",
      "avatar": "https://example.com/group/101.jpg",
      "description": "专注前端技术的交流群",
      "member_count": 89
    }
  ],
  "total": 12
}
```

### 搜索规则

1. 关键词匹配：群名称或群描述包含关键词即可
2. 只返回未被删除的群组（is_deleted=0）
3. 排序规则（优先级从高到低）：
   - 完全匹配（群名称或描述等于关键词）
   - 前缀匹配（群名称或描述以关键词开头）
   - 包含匹配（群名称或描述包含关键词）
   - 在同等匹配优先级下，按群成员数量降序排列（member_count DESC）
4. 大小写不敏感

### 错误码

| 错误码 | 说明 |
|--------|------|
| 1001 | 参数错误 |
| 1002 | 数据库查询失败 |

---

## 使用场景

### 1. 添加好友时搜索用户

用户在添加好友界面输入关键词，实时搜索匹配的用户：

```javascript
// 前端示例代码
async function searchUsers(keyword) {
  const response = await fetch(
    `/api/v1/search/users?keyword=${encodeURIComponent(keyword)}&limit=20`,
    {
      headers: {
        'Authorization': `Bearer ${getToken()}`
      }
    }
  );
  const data = await response.json();
  return data.results;
}
```

### 2. 加入群组时搜索群

用户在加群界面搜索感兴趣的群组：

```javascript
// 前端示例代码
async function searchGroups(keyword) {
  const response = await fetch(
    `/api/v1/search/groups?keyword=${encodeURIComponent(keyword)}&limit=20`,
    {
      headers: {
        'Authorization': `Bearer ${getToken()}`
      }
    }
  );
  const data = await response.json();
  return data.results;
}
```

### 3. 分页加载

使用 offset 和 limit 实现分页加载：

```javascript
// 加载第 2 页（每页 20 条）
const page = 2;
const pageSize = 20;
const offset = (page - 1) * pageSize;

const response = await fetch(
  `/api/v1/search/users?keyword=张三&limit=${pageSize}&offset=${offset}`,
  {
    headers: {
      'Authorization': `Bearer ${getToken()}`
    }
  }
);
```

---

## 性能优化建议

### 1. 数据库索引

确保以下字段有索引以提升查询性能：

```sql
-- 用户表索引
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_nickname ON users(nickname);

-- 群组表索引
CREATE INDEX idx_groups_name ON groups(name);
CREATE INDEX idx_groups_description ON groups(description(100));
CREATE INDEX idx_groups_is_deleted ON groups(is_deleted);
```

### 2. 前端防抖

搜索输入框应该使用防抖（debounce）技术，避免频繁请求：

```javascript
import { debounce } from 'lodash';

const debouncedSearch = debounce(async (keyword) => {
  const results = await searchUsers(keyword);
  // 更新UI
}, 300); // 300ms 防抖
```

### 3. 缓存热门搜索

对于热门关键词的搜索结果，可以考虑使用 Redis 缓存：

```go
// 伪代码示例
func (h *UserHandler) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
    cacheKey := fmt.Sprintf("search:users:%s:%d:%d", req.Keyword, req.Limit, req.Offset)
    
    // 尝试从缓存获取
    if cached, err := redis.Get(cacheKey); err == nil {
        return parseResponse(cached), nil
    }
    
    // 从数据库查询
    results := queryFromDB(req)
    
    // 缓存结果（5分钟）
    redis.Set(cacheKey, results, 5*time.Minute)
    
    return results, nil
}
```

---

## 安全注意事项

1. **SQL 注入防护**: 使用参数化查询，避免直接拼接 SQL
2. **速率限制**: 建议对搜索接口设置速率限制，防止恶意刷接口
3. **关键词长度限制**: 建议限制关键词长度不超过 50 字符
4. **敏感信息过滤**: 搜索结果不应包含敏感信息（如密码、邮箱等）

---

## 测试用例

### 用户搜索测试

```bash
# 1. 正常搜索
curl -X GET "http://localhost:8080/api/v1/search/users?keyword=test" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 2. 分页测试
curl -X GET "http://localhost:8080/api/v1/search/users?keyword=test&limit=5&offset=10" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 3. 空关键词（应该返回所有用户或提示错误）
curl -X GET "http://localhost:8080/api/v1/search/users?keyword=" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 群组搜索测试

```bash
# 1. 正常搜索
curl -X GET "http://localhost:8080/api/v1/search/groups?keyword=技术" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 2. 大limit测试（超过100应该被限制）
curl -X GET "http://localhost:8080/api/v1/search/groups?keyword=技术&limit=200" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 3. 特殊字符测试
curl -X GET "http://localhost:8080/api/v1/search/groups?keyword=C%2B%2B" \
  -H "Authorization: Bearer YOUR_TOKEN"
```
