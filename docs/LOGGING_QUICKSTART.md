# 日志查看快速指南

## 📋 当前日志配置

所有服务已迁移至 **Zap 结构化日志系统**，日志以 JSON 格式输出，便于解析和分析。

## 🔍 查看 Docker 容器日志

### 查看单个服务日志

```powershell
# User Service
docker logs chatim_user_service

# Message Service
docker logs chatim_message_service

# Group Service
docker logs chatim_group_service

# Friendship Service
docker logs chatim_friendship_service

# API Gateway
docker logs chatim_api_gateway
```

### 实时跟踪日志（类似 tail -f）

```powershell
# 跟踪单个服务
docker logs -f chatim_user_service

# 跟踪多个服务
docker-compose logs -f user-service message-service

# 跟踪所有服务
docker-compose logs -f
```

### 查看最近的日志

```powershell
# 查看最近 50 条日志
docker logs --tail 50 chatim_user_service

# 查看最近 5 分钟的日志
docker logs --since 5m chatim_user_service

# 查看指定时间后的日志
docker logs --since 2025-12-17T00:00:00 chatim_user_service
```

## 📊 日志格式说明

### JSON 格式日志
```json
{
  "level": "INFO",
  "ts": "2025-12-16T16:45:51.411Z",
  "caller": "user/main.go:76",
  "msg": "🚀 User Service gRPC server started",
  "port": ":50051"
}
```

**字段说明：**
- `level`: 日志级别（DEBUG, INFO, WARN, ERROR, FATAL）
- `ts`: 时间戳（ISO 8601 格式）
- `caller`: 调用位置（文件:行号）
- `msg`: 日志消息
- 其他字段: 结构化数据（如 `port`, `user_id`, `error` 等）

## 🎯 常用日志查询

### 1. 查看错误日志

```powershell
# 筛选包含 "error" 的日志
docker logs chatim_user_service 2>&1 | Select-String "error" -CaseSensitive

# 查看 ERROR 级别的日志
docker logs chatim_user_service 2>&1 | Select-String '"level":"ERROR"'
```

### 2. 查看特定用户的日志

```powershell
# 查找包含特定 user_id 的日志
docker logs chatim_message_service 2>&1 | Select-String "user_12345"
```

### 3. 查看服务启动日志

```powershell
# 查看服务启动信息
docker logs chatim_user_service 2>&1 | Select-String "starting|started"
```

### 4. 导出日志到文件

```powershell
# 导出单个服务日志
docker logs chatim_user_service > logs/user_service.log 2>&1

# 导出所有服务日志
docker-compose logs --no-color > logs/all_services.log 2>&1
```

### 5. 查看服务健康状态

```powershell
# 查看最近的启动/错误信息
docker logs --tail 100 chatim_user_service 2>&1 | Select-String "started|error|failed"
```

## 🔄 实时监控所有服务

```powershell
# 同时查看所有服务的实时日志
docker-compose logs -f --tail=10

# 仅查看核心服务
docker-compose logs -f user-service message-service api-gateway
```

## 📁 日志持久化配置

如果需要将日志保存到文件（而不仅仅是容器日志），可以配置 `config.yaml`：

```yaml
log:
  level: "info"
  output_path: "logs/app.log"  # 容器内路径
  dev_mode: false
```

然后在 `docker-compose.yml` 中添加卷映射：

```yaml
services:
  user-service:
    volumes:
      - ./logs/user:/root/logs  # 将容器日志挂载到宿主机
```

## 🛠️ 故障排查

### 问题：看不到日志

**检查容器状态：**
```powershell
docker ps -a | Select-String "user"
```

**查看容器是否正在运行：**
```powershell
docker-compose ps
```

**重启服务：**
```powershell
docker-compose restart user-service
```

### 问题：日志太多

**限制日志输出：**
```powershell
# 只看最近 20 条
docker logs --tail 20 chatim_user_service

# 只看 INFO 级别以上
docker logs chatim_user_service 2>&1 | Select-String '"level":"INFO"|"level":"WARN"|"level":"ERROR"'
```

### 问题：需要更详细的日志

**临时调整日志级别：**

修改 `config.yaml`：
```yaml
log:
  level: "debug"  # 改为 debug
```

然后重启服务：
```powershell
docker-compose restart
```

## 📈 与监控系统集成

### Prometheus 指标
访问：http://localhost:9091

### Grafana 可视化
访问：http://localhost:3000
- 默认账号: `admin` / `admin`

### pprof 性能分析
访问：http://localhost:6060/debug/pprof/

## 💡 最佳实践

1. **开发环境**：使用 `docker-compose logs -f` 实时查看
2. **生产环境**：配置日志文件输出 + 日志收集系统（ELK/Loki）
3. **故障排查**：先看 ERROR 日志，再看上下文
4. **性能监控**：结合 Prometheus + Grafana 查看指标
5. **定期清理**：使用 `docker system prune` 清理旧日志

## 🔗 相关文档

- [完整日志使用指南](./LOGGING_GUIDE.md)
- [监控系统快速开始](./MONITORING_QUICKSTART.md)
- [监控完整指南](./MONITORING_GUIDE.md)

---
**更新时间**: 2025-12-17
