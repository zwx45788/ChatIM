# ChatIM 环境变量配置指南

## 📁 环境变量文件位置

项目根目录下有以下环境变量相关文件：

### 文件清单

| 文件 | 用途 | 是否提交 Git | 说明 |
|------|------|-----------|------|
| **`.env`** | 本地开发环境变量 | ❌ 否 | 包含敏感信息，Git 会忽略 |
| **`.env.example`** | 环境变量模板 | ✅ 是 | 新开发者的参考，无敏感信息 |
| **`.env.local`** | 本地覆盖配置 | ❌ 否 | 用于临时本地测试 |
| **`docker-compose.env`** | Docker 部署环境变量 | ✅ 是 | Docker 容器使用的配置 |

---

## 🚀 快速开始

### 步骤 1: 创建本地环境变量文件

```bash
# 复制模板文件
cp .env.example .env

# 根据需要编辑
vi .env  # 或使用您的编辑器
```

### 步骤 2: 验证环境变量

```bash
# 显示所有环境变量
cat .env

# 或只显示特定变量
grep CHATIM_SERVER .env
```

---

## 📋 环境变量说明

### gRPC 服务地址

#### 本地开发模式

```bash
# 各服务都在本地运行
CHATIM_SERVER_USER_GRPC_ADDR=127.0.0.1:50051
CHATIM_SERVER_MESSAGE_GRPC_ADDR=127.0.0.1:50052
CHATIM_SERVER_GROUP_GRPC_ADDR=127.0.0.1:50053
CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR=127.0.0.1:50054
```

#### Docker 部署模式

```bash
# 各服务使用容器名在 Docker 网络中通信
CHATIM_SERVER_USER_GRPC_ADDR=user-service:50051
CHATIM_SERVER_MESSAGE_GRPC_ADDR=message-service:50052
CHATIM_SERVER_GROUP_GRPC_ADDR=group-service:50053
CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR=friendship-service:50054
```

### 数据库配置

#### 本地数据库

```bash
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_USER=chatim_user
MYSQL_PASSWORD=chatim_pass
MYSQL_DATABASE=chatim
CHATIM_DATABASE_MYSQL_DSN=chatim_user:chatim_pass@tcp(127.0.0.1:3306)/chatim?charset=utf8mb4&parseTime=True&loc=Local

REDIS_HOST=127.0.0.1
REDIS_PORT=6379
CHATIM_DATABASE_REDIS_ADDR=127.0.0.1:6379
```

#### Docker 数据库（容器中运行）

如果 MySQL 和 Redis 在 Docker 中：

```bash
# 如果本地端口映射为 3307 和 6380
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3307
CHATIM_DATABASE_MYSQL_DSN=chatim_user:chatim_pass@tcp(127.0.0.1:3307)/chatim?charset=utf8mb4&parseTime=True&loc=Local

REDIS_HOST=127.0.0.1
REDIS_PORT=6380
CHATIM_DATABASE_REDIS_ADDR=127.0.0.1:6380
```

#### Docker Compose 中的数据库

```bash
# 使用服务名
CHATIM_DATABASE_MYSQL_DSN=chatim_user:chatim_pass@tcp(mysql:3306)/chatim?charset=utf8mb4&parseTime=True&loc=Local
CHATIM_DATABASE_REDIS_ADDR=redis:6379
```

---

## 🔄 使用场景

### 场景 1: 完全本地开发

**条件**: 本地有 MySQL 和 Redis

**配置**:
```bash
# .env 文件中
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
CHATIM_DATABASE_MYSQL_DSN=chatim_user:chatim_pass@tcp(127.0.0.1:3306)/chatim?charset=utf8mb4&parseTime=True&loc=Local

REDIS_HOST=127.0.0.1
REDIS_PORT=6379
CHATIM_DATABASE_REDIS_ADDR=127.0.0.1:6379

CHATIM_SERVER_USER_GRPC_ADDR=127.0.0.1:50051
CHATIM_SERVER_MESSAGE_GRPC_ADDR=127.0.0.1:50052
CHATIM_SERVER_GROUP_GRPC_ADDR=127.0.0.1:50053
CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR=127.0.0.1:50054
```

**启动**:
```bash
# Terminal 1 - MySQL
docker run -d -p 3306:3306 -e MYSQL_ROOT_PASSWORD=060629 mysql

# Terminal 2 - Redis
docker run -d -p 6379:6379 redis

# Terminal 3 - User Service
go run ./cmd/user/main.go

# Terminal 4 - Message Service
go run ./cmd/message/main.go

# Terminal 5 - Group Service
go run ./cmd/group/main.go

# Terminal 6 - Friendship Service
go run ./cmd/friendship/main.go

# Terminal 7 - API Gateway
go run ./cmd/api/main.go
```

