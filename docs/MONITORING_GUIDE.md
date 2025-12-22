# ChatIM 监控系统完整指南

## 📊 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    ChatIM 监控系统架构                       │
└─────────────────────────────────────────────────────────────┘

    应用层                采集层              存储层         展示层
    
┌──────────┐         ┌──────────┐        ┌──────────┐    ┌──────────┐
│ API      │────────▶│ Metrics  │───────▶│Prometheus│───▶│ Grafana  │
│ Gateway  │ metrics │ Exporter │ scrape │          │ QL │          │
└──────────┘         └──────────┘        └──────────┘    └──────────┘
                                               │
┌──────────┐         ┌──────────┐             │           ┌──────────┐
│ Message  │────────▶│  pprof   │             ├──────────▶│Alerting  │
│ Service  │ profile │  HTTP    │             │  rules    │ Manager  │
└──────────┘         └──────────┘             │           └──────────┘
                                               │
┌──────────┐                                   │           ┌──────────┐
│   User   │                                   └──────────▶│  钉钉    │
│ Service  │                                    trigger    │  邮件    │
└──────────┘                                               └──────────┘
```

---

## 🚀 快速开始

### 1. 安装依赖

```bash
# 进入项目目录
cd d:\git-demo\ChatIM

# 安装 Go 依赖
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
```

### 2. 启动监控服务

```bash
# 使用 Docker Compose 启动完整服务栈
docker-compose up -d

# 查看服务状态
docker-compose ps
```

### 3. 访问监控面板

| 服务 | URL | 用户名 | 密码 |
|------|-----|--------|------|
| **Grafana** | http://localhost:3000 | admin | admin123 |
| **Prometheus** | http://localhost:9091 | - | - |
| **Alertmanager** | http://localhost:9093 | - | - |
| **pprof (API Gateway)** | http://localhost:6060/debug/pprof/ | - | - |
| **Metrics (API Gateway)** | http://localhost:9090/metrics | - | - |

---

## 📈 Prometheus 指标详解

### HTTP 服务指标

```promql
# 请求总数（按方法、端点、状态码分组）
chatim_http_requests_total{method="POST", endpoint="/api/v1/messages/send", status="200"}

# 请求延迟（直方图）
chatim_http_request_duration_seconds{method="POST", endpoint="/api/v1/messages/send"}

# 请求/响应大小
chatim_http_request_size_bytes
chatim_http_response_size_bytes
```

**常用查询：**

```promql
# QPS（每秒请求数）
sum(rate(chatim_http_requests_total[1m]))

# 错误率
sum(rate(chatim_http_requests_total{status=~"5.."}[5m])) 
/ 
sum(rate(chatim_http_requests_total[5m]))

# P95 延迟
histogram_quantile(0.95, 
  sum(rate(chatim_http_request_duration_seconds_bucket[5m])) by (le)
)

# P99 延迟
histogram_quantile(0.99, 
  sum(rate(chatim_http_request_duration_seconds_bucket[5m])) by (le)
)
```

### 消息业务指标

```promql
# 消息发送总数
chatim_messages_sent_total{type="private", status="success"}
chatim_messages_sent_total{type="group", status="failed"}

# 消息发送延迟
chatim_message_send_duration_seconds{type="private"}

# 未读消息数（按用户）
chatim_unread_messages_count{user_id="user123"}

# Redis Stream 积压
chatim_redis_stream_pending_messages{stream_key="stream:private:user123"}
```

**常用查询：**

```promql
# 私聊消息发送速率（每分钟）
sum(rate(chatim_messages_sent_total{type="private"}[1m])) * 60

# 消息发送失败率
sum(rate(chatim_messages_sent_total{status="failed"}[5m])) 
/ 
sum(rate(chatim_messages_sent_total[5m]))

# 平均消息发送延迟
rate(chatim_message_send_duration_seconds_sum[5m])
/
rate(chatim_message_send_duration_seconds_count[5m])
```

### WebSocket 指标

```promql
# 当前活跃连接数
chatim_websocket_active_connections

# 消息推送总数
chatim_websocket_messages_pushed_total{type="private", status="success"}

# 连接持续时间
chatim_websocket_connection_duration_seconds
```

### Redis 指标

```promql
# Redis 操作总数
chatim_redis_operations_total{operation="xadd", status="success"}

# Redis 操作延迟
chatim_redis_operation_duration_seconds{operation="xread"}

