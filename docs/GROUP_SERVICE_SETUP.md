# Group Service 配置

## ✅ Group Service 已启用

Group service 现已完全启用，用于管理用户创建的群组。

### 📋 服务信息

| 项目 | 值 |
|------|-----|
| **服务名** | Group Service |
| **gRPC 端口** | 50053 |
| **开发环境** | `127.0.0.1:50053` |
| **Docker 环境** | `group-service:50053` |
| **源代码** | `cmd/group/main.go` |
| **Dockerfile** | `cmd/group/Dockerfile` |

### 🚀 启动方式

#### 本地开发

```bash
go run ./cmd/group/main.go
```

#### Docker 部署

```bash
# 启动单个 group-service
docker-compose up group-service

# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f group-service

# 停止服务
docker-compose down
```

### 🔗 API Gateway 集成

API Gateway 现在会自动连接到 Group Service:

```yaml
environment:
  CHATIM_SERVER_GROUP_GRPC_ADDR: "group-service:50053"

depends_on:
  - group-service  # ✨ 已启用
```

### 📊 完整的微服务架构

```
API Gateway (8080)
    ├── User Service (50051)
    ├── Message Service (50052)
    ├── Group Service (50053) ✨ 已启用
    └── Friendship Service (50054)
          ↓
       MySQL + Redis
```

### 🔍 验证服务

#### 本地验证
```bash
# 检查端口是否监听
netstat -an | grep 50053

# 测试 gRPC 连接
grpcurl -plaintext localhost:50053 list
```

#### Docker 验证
```bash
# 查看容器状态
docker ps | grep group

# 查看容器日志
docker logs chatim_group_service

# 进入容器
docker exec -it chatim_group_service sh
```

### 📝 Group Service 功能

Group Service 提供以下功能：
- 创建群组
- 获取群组列表
- 获取群组成员
- 更新群组信息
- 删除群组
- 管理群组成员

### 🔧 配置参数

配置项位置: `pkg/config/config.yaml`

```yaml
server:
  group_grpc_port: ":50053"           # 本地开发端口
  group_grpc_addr: "127.0.0.1:50053"  # 本地开发地址
```

Docker 环境变量覆盖:
```bash
CHATIM_SERVER_GROUP_GRPC_ADDR=group-service:50053
```

### ✅ 状态检查清单

- [x] Group Service 源代码存在
- [x] Dockerfile 配置完整
- [x] docker-compose.yml 已启用
- [x] 端口配置 (50053)
- [x] API Gateway 依赖配置
- [x] 配置文件支持

---

**状态**: ✅ Group Service 已完全启用  
**启用时间**: 当前  
**依赖**: MySQL, Redis
