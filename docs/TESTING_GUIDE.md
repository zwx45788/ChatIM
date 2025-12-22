# ChatIM 测试指南

## 📋 测试文件说明

### 1. test_api.ps1 - API 集成测试
**功能**: 完整测试所有 HTTP API 端点
- 用户注册、登录、信息获取
- 好友请求、接受、列表查询
- 私聊消息发送、拉取
- 群组创建、消息发送、成员管理
- 会话列表
- 监控系统健康检查

**运行方式**:
```powershell
# 直接运行
.\test_api.ps1

# 或使用 PowerShell
powershell -ExecutionPolicy Bypass -File test_api.ps1
```

**输出**: 
- 终端彩色输出测试结果
- 生成 JSON 测试报告: `test_report_YYYYMMDD_HHmmss.json`

---

### 2. test_monitoring.ps1 - 监控系统测试
**功能**: 测试所有监控组件
- pprof 性能分析端点
- Prometheus Metrics 指标
- Prometheus 查询和目标状态
- Grafana 健康状态
- Alertmanager 告警
- 容器运行状态
- 性能基准测试

**运行方式**:
```powershell
.\test_monitoring.ps1
```

---

### 3. test/integration_test.go - Go 集成测试
**功能**: gRPC 服务集成测试
- 用户服务完整流程
- 并发请求测试
- 性能基准测试

**运行方式**:
```bash
# 运行所有测试
go test -v ./test/...

# 运行性能基准测试
go test -bench=. ./test/...

# 运行特定测试
go test -v ./test/ -run TestUserServiceIntegration
```

---

## 🚀 快速开始

### 前置条件
1. 确保所有服务已启动:
```powershell
docker-compose up -d
```

2. 等待服务完全启动 (约 10-15 秒):
```powershell
docker-compose ps
```

### 运行完整测试套件

```powershell
# 1. 运行 API 测试
.\test_api.ps1

# 2. 运行监控测试
.\test_monitoring.ps1

# 3. 运行 Go 集成测试
go test -v ./test/...
```

---

## 📊 测试结果说明

### API 测试结果
测试会输出彩色的实时结果：
- ✅ 绿色 = 测试通过
- ❌ 红色 = 测试失败
- ℹ️ 蓝色 = 信息提示

最后会显示:
```
总测试数: 25
通过: 23
失败: 2
通过率: 92%
```

### 测试报告
JSON 格式的详细报告包含:
```json
{
  "Test": "User Registration (User1)",
  "Success": true,
  "Message": "",
  "Time": "14:30:25"
}
```

---

## 🔍 故障排查

### 问题：测试全部失败

**检查服务状态**:
```powershell
docker-compose ps
```

**查看服务日志**:
```powershell
docker-compose logs api-gateway
docker-compose logs user-service
```

**重启服务**:
```powershell
docker-compose restart
```

---

### 问题：部分测试失败

**常见原因**:

1. **用户已存在**: 测试使用时间戳生成用户名，如果系统时钟有问题可能冲突
   - 解决: 清理数据库或等待几秒重试

2. **端口未开放**: 
   ```powershell
   # 检查端口监听
   netstat -an | Select-String "8081|9090|6060"
   ```

3. **网络延迟**: 
   - 增加脚本中的 `Start-Sleep` 时间
   - 或在本地运行服务而非 Docker

---

### 问题：Go 测试失败

**检查 Go 环境**:
```bash
go version
go mod tidy
```

**安装测试依赖**:
```bash
go get github.com/stretchr/testify/assert
```

---

## � 压力测试

### 使用 PowerShell 压力测试脚本

```powershell
# 基础压力测试 (20用户 x 5请求 = 100总请求)
.\test_stress_simple.ps1

# 中等强度 (50用户 x 10请求 = 500总请求)
.\test_stress_simple.ps1 -Users 50 -Requests 10

# 高强度 (100用户 x 20请求 = 2000总请求)
.\test_stress_simple.ps1 -Users 100 -Requests 20
```

**测试输出指标**:
- 成功率 (Success Rate)
- 平均 QPS (每秒请求数)
- 平均延迟 (Avg Latency)
- 容器资源使用 (CPU/Memory)
- Goroutines 数量
- 堆内存使用

**性能基准**:
- 成功率 > 95%: 优秀
- 成功率 80-95%: 良好
- 成功率 < 80%: 需要优化
- QPS > 50: 优秀
- QPS 10-50: 良好
- QPS < 10: 需要优化

---

## �📈 性能测试

### 使用 Go Benchmark
```bash
# 运行基准测试
go test -bench=. -benchtime=10s ./test/...

# 生成 CPU profile
go test -bench=. -cpuprofile=cpu.prof ./test/...

# 分析 profile
go tool pprof cpu.prof
```

### 使用 Apache Bench (ab)
```bash
# 测试健康检查端点
ab -n 1000 -c 10 http://localhost:8081/api/v1/health

# 测试登录接口 (需要先准备测试数据)
ab -n 100 -c 5 -p login.json -T application/json http://localhost:8081/api/v1/login
```

---

## 🎯 CI/CD 集成

### GitHub Actions 示例
```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Start services
        run: docker-compose up -d
      
      - name: Wait for services
        run: Start-Sleep -Seconds 30
      
      - name: Run API tests
        run: .\test_api.ps1
      
      - name: Run monitoring tests
        run: .\test_monitoring.ps1
      
      - name: Upload test reports
        uses: actions/upload-artifact@v3
        with:
          name: test-reports
          path: test_report_*.json
```

---

## 📚 扩展测试

### 添加新的 API 测试
在 `test_api.ps1` 中添加:
```powershell
Write-Section "8. 新功能测试"

Write-Info "测试新功能..."
$result = Invoke-ApiRequest -Method POST -Endpoint "/api/v1/new-feature" -Token $token1 -Body @{
    param1 = "value1"
}

if ($result.Success -and $result.Data.code -eq 200) {
    Write-Success "新功能测试通过"
    Add-TestResult "New Feature" $true
} else {
    Write-Error "新功能测试失败"
    Add-TestResult "New Feature" $false
}
```

### 添加压力测试
创建 `test_stress.ps1`:
```powershell
# 模拟 100 个并发用户
$concurrency = 100
$jobs = @()

for ($i = 1; $i -le $concurrency; $i++) {
    $jobs += Start-Job -ScriptBlock {
        # 测试逻辑
    }
}

$jobs | Wait-Job | Receive-Job
```

---

## ⚠️ 注意事项

1. **测试环境隔离**: 建议使用独立的测试数据库
2. **清理测试数据**: 测试后清理生成的测试用户和数据
3. **并发测试**: 注意数据库连接池大小限制
4. **时间同步**: Docker 容器与宿主机时间应同步
5. **端口冲突**: 确保测试端口未被占用

---

## 📞 支持

如果遇到问题:
1. 查看详细日志: `docker-compose logs -f`
2. 查看测试报告: `test_report_*.json`
3. 检查文档: `docs/*.md`

---

**测试愉快！** 🎉