# Redis 连接池状态
chatim_redis_pool_connections{state="idle"}
```

### 数据库指标

```promql
# 数据库查询总数
chatim_db_queries_total{operation="select", table="messages", status="success"}

# 查询延迟
chatim_db_query_duration_seconds{operation="insert", table="users"}

# 连接池状态
chatim_db_connection_pool{state="open"}
chatim_db_connection_pool{state="in_use"}
```

### Go 运行时指标

```promql
# Goroutine 数量
chatim_go_goroutines

# 内存使用
chatim_go_memory_alloc_bytes
chatim_go_memory_heap_bytes

# GC 暂停时间
chatim_go_gc_pause_duration_seconds
```

**常用查询：**

```promql
# 内存增长率（每小时）
rate(chatim_go_memory_heap_bytes[1h]) * 3600

# GC 频率（每分钟）
rate(go_gc_duration_seconds_count[1m]) * 60
```

---

## 🔍 pprof 性能分析

### 访问 pprof

```bash
# API Gateway pprof 端点
http://localhost:6060/debug/pprof/

# 可用的 profile 类型：
# - heap: 堆内存分配
# - goroutine: 当前所有 goroutine 栈
# - threadcreate: 导致创建新 OS 线程的栈
# - block: 导致阻塞的栈
# - mutex: 锁竞争的栈
# - profile: CPU profile（需要采集 30 秒）
```

### 常用分析命令

#### 1. CPU 性能分析

```bash
# 采集 30 秒 CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# 进入交互模式后：
(pprof) top10              # 查看 CPU 占用 top10
(pprof) list SendMessage   # 查看 SendMessage 函数详细信息
(pprof) web                # 生成调用图（需要 graphviz）
(pprof) pdf                # 生成 PDF 报告
(pprof) png                # 生成 PNG 图片
```

#### 2. 内存分析

```bash
# 分析当前堆内存
go tool pprof http://localhost:6060/debug/pprof/heap

(pprof) top               # 内存分配 top10
(pprof) list PullMessages # 查看 PullMessages 函数内存分配
(pprof) web               # 可视化

# 分析分配对象（allocs）
go tool pprof http://localhost:6060/debug/pprof/allocs
```

#### 3. Goroutine 分析

```bash
# 查看所有 goroutine
go tool pprof http://localhost:6060/debug/pprof/goroutine

(pprof) top               # goroutine 数量 top10
(pprof) traces            # 查看调用栈

# 或直接查看文本
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

#### 4. 锁竞争分析

```bash
# 分析锁竞争
go tool pprof http://localhost:6060/debug/pprof/mutex

(pprof) top
(pprof) list              # 查看竞争最激烈的代码
```

#### 5. 阻塞分析

```bash
# 分析阻塞点
go tool pprof http://localhost:6060/debug/pprof/block

(pprof) top
```

### 火焰图生成

```bash
# 安装 go-torch（可选）
go get github.com/uber/go-torch

# 生成 CPU 火焰图
go-torch http://localhost:6060/debug/pprof/profile

# 生成内存火焰图
go-torch --alloc_space http://localhost:6060/debug/pprof/heap
```

---

## 📊 Grafana 仪表盘使用

### 预配置仪表盘

系统提供以下预配置仪表盘（位于 `monitoring/grafana/dashboards/`）：

1. **系统概览 (Overview)**
   - QPS、错误率、延迟
   - 内存、CPU、Goroutine 趋势
   - WebSocket 连接数

2. **消息服务 (Message Service)**
   - 私聊/群聊消息发送速率
   - 消息发送成功率
   - Redis Stream 积压情况

3. **性能详情 (Performance)**
   - P50/P95/P99 延迟分布
   - 数据库查询性能
   - Redis 操作性能

### 创建自定义仪表盘

