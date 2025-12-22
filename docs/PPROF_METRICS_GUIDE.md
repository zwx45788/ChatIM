# pprof 和 Metrics 访问指南

## ✅ 问题已解决

**问题原因**：pprof 服务之前监听 `localhost:6060`（仅容器内部可访问），现已修复为监听 `0.0.0.0:6060`（可从宿主机访问）。

**修复内容**：
1. ✅ 修改 [pkg/profiling/profiling.go](../pkg/profiling/profiling.go) - pprof 监听地址改为 `0.0.0.0`
2. ✅ 更新 [docker-compose.yml](../docker-compose.yml) - 添加端口映射 `6060:6060` 和 `9090:9090`
3. ✅ 重新构建并启动服务

## 📊 访问服务

### 1. pprof 性能分析
**地址**: http://localhost:6060/debug/pprof/

**可用的分析端点**：
- http://localhost:6060/debug/pprof/ - 概览页面
- http://localhost:6060/debug/pprof/heap - 堆内存分析
- http://localhost:6060/debug/pprof/goroutine - Goroutine 分析
- http://localhost:6060/debug/pprof/profile?seconds=30 - CPU 性能分析（30秒）
- http://localhost:6060/debug/pprof/block - 阻塞分析
- http://localhost:6060/debug/pprof/mutex - 互斥锁分析
- http://localhost:6060/debug/pprof/allocs - 内存分配分析

### 2. Prometheus Metrics
**地址**: http://localhost:9090/metrics

**验证 metrics 是否工作**：
```powershell
# 使用 curl 查看 metrics
curl http://localhost:9090/metrics | Select-String "chatim_"

# 或在浏览器中直接访问
start http://localhost:9090/metrics
```

**主要指标类别**：
- `chatim_http_*` - HTTP 请求指标
- `chatim_messages_*` - 消息相关指标
- `chatim_websocket_*` - WebSocket 连接指标
- `chatim_redis_*` - Redis 操作指标
- `chatim_db_*` - 数据库查询指标
- `chatim_go_*` - Go 运行时指标

## 🔍 使用 pprof 进行性能分析

### 方法 1：浏览器查看（简单）

直接在浏览器打开 http://localhost:6060/debug/pprof/，可以看到各种性能数据的链接。

### 方法 2：命令行分析（专业）

#### CPU 分析
```powershell
# 收集 30 秒的 CPU 性能数据
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 进入交互模式后可以使用：
# top10 - 查看前 10 个热点函数
# list funcName - 查看具体函数的代码
# web - 生成调用图（需要安装 Graphviz）
# pdf - 生成 PDF 报告
```

#### 内存分析
```powershell
# 堆内存分析
go tool pprof http://localhost:6060/debug/pprof/heap

# 内存分配分析
go tool pprof http://localhost:6060/debug/pprof/allocs
```

#### Goroutine 分析
```powershell
# 查看所有 goroutine
go tool pprof http://localhost:6060/debug/pprof/goroutine

# 或直接下载查看
curl http://localhost:6060/debug/pprof/goroutine?debug=2 -o goroutines.txt
```

#### 生成可视化图表
```powershell
# 需要先安装 Graphviz: https://graphviz.org/download/
# 然后生成 CPU 火焰图
go tool pprof -http=:8888 http://localhost:6060/debug/pprof/profile?seconds=30
# 会在浏览器中打开 http://localhost:8888 显示交互式图表
```

## 📈 查看日志

### API Gateway 日志
```powershell
# 查看完整日志
docker logs chatim_api_gateway

# 实时跟踪日志
docker logs -f chatim_api_gateway

# 查看最近 50 条
docker logs --tail 50 chatim_api_gateway

# 查看 pprof 和 metrics 启动信息
docker logs chatim_api_gateway 2>&1 | Select-String "pprof|metrics|Prometheus"
```

**应该看到类似输出**：
```json
{"level":"INFO","ts":"2025-12-16T16:54:56.006Z","caller":"profiling/profiling.go:21","msg":"🔍 pprof server started","addr":"http://localhost:6060/debug/pprof/"}
{"level":"INFO","ts":"2025-12-16T16:54:56.008Z","caller":"api/main.go:42","msg":"📊 Prometheus metrics server started at http://localhost:9090/metrics"}
```

