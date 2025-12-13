# Friendship Service 配置总结

## ✅ 已完成的配置

### 1. 端口分配

| 服务 | 开发端口 | Docker 端口 | 说明 |
|------|---------|-----------|------|
| API Gateway | :8080 | 8081:8080 | HTTP REST API |
| User Service | :50051 | 50051:50051 | gRPC |
| Message Service | :50052 | 50052:50052 | gRPC |
| Group Service | :50053 | 50053:50053 | gRPC (暂未启用) |
| **Friendship Service** | **:50054** | **50054:50054** | **gRPC** ✨ 新增 |

### 2. 配置文件更新

#### config.yaml
```yaml
server:
  friendship_grpc_port: ":50054"          # 本地开发
  friendship_grpc_addr: "127.0.0.1:50054" # 本地开发连接地址
```

#### config.go
```go
// 环境变量绑定
viper.BindEnv("server.friendship_grpc_addr", "CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR")
```

### 3. Docker 配置

#### cmd/friendship/Dockerfile ✨ 新建
```dockerfile
# 多阶段构建
# Stage 1: 编译
FROM golang:1.21-alpine AS builder
# Stage 2: 运行
FROM alpine:latest
EXPOSE 50054
CMD ["./friendship"]
```

#### docker-compose.yml 更新
```yaml
# 新增 friendship-service
friendship-service:
  build:
    context: .
    dockerfile: cmd/friendship/Dockerfile
  container_name: chatim_friendship_service
  ports:
    - "50054:50054"
  environment:
    CHATIM_DATABASE_MYSQL_DSN: "..."
    CHATIM_DATABASE_REDIS_ADDR: "..."
  depends_on:
    - mysql
    - redis

# API Gateway 更新
api-gateway:
  environment:
    CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR: "friendship-service:50054"
  depends_on:
    - friendship-service  # 新增依赖
```

---

## 🚀 启动方式

### 本地开发

```bash
# 1. 启动 MySQL 和 Redis
docker-compose up mysql redis

# 2. 启动各个服务（新开终端）
# Terminal 1
go run ./cmd/user/main.go

# Terminal 2
go run ./cmd/message/main.go

# Terminal 3
go run ./cmd/friendship/main.go  # ✨ Friendship Service

# Terminal 4
go run ./cmd/api/main.go
```

### Docker 容器

```bash
# 启动所有容器（包括 friendship-service）
docker-compose up -d

# 查看日志
docker-compose logs -f friendship-service

# 停止容器
docker-compose down
```

### 验证服务运行

```bash
# 本地开发
netstat -an | grep 50054

# Docker
docker ps | grep friendship
docker logs chatim_friendship_service

# 测试连接
grpcurl -plaintext localhost:50054 list
```

---

## 📊 架构图

```
客户端
  ↓
API Gateway (8080/8081)
  ├── → User Service (50051)
  ├── → Message Service (50052)
  ├── → Group Service (50053)
  └── → Friendship Service (50054) ✨ 新增
       ↓
       MySQL + Redis
```

---

## 📝 环境变量映射

| 环境变量 | 配置键 | 用途 | 默认值 |
|---------|--------|------|--------|
| `CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR` | `server.friendship_grpc_addr` | Friendship Service 地址 | `127.0.0.1:50054` |

**Docker 环境自动覆盖**:
```bash
CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR=friendship-service:50054
```

---

## ✅ 验证清单

- [x] 端口配置 (:50054)
- [x] config.yaml 更新
- [x] config.go 环境变量绑定
- [x] Dockerfile 创建
- [x] docker-compose.yml 更新
- [x] API Gateway 依赖配置
- [x] 项目编译测试 ✅ PASS

---

## 🔗 相关文件

- **配置**: `pkg/config/config.yaml` 和 `pkg/config/config.go`
- **Dockerfile**: `cmd/friendship/Dockerfile`
- **编排**: `docker-compose.yml`
- **源码**: `cmd/friendship/main.go`

---

**状态**: ✅ Friendship Service 已完全集成到部署系统