1. 登录 Grafana (http://localhost:3000)
2. 点击 "+" → "Dashboard"
3. 点击 "Add new panel"
4. 编写 PromQL 查询

**示例面板：**

```json
{
  "title": "消息发送 QPS",
  "targets": [
    {
      "expr": "sum(rate(chatim_messages_sent_total[1m])) * 60",
      "legendFormat": "总 QPS"
    },
    {
      "expr": "sum(rate(chatim_messages_sent_total{type=\"private\"}[1m])) * 60",
      "legendFormat": "私聊 QPS"
    }
  ]
}
```

---

## 🚨 告警配置

### 告警规则说明

系统预配置了以下告警（`monitoring/prometheus/alert-rules.yml`）：

| 告警名称 | 触发条件 | 严重级别 |
|---------|---------|---------|
| **HighHTTPErrorRate** | 5xx 错误率 > 5%，持续 5 分钟 | Critical |
| **HighHTTPLatency** | P95 延迟 > 1s，持续 10 分钟 | Warning |
| **HighMessageSendFailureRate** | 消息发送失败率 > 10% | Critical |
| **RedisStreamBacklog** | Stream 积压 > 1000 条 | Warning |
| **HighMemoryUsage** | 堆内存 > 1GB | Warning |
| **HighGoroutineCount** | Goroutine > 10000 | Warning |
| **ServiceDown** | 服务停止响应 > 1 分钟 | Critical |

### 配置告警通知

编辑 `monitoring/alertmanager/alertmanager.yml`：

#### 钉钉通知

```yaml
receivers:
  - name: 'critical-receiver'
    webhook_configs:
      - url: 'https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN'
        send_resolved: true
```

#### 企业微信通知

```yaml
receivers:
  - name: 'critical-receiver'
    wechat_configs:
      - corp_id: 'YOUR_CORP_ID'
        to_party: '1'
        agent_id: 'YOUR_AGENT_ID'
        api_secret: 'YOUR_SECRET'
```

#### 邮件通知

```yaml
receivers:
  - name: 'critical-receiver'
    email_configs:
      - to: 'admin@example.com'
        from: 'alertmanager@example.com'
        smarthost: 'smtp.gmail.com:587'
        auth_username: 'your-email@gmail.com'
        auth_password: 'your-password'
```

---

## 🛠️ 典型问题排查流程

### 问题 1：发现延迟增加

```
1. Grafana 告警: P95 延迟从 50ms → 500ms

2. Prometheus 查询确认
   histogram_quantile(0.95, 
     chatim_http_request_duration_seconds_bucket{endpoint="/api/v1/messages/send"}
   )

3. pprof 抓取 CPU profile
   go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

4. 分析热点
   (pprof) top10
   # 发现：json.Marshal 占用 40% CPU

5. 查看具体代码
   (pprof) list json.Marshal
   # 定位到：notification 序列化

6. 优化方案
   - 使用对象池 (sync.Pool)
   - 或使用更快的 JSON 库（如 sonic）

7. 部署后验证
   # Grafana 显示：P95 延迟降至 60ms ✅
```

### 问题 2：内存持续增长

```
1. Grafana 发现内存持续上涨

2. 抓取堆内存快照
   go tool pprof http://localhost:6060/debug/pprof/heap

3. 分析内存占用
   (pprof) top
   # 发现：PullMessages 函数占用 512MB

4. 检查是否有内存泄漏
   (pprof) list PullMessages
   # 发现：XRevRangeN 返回了大量数据没有限制

5. 修复
   # 添加 limit 参数限制返回数量

6. 验证
   # 观察内存趋势平稳 ✅
```

### 问题 3：Goroutine 泄漏

```
1. Grafana 显示 goroutine 数量持续增长

2. 查看 goroutine 详情
   curl http://localhost:6060/debug/pprof/goroutine?debug=1 > goroutine.txt

3. 分析泄漏点
   cat goroutine.txt | grep "Created by"
   # 发现：大量 goroutine 卡在 redis.Publish

4. 定位代码
   # 异步发布消息的 goroutine 没有超时控制

5. 修复
   ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
   defer cancel()

6. 验证
   # goroutine 数量恢复正常 ✅
```

---

## 📦 文件结构

```
ChatIM/
├── pkg/
│   ├── metrics/
│   │   └── metrics.go              # Prometheus 指标定义
│   └── profiling/
│       └── profiling.go            # pprof 初始化
│
├── internal/
│   └── api_gateway/
│       └── middleware/
│           └── prometheus.go       # Prometheus 中间件
│
├── monitoring/
│   ├── prometheus/
│   │   ├── prometheus.yml          # Prometheus 配置
│   │   └── alert-rules.yml         # 告警规则
│   │
│   ├── grafana/
│   │   ├── provisioning/
│   │   │   ├── datasources/        # 数据源配置
│   │   │   └── dashboards/         # 仪表盘配置
│   │   └── dashboards/             # 预配置仪表盘
│   │
│   └── alertmanager/
│       └── alertmanager.yml        # Alertmanager 配置
│
└── docker-compose.yml              # Docker Compose 配置
```

---

## 🎯 最佳实践

### 1. 指标采集

✅ **推荐做法：**
- 为关键业务操作添加指标（发消息、拉消息、加好友等）
- 使用直方图（Histogram）记录延迟
- 为高基数标签（如 user_id）设置合理限制

❌ **避免：**
- 在热路径上进行复杂计算
- 使用过多的标签维度
- 在循环中频繁调用指标更新

### 2. pprof 使用

✅ **推荐做法：**
- 在生产环境使用独立端口（如 localhost:6060）
- 定期采集性能数据作为基线
- 结合 Prometheus 告警触发 pprof 分析

❌ **避免：**
- 长时间开启高频率的 profile
- 在生产环境暴露 pprof 到公网

### 3. 告警配置

✅ **推荐做法：**
- 告警应该可操作（Actionable）
- 设置合理的阈值和持续时间
- 区分严重级别（Critical / Warning）
- 避免告警疲劳

❌ **避免：**
- 过于敏感的告警阈值
- 没有分组的告警轰炸
- 缺少恢复通知

---

## 🔧 故障排查速查表

| 现象 | 可能原因 | 排查工具 |
|------|---------|---------|
| 延迟突增 | CPU 瓶颈、数据库慢查询、网络问题 | pprof (CPU), Prometheus (查询延迟) |
| 内存增长 | 内存泄漏、缓存过大、goroutine 泄漏 | pprof (heap, goroutine) |
| 错误率上升 | 依赖服务故障、资源耗尽、代码 bug | Prometheus (错误指标), 日志 |
| QPS 下降 | 客户端问题、负载均衡问题 | Prometheus (QPS 趋势) |
| 连接数异常 | WebSocket 重连风暴、客户端泄漏 | Prometheus (连接数), pprof (goroutine) |

---

## 📚 参考资源

- [Prometheus 官方文档](https://prometheus.io/docs/)
- [Grafana 官方文档](https://grafana.com/docs/)
- [Go pprof 使用指南](https://github.com/google/pprof/blob/master/doc/README.md)
- [PromQL 查询语言](https://prometheus.io/docs/prometheus/latest/querying/basics/)

---

## ❓ 常见问题

### Q1: 为什么 Grafana 显示 "No Data"?

**A:** 检查：
1. Prometheus 是否正常采集数据：访问 http://localhost:9091/targets
2. 数据源配置是否正确：Grafana → Configuration → Data Sources
3. PromQL 查询是否正确

### Q2: pprof 页面无法访问？

**A:** 检查：
1. 服务是否启用了 pprof：查看日志 "🔍 pprof server started"
2. 端口是否被占用：`netstat -ano | findstr 6060`
3. 防火墙是否阻止访问

### Q3: Alertmanager 没有发送告警？

**A:** 检查：
1. Prometheus 是否正确加载告警规则：http://localhost:9091/rules
2. Alertmanager 配置是否正确：http://localhost:9093/#/status
3. 接收器配置是否正确（webhook URL、邮箱等）

### Q4: 指标数据太多，Prometheus 性能下降？

**A:** 优化措施：
1. 减少高基数标签（如 user_id）
2. 使用 `metric_relabel_configs` 删除不需要的指标
3. 调整采集间隔（scrape_interval）
4. 使用 Prometheus 联邦集群

---

## 🎉 总结

**你现在拥有的监控能力：**

✅ **实时监控**
- HTTP 请求 QPS、延迟、错误率
- 消息发送速率、成功率
- WebSocket 连接数
- 资源使用（CPU、内存、Goroutine）

✅ **性能分析**
- CPU 热点分析
- 内存分配分析
- Goroutine 泄漏检测
- 锁竞争分析

✅ **告警通知**
- 多级别告警（Critical / Warning）
- 多种通知方式（钉钉、邮件、企业微信）
- 告警聚合和抑制

✅ **可视化**
- Grafana 实时仪表盘
- 火焰图
- 调用图

**下一步：**
1. 根据业务需求调整告警阈值
2. 创建自定义 Grafana 仪表盘
3. 配置告警通知渠道
4. 定期分析性能数据，持续优化

---

**监控系统部署成功！** 🚀

如有问题，请查看：
- Prometheus 日志：`docker logs chatim_prometheus`
- Grafana 日志：`docker logs chatim_grafana`
- Alertmanager 日志：`docker logs chatim_alertmanager`