### 所有服务日志
```powershell
# 查看所有服务
docker-compose logs

# 实时查看所有服务
docker-compose logs -f

# 查看特定服务组合
docker-compose logs -f api-gateway user-service message-service
```

详细日志使用指南请参考：[LOGGING_QUICKSTART.md](./LOGGING_QUICKSTART.md)

## 🛠️ 故障排查

### 问题：pprof 无法访问

**检查服务是否启动**：
```powershell
docker ps | Select-String "api"
```

**检查端口映射**：
```powershell
docker port chatim_api_gateway
```
应该看到：
```
6060/tcp -> 0.0.0.0:6060
8080/tcp -> 0.0.0.0:8081
9090/tcp -> 0.0.0.0:9090
```

**检查容器内部端口监听**：
```powershell
docker exec chatim_api_gateway netstat -tlnp | Select-String "6060"
```
应该看到：
```
tcp        0      0 :::6060                 :::*                    LISTEN      1/api-gateway
```

**查看启动日志**：
```powershell
docker logs chatim_api_gateway | Select-String "pprof"
```

### 问题：Metrics 无法访问

**测试连接**：
```powershell
# 使用 curl 测试
curl http://localhost:9090/metrics

# 使用 PowerShell
Invoke-WebRequest -Uri http://localhost:9090/metrics -UseBasicParsing
```

**检查是否有数据**：
```powershell
curl http://localhost:9090/metrics | Select-String "chatim_" | Measure-Object
```

### 问题：需要重启服务

```powershell
# 重启 API Gateway
docker-compose restart api-gateway

# 重启所有服务
docker-compose restart

# 完全重建（如果修改了代码）
docker-compose up -d --build api-gateway
```

## 🎯 常见使用场景

### 场景 1：查找内存泄漏
```powershell
# 1. 运行程序一段时间后抓取堆快照
go tool pprof -http=:8888 http://localhost:6060/debug/pprof/heap

# 2. 在浏览器中查看：
#    - Top：查看内存占用最多的函数
#    - Graph：查看调用关系
#    - Flame Graph：火焰图直观展示
```

### 场景 2：分析 CPU 热点
```powershell
# 收集 60 秒 CPU 数据
go tool pprof -http=:8888 http://localhost:6060/debug/pprof/profile?seconds=60

# 在交互模式查看：
# top20 - 前 20 个耗时函数
# list functionName - 查看具体代码行
```

### 场景 3：排查 Goroutine 泄漏
```powershell
# 查看当前 goroutine 数量
curl http://localhost:6060/debug/pprof/goroutine?debug=2

# 或使用 pprof
go tool pprof http://localhost:6060/debug/pprof/goroutine
# 然后执行 top 查看创建最多 goroutine 的函数
```

### 场景 4：监控指标查询
```powershell
# 查看 HTTP 请求总数
curl http://localhost:9090/metrics | Select-String "chatim_http_requests_total"

# 查看当前 goroutine 数量
curl http://localhost:9090/metrics | Select-String "chatim_go_goroutines"

# 查看内存使用
curl http://localhost:9090/metrics | Select-String "chatim_go_memory"
```

## 📚 相关文档

- [完整监控指南](./MONITORING_GUIDE.md)
- [日志查看指南](./LOGGING_QUICKSTART.md)
- [Go pprof 官方文档](https://pkg.go.dev/runtime/pprof)
- [Prometheus 指标类型](https://prometheus.io/docs/concepts/metric_types/)

## ⚠️ 注意事项

1. **生产环境安全**：pprof 端点暴露了程序内部信息，生产环境应该：
   - 限制访问 IP（通过防火墙或 Nginx）
   - 使用认证机制
   - 仅在需要时临时开启

2. **性能影响**：
   - CPU profiling 会有 5-10% 的性能开销
   - 内存分析基本无开销
   - 避免同时运行多个 profiling

3. **数据保存**：
   - pprof 数据可以保存为文件供后续分析
   - 使用 `-output` 参数保存结果

---

**更新时间**: 2025-12-17  
**状态**: ✅ 已修复并验证
