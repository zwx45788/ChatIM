# 友谊服务部署指南

## 前置条件

1. Go 1.21+ 
2. MySQL 8.0+
3. 已部署的 User Service（用于认证）
4. 已构建的 Proto 文件

## 部署步骤

### 1. 数据库准备

#### 执行迁移脚本

```bash
# 使用 MySQL 客户端
mysql -u root -p chaim < migrations/004_friend_and_group_requests.sql
```

或者让服务在启动时自动执行（推荐）：

```bash
# 服务启动时会自动检查并执行未执行的迁移
cd cmd/friendship
go run main.go
```

#### 验证表创建

```sql
-- 连接到 ChatIM 数据库
USE chaim;

-- 验证三个表已创建
SHOW TABLES LIKE '%friend%';
SHOW TABLES LIKE '%group_join%';

-- 检查表结构
DESCRIBE friend_requests;
DESCRIBE friends;
DESCRIBE group_join_requests;
```

### 2. 配置文件更新

编辑 `config.yaml`，添加 Friendship 服务配置：

```yaml
server:
  api_port: ":8080"
  user_grpc_port: ":50051"
  message_grpc_port: ":50052"
  group_grpc_port: ":50054"
  friendship_grpc_port: ":50053"
  
  # gRPC 地址（用于服务间通信）
  user_grpc_addr: "localhost:50051"
  message_grpc_addr: "localhost:50052"
  group_grpc_addr: "localhost:50054"
  friendship_grpc_addr: "localhost:50053"

database:
  mysql:
    dsn: "root:password@tcp(localhost:3306)/chaim?charset=utf8mb4&parseTime=true"
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0

jwt:
  secret: "your-jwt-secret-key"
```

### 3. 构建二进制文件

```bash
# 编译 Friendship 服务
cd cmd/friendship
go build -o friendship main.go

# 或编译所有服务
cd ..
go build ./...
```

### 4. 启动服务

#### 开发环境

```bash
cd cmd/friendship
go run main.go
```

#### 生产环境

```bash
# 后台运行
nohup ./friendship > friendship.log 2>&1 &

# 或使用 systemd（Linux）
# 创建 /etc/systemd/system/friendship.service
```

#### Windows 环境

```bash
cd cmd
start.bat
```

### 5. 验证服务

使用 grpcurl 或其他 gRPC 客户端测试：

```bash
# 列出服务
grpcurl -plaintext localhost:50053 list

# 查看 FriendshipService 的方法
grpcurl -plaintext localhost:50053 list ChatIM.friendship.FriendshipService

# 测试 SendFriendRequest
grpcurl -plaintext \
  -d '{"to_user_id":"user123","message":"你好，可以加好友吗？"}' \
  -H "authorization: Bearer <jwt_token>" \
  localhost:50053 \
  ChatIM.friendship.FriendshipService/SendFriendRequest
```

### 6. 日志查看

```bash
# 查看实时日志
tail -f friendship.log

# 查看完整日志
cat friendship.log
```

## Docker 部署

### Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -o friendship ./cmd/friendship

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/friendship .
COPY config.yaml .
COPY migrations/ migrations/

EXPOSE 50053

CMD ["./friendship"]
```

### docker-compose.yml 配置

```yaml
version: '3.8'

services:
  friendship:
    build:
      context: .
      dockerfile: Dockerfile.friendship
    ports:
      - "50053:50053"
    environment:
      - MYSQL_HOST=mysql
      - MYSQL_PORT=3306
      - MYSQL_USER=root
      - MYSQL_PASSWORD=password
      - MYSQL_DB=chaim
      - REDIS_ADDR=redis:6379
    depends_on:
      - mysql
      - redis
    networks:
      - chatim-network
    volumes:
      - ./logs:/root/logs

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: password
      MYSQL_DATABASE: chaim
    volumes:
      - mysql_data:/var/lib/mysql
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    networks:
      - chatim-network

  redis:
    image: redis:7-alpine
    networks:
      - chatim-network

volumes:
  mysql_data:

networks:
  chatim-network:
    driver: bridge
```

启动：

```bash
docker-compose up -d
```

## 监控和维护

### 健康检查

```bash
# 检查服务是否运行
grpcurl -plaintext localhost:50053 grpc.health.v1.Health/Check
```

### 性能优化建议

1. **连接池**: 数据库连接使用连接池
2. **缓存**: 频繁查询的好友列表可缓存至 Redis
3. **索引**: 确保 MySQL 表的索引正确

### 常见问题排查

#### 1. 无法连接数据库

```
ERROR: Failed to initialize database
```

**解决**:
- 检查 MySQL 是否运行
- 验证 DSN 配置正确
- 检查用户权限

#### 2. 无法加载迁移

```
ERROR: Failed to run migrations
```

**解决**:
- 确保 migrations 目录存在
- 检查 migrations 目录权限
- 验证数据库用户有创建表权限

#### 3. gRPC 端口被占用

```
ERROR: Failed to listen on gRPC port
```

**解决**:
- 修改 config.yaml 中的 friendship_grpc_port
- 或关闭占用该端口的进程

#### 4. 权限不足错误

```
ERROR: Permission denied
```

**解决**:
- 验证 User Service 返回的 JWT token 有效
- 检查用户是否为群主/管理员（群操作）
- 确保操作权限与资源所有者匹配

## 集成到 API Gateway

若使用 API Gateway（如 Kong、Envoy），需要添加路由配置：

```yaml
# 示例：Kong 配置
services:
  - name: friendship-service
    host: localhost
    port: 50053
    protocol: grpc
    routes:
      - paths:
          - /ChatIM.friendship.FriendshipService
```

## 备份和恢复

### 备份数据库

```bash
mysqldump -u root -p chaim > backup.sql
```

### 恢复数据库

```bash
mysql -u root -p chaim < backup.sql
```

## 升级流程

1. 停止现有服务
2. 备份数据库
3. 更新代码
4. 执行新的迁移脚本
5. 重新构建二进制
6. 启动新服务
7. 验证功能

## 性能指标

目标指标：
- API 响应时间 < 100ms
- 并发用户 > 10,000
- 数据库查询 < 50ms

监控关键指标：
- gRPC 请求延迟
- 数据库连接数
- Redis 命中率
- 错误率

## 安全建议

1. **认证**: 所有请求需要有效的 JWT token
2. **授权**: 严格校验操作权限（权限字段）
3. **数据验证**: 校验所有输入参数
4. **日志**: 记录所有敏感操作
5. **HTTPS/TLS**: 生产环境使用 TLS
6. **速率限制**: 防止滥用

