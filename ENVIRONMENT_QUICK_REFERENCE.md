# 环境变量文件快速参考

## 📍 文件位置

所有环境变量文件都在项目根目录：`d:\git-demo\ChatIM\`

## 📋 文件清单

### 1. `.env` ⭐ 最重要
**位置**: `d:\git-demo\ChatIM\.env`
**大小**: 1.7 KB
**用途**: 本地开发环境变量（主配置文件）
**Git**: ❌ 忽略（包含敏感信息）

**包含内容**:
- gRPC 服务地址 (127.0.0.1:50051-50054)
- MySQL 连接信息
- Redis 连接信息
- JWT 密钥

### 2. `.env.example` 📖 模板
**位置**: `d:\git-demo\ChatIM\.env.example`
**大小**: 1.9 KB
**用途**: 环境变量模板和文档
**Git**: ✅ 提交（无敏感信息）

**使用方法**:
```bash
cp .env.example .env
# 然后编辑 .env 文件
```

### 3. `.env.local` 🔧 本地覆盖
**位置**: `d:\git-demo\ChatIM\.env.local`
**大小**: 1.1 KB
**用途**: 临时本地开发覆盖（可选）
**Git**: ❌ 忽略

**何时使用**: 需要临时覆盖某些变量进行测试时

### 4. `docker-compose.env` 🐳 Docker 配置
**位置**: `d:\git-demo\ChatIM\docker-compose.env`
**大小**: 1.1 KB
**用途**: Docker Compose 部署的环境变量
**Git**: ✅ 提交

**主要区别**:
- gRPC 地址使用服务名: `user-service:50051`
- 数据库使用 Docker 服务名: `mysql:3306`, `redis:6379`

## 🚀 快速开始

### 本地开发

```bash
# 1. 进入项目目录
cd d:\git-demo\ChatIM

# 2. 检查 .env 文件
cat .env | head -20

# 3. 编辑 .env（如需要）
# 默认配置已适合本地开发

# 4. 启动所有服务
# 在不同终端中运行：
go run ./cmd/user/main.go
go run ./cmd/message/main.go
go run ./cmd/group/main.go
go run ./cmd/friendship/main.go
go run ./cmd/api/main.go
```

### Docker 部署

```bash
# 使用 docker-compose.env 文件
docker-compose --env-file docker-compose.env up -d

# 或在 docker-compose.yml 中指定
# services:
#   user-service:
#     env_file:
#       - docker-compose.env
```

## 📊 环境变量对照表

| 变量名 | 本地值 | Docker 值 | 说明 |
|--------|--------|-----------|------|
| `CHATIM_SERVER_USER_GRPC_ADDR` | `127.0.0.1:50051` | `user-service:50051` | User Service 地址 |
| `CHATIM_SERVER_MESSAGE_GRPC_ADDR` | `127.0.0.1:50052` | `message-service:50052` | Message Service 地址 |
| `CHATIM_SERVER_GROUP_GRPC_ADDR` | `127.0.0.1:50053` | `group-service:50053` | Group Service 地址 |
| `CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR` | `127.0.0.1:50054` | `friendship-service:50054` | Friendship Service 地址 |
| `CHATIM_DATABASE_MYSQL_DSN` | `...@tcp(127.0.0.1:3306)/...` | `...@tcp(mysql:3306)/...` | MySQL 连接 |
| `CHATIM_DATABASE_REDIS_ADDR` | `127.0.0.1:6379` | `redis:6379` | Redis 连接 |

## 🔐 安全检查清单

- [ ] `.env` 文件不提交到 Git
- [ ] `.env` 中的密钥已更改（不使用默认值）
- [ ] 生产环境使用密钥管理系统
- [ ] `.env.example` 中无敏感信息
- [ ] 定期轮换密钥

## 📝 文件大小统计

```
.env                 1.7 KB   ← 使用这个
.env.example         1.9 KB   ← 参考这个
.env.local           1.1 KB   ← 可选覆盖
docker-compose.env   1.1 KB   ← Docker 使用
────────────────────────────
总计                 6.0 KB
```

## 🔗 相关文档

- **详细指南**: `docs/ENVIRONMENT_VARIABLES.md`
- **配置系统**: `pkg/config/config.go`
- **YAML 配置**: `pkg/config/config.yaml`
- **Docker 编排**: `docker-compose.yml`

## ❓ 常见问题

**Q: 如何为不同环境使用不同配置?**
A: 创建 `.env.staging`, `.env.production` 等，然后选择加载

**Q: Docker Compose 中如何使用自定义 .env 文件?**
A: `docker-compose --env-file custom.env up`

**Q: 如何验证所有环境变量都已设置?**
A: 检查 `pkg/config/config.go` 中的 `BindEnv()` 调用

**Q: 生产环境应该如何管理敏感信息?**
A: 使用专门的密钥管理系统（如 AWS Secrets Manager, HashiCorp Vault 等）

---

**最后更新**: 2024年  
**文件位置**: `d:\git-demo\ChatIM\`
