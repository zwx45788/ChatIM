# ChatIM 监控系统快速启动指南

## 🚀 一键启动

```bash
# 1. 进入项目目录
cd d:\git-demo\ChatIM

# 2. 启动完整服务（包括监控）
docker-compose up -d

# 3. 检查服务状态
docker-compose ps
```

## 📊 访问监控面板

| 服务 | 地址 | 说明 |
|------|------|------|
| **Grafana** | http://localhost:3000 | 用户名: `admin` 密码: `admin123` |
| **Prometheus** | http://localhost:9091 | 指标查询和告警规则 |
| **Alertmanager** | http://localhost:9093 | 告警管理 |
| **API Gateway** | http://localhost:8081 | HTTP API 接口 |
| **pprof** | http://localhost:6060/debug/pprof/ | 性能分析（已修复✅） |
| **Metrics** | http://localhost:9090/metrics | 原始指标数据（已修复✅） |

## ✅ 验证监控系统

### 1. 检查 Prometheus 采集状态
访问 http://localhost:9091/targets，确保所有 target 状态为 **UP**

### 2. 检查 Grafana 数据源
1. 登录 Grafana
2. Configuration → Data Sources
3. 点击 "Prometheus"，点击 "Test" 按钮
4. 应该显示 "Data source is working"

### 3. 测试指标采集
在 Prometheus 中执行查询：
```promql
up
chatim_http_requests_total
chatim_go_goroutines
```

## 📈 查看监控数据

### Grafana 预配置面板（开发中）
- 系统概览
- 消息服务详情
- 性能分析

### 自定义查询示例

在 Prometheus 或 Grafana 中尝试：

```promql
# QPS
sum(rate(chatim_http_requests_total[1m]))

# 错误率
sum(rate(chatim_http_requests_total{status=~"5.."}[5m])) 
/ 
sum(rate(chatim_http_requests_total[5m]))

# P95 延迟
histogram_quantile(0.95, 
  sum(rate(chatim_http_request_duration_seconds_bucket[5m])) by (le)
)

# 当前 goroutine 数
chatim_go_goroutines

# 内存使用
chatim_go_memory_heap_bytes
```

## 🔍 使用 pprof 分析性能

```bash
# CPU 分析（30 秒）
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 内存分析
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine 分析
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

## 🚨 配置告警通知

编辑 `monitoring/alertmanager/alertmanager.yml`，取消注释并配置：

### 钉钉
```yaml
webhook_configs:
  - url: 'https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN'
```

### 邮件
```yaml
email_configs:
  - to: 'admin@example.com'
    from: 'alertmanager@example.com'
    smarthost: 'smtp.gmail.com:587'
    auth_username: 'your-email@gmail.com'
    auth_password: 'your-password'
```

重启 Alertmanager：
```bash
docker-compose restart alertmanager
```

## 📚 完整文档

详细使用说明请查看：[MONITORING_GUIDE.md](./MONITORING_GUIDE.md)

## 🛠️ 故障排查

### 问题：Grafana 显示 "No Data"
```bash
# 检查 Prometheus 是否正常
curl http://localhost:9091/-/healthy

# 检查 targets 状态
curl http://localhost:9091/api/v1/targets
```

### 问题：pprof 无法访问
```bash
# 检查 API Gateway 日志
docker logs chatim_api_gateway | grep pprof

# 或本地运行时查看控制台输出
```

### 问题：告警不生效
```bash
# 检查 Prometheus 告警规则
curl http://localhost:9091/api/v1/rules

# 检查 Alertmanager 状态
curl http://localhost:9093/api/v1/status
```

## 🎯 下一步

1. ✅ 启动监控系统
2. ✅ 验证数据采集
3. ⏳ 根据业务调整告警阈值
4. ⏳ 创建自定义 Grafana 仪表盘
5. ⏳ 配置告警通知渠道

---

**监控系统已就绪！** 🎉