### 场景 2: Docker Compose 完全部署

**条件**: 使用 Docker 运行所有服务

**配置**:
```bash
# docker-compose.env 中的配置
CHATIM_SERVER_USER_GRPC_ADDR=user-service:50051
CHATIM_SERVER_MESSAGE_GRPC_ADDR=message-service:50052
CHATIM_SERVER_GROUP_GRPC_ADDR=group-service:50053
CHATIM_SERVER_FRIENDSHIP_GRPC_ADDR=friendship-service:50054

CHATIM_DATABASE_MYSQL_DSN=chatim_user:chatim_pass@tcp(mysql:3306)/chatim?charset=utf8mb4&parseTime=True&loc=Local
CHATIM_DATABASE_REDIS_ADDR=redis:6379
```

**启动**:
```bash
docker-compose up -d
```

### 场景 3: 混合模式（Docker DB + 本地服务）

**条件**: MySQL 和 Redis 在 Docker，但服务在本地

**配置**:
```bash
# 启动 Docker 中的 MySQL 和 Redis
docker-compose up -d mysql redis

# .env 中配置
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3307
CHATIM_DATABASE_MYSQL_DSN=chatim_user:chatim_pass@tcp(127.0.0.1:3307)/chatim?charset=utf8mb4&parseTime=True&loc=Local

REDIS_HOST=127.0.0.1
REDIS_PORT=6380
CHATIM_DATABASE_REDIS_ADDR=127.0.0.1:6380

# gRPC 地址仍为本地
CHATIM_SERVER_USER_GRPC_ADDR=127.0.0.1:50051
# ... 其他服务 ...
```

---

## 🔐 安全建议

### ⚠️ 不要做的事

```bash
# ❌ 不要在 .env 中保存真实的生产密钥
# ❌ 不要提交 .env 文件到 Git
# ❌ 不要在公开仓库中暴露敏感信息
```

### ✅ 应该做的事

```bash
# ✅ 使用 .env.example 作为模板
# ✅ 每个开发者有自己的 .env 文件
# ✅ 生产环境使用密钥管理系统（如 AWS Secrets Manager）
# ✅ 定期轮换密钥和凭证
```

### 生产环境建议

```bash
# 不要在文件中存储敏感信息
# 而是使用环境变量或密钥管理系统

# 示例：使用 systemd 环境变量
# /etc/environment
# CHATIM_JWT_SECRET=<secure-key-from-vault>
# MYSQL_PASSWORD=<secure-password-from-vault>
```

---

## 🛠️ 环境变量加载顺序

Config 系统按以下顺序加载配置：

1. **config.yaml** - 基础配置文件
2. **环境变量** - 覆盖 YAML 中的对应值
3. **命令行参数** - 最高优先级（如果支持）

优先级：命令行 > 环境变量 > 配置文件

---

## 📝 常见问题

### Q: 如何为不同的环境使用不同的配置？

```bash
# 创建多个环境文件
.env                    # 本地开发
.env.staging           # 测试环境
.env.production        # 生产环境

# 加载特定文件
# 在 shell 中：
source .env.staging
go run ./cmd/api/main.go

# 或使用 docker-compose
docker-compose -f docker-compose.yml -f docker-compose.staging.yml up
```

### Q: 如何检查是否所有必需的环境变量都已设置？

```bash
# 创建检查脚本
#!/bin/bash
required_vars=(
  "CHATIM_SERVER_USER_GRPC_ADDR"
  "CHATIM_SERVER_MESSAGE_GRPC_ADDR"
  "MYSQL_HOST"
  "REDIS_HOST"
)

for var in "${required_vars[@]}"; do
  if [ -z "${!var}" ]; then
    echo "Error: $var is not set"
    exit 1
  fi
done
echo "All required variables are set ✅"
```

### Q: Docker Compose 不读取 .env 文件怎么办？

```bash
# 显式指定 env 文件
docker-compose --env-file docker-compose.env up

# 或在 docker-compose.yml 中指定
# services:
#   user-service:
#     env_file:
#       - docker-compose.env
```

---

## 📚 相关文件

- `pkg/config/config.go` - 配置加载代码
- `pkg/config/config.yaml` - YAML 配置文件
- `docker-compose.yml` - Docker 编排文件
- `.gitignore` - Git 忽略规则

---

**最后更新**: 2024年  
**状态**: ✅ 完成
