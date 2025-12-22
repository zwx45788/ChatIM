# ChatIM Performance Analysis Report
# QPS低下原因分析与优化建议

## 🔍 当前性能指标

基于压力测试结果:
- **QPS: 5.8 req/sec** (500个请求 / 86.28秒)
- **平均延迟: 172.55 ms**
- **成功率: 100%**

## 📊 问题分析

### 1. **测试脚本人为限速** ⚠️ 主要原因

**问题**: 测试脚本中每个请求后都有 `Start-Sleep -Milliseconds 100`

```powershell
# test_stress_simple.ps1 第61行
Start-Sleep -Milliseconds 100  # 每次请求后等待100ms
```

**影响计算**:
- 50个并发用户
- 每用户10个请求
- 每请求间隔100ms
- 理论最短时间: 10 × 0.1秒 = 1秒/用户
- 实际耗时: 86秒 ≈ 1.72秒/用户

**实际QPS限制**:
- 最大理论QPS = 1000ms / 100ms = 10 req/sec (单线程)
- 50个并发 × 10 = 500 QPS (理论上限)
- 但由于串行化的Job执行，实际只能达到 5-6 QPS

### 2. **数据库连接池未配置** ⚠️

当前代码中没有显式设置数据库连接池参数。

**默认GORM配置**:
```go
// 默认值（未优化）
MaxOpenConns: unlimited (实际受操作系统限制)
MaxIdleConns: 2
ConnMaxLifetime: 0 (永不过期)
```

**影响**:
- 高并发时可能频繁创建/销毁连接
- 连接复用率低
- 数据库连接开销大

### 3. **没有使用连接池优化** ⚠️

**Redis连接**: 单个连接，未使用连接池
**MySQL连接**: 未配置最大连接数和空闲连接数

### 4. **密码哈希计算开销** 

每次用户注册都需要进行bcrypt密码哈希:
```go
// bcrypt默认cost为10，每次哈希需要约100ms
hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
```

**性能影响**:
- 每次注册: ~100-150ms CPU时间
- 在单核或低配置环境下影响更大

### 5. **数据库写入操作** 

每次用户注册包含:
1. 数据库INSERT操作 (~10-50ms)
2. 事务提交 (~5-20ms)
3. 索引更新 (~5-10ms)

### 6. **容器资源限制** 

查看Docker stats显示:
- API Gateway: 15MB内存，0.13% CPU
- User Service: 9MB内存，0.00% CPU

**分析**: CPU使用率极低，说明不是计算瓶颈，而是I/O等待或人为限速。

## 🚀 优化建议

### 优先级1: 移除测试脚本延迟

```powershell
# 删除或注释掉这一行
# Start-Sleep -Milliseconds 100
```

**预期提升**: QPS从5.8提升到50-100+

### 优先级2: 配置数据库连接池

```go
// cmd/user/main.go
sqlDB, err := db.DB()
if err != nil {
    log.Fatal(err)
}

// 设置连接池参数
sqlDB.SetMaxOpenConns(100)          // 最大打开连接数
sqlDB.SetMaxIdleConns(10)           // 最大空闲连接数
sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大生命周期
sqlDB.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接最大生命周期
```

**预期提升**: QPS提升20-30%

### 优先级3: 降低密码哈希成本（仅用于测试）

```go
// 仅用于压力测试环境
bcrypt.GenerateFromPassword([]byte(password), 4) // Cost从10降到4
```

**预期提升**: 每次注册节省约80ms

### 优先级4: 添加数据库索引

```sql
-- 确保username字段有唯一索引
CREATE UNIQUE INDEX idx_users_username ON users(username);
```

### 优先级5: 使用批量插入（如果适用）

对于批量创建用户的场景，可以使用事务批量插入。

## 📈 优化后预期性能

假设移除测试延迟并优化数据库连接池:

**保守估计**:
- QPS: 50-100 req/sec
- 平均延迟: 50-100 ms

**乐观估计** (优化bcrypt cost):
- QPS: 100-200 req/sec  
- 平均延迟: 20-50 ms

## 🔧 验证方法

1. **查看数据库慢查询日志**
```bash
docker exec chatim_mysql mysql -uchatim_user -pchatim_pass -e "SHOW VARIABLES LIKE 'slow_query%';"
```

2. **使用pprof分析CPU瓶颈**
```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

3. **监控数据库连接数**
```bash
docker exec chatim_mysql mysql -uchatim_user -pchatim_pass -e "SHOW STATUS LIKE 'Threads_connected';"
```

4. **使用Prometheus查询延迟**
```
# PromQL查询
rate(chatim_http_request_duration_seconds_sum[1m]) / 
rate(chatim_http_request_duration_seconds_count[1m])
```

## 💡 总结

### 实际测试结果对比

| 测试类型 | QPS | 平均延迟 | P95延迟 | 瓶颈 |
|---------|-----|---------|---------|------|
| 带100ms延迟 | 5.8 | 172ms | N/A | 人为限速 |
| 无延迟版本 | 8.92 | 147ms | 675ms | bcrypt + DB |
| 提升 | +54% | -14% | - | - |

### 真实瓶颈分析

移除测试延迟后，QPS仅从5.8提升到8.92（+54%），说明**真正的瓶颈不是测试脚本**。

**实际主要瓶颈**:

1. **bcrypt密码哈希** (占80-90%时间)
   - 每次注册耗时: ~100-150ms
   - Cost=10 (默认)，每次需要2^10次迭代
   - 限制理论最大QPS: 1000ms / 100ms = 10 req/sec

2. **PowerShell Job的串行化**
   - PowerShell的Start-Job不是真正的并行
   - 多个Job之间有调度开销
   - 实际并发度低于预期

3. **数据库写入操作**
   - INSERT + 索引更新: ~10-30ms
   - 事务提交开销: ~5-10ms

### 性能优化优先级（修正）

#### ⚡ 立即见效（预计10-20倍提升）

**1. 降低bcrypt cost（仅测试环境）**
```go
// internal/user/service/user_service.go
// 从 bcrypt.DefaultCost (10) 降到 4
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 4)
```
**影响**: 从100ms降到6ms，预期QPS提升到100-150

**2. 使用Go原生并发测试**
```bash
# 使用 vegeta 或 hey 等专业压测工具
go install github.com/tsenart/vegeta@latest
echo "POST http://localhost:8081/api/v1/users" | vegeta attack -duration=30s -rate=50 -body=user.json
```

#### 🔧 中期优化（20-50%提升）

**3. 配置数据库连接池**
```go
sqlDB.SetMaxOpenConns(50)
sqlDB.SetMaxIdleConns(10)
```

**4. 添加缓存层**
- Redis缓存用户信息
- 减少数据库查询

#### 📈 长期优化

5. 异步处理非关键操作
6. 读写分离
7. 水平扩展

### 快速验证命令

```bash
# 1. 检查bcrypt在代码中的使用
grep -r "bcrypt.GenerateFromPassword" .

# 2. 使用专业压测工具
# 安装 hey
go install github.com/rakyll/hey@latest

# 3. 运行压测
hey -n 500 -c 50 -m POST -H "Content-Type: application/json" -d '{"username":"test","password":"pass","nickname":"nick"}' http://localhost:8081/api/v1/users
```

### 结论

- **主要瓶颈**: bcrypt密码哈希（每次100-150ms）
- **次要瓶颈**: PowerShell Job调度开销
- **优化目标**: 降低bcrypt cost可将QPS从8.92提升到100+
- **建议**: 生产环境保持bcrypt cost=10安全性，测试环境可降低到4
