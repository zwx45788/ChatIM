# 迁移系统快速参考

## 问题
❌ 如果我想在 `init.sql` 中增加数据库字段，是不是无法生效？

## 答案
✅ **没错，你已经解决了！** 新的迁移系统完全解决了这个问题。

---

## 快速开始

### 添加新字段只需 3 步：

#### 1️⃣ 创建迁移文件

```bash
# 文件：migrations/003_add_user_avatar.sql

ALTER TABLE `users` 
ADD COLUMN IF NOT EXISTS `avatar_url` VARCHAR(255) NULL,
ADD COLUMN IF NOT EXISTS `avatar_updated_at` TIMESTAMP NULL;

INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('003_add_user_avatar');
```

#### 2️⃣ 启动服务

```bash
docker-compose restart user-service
```

#### 3️⃣ 查看日志（自动执行）

```bash
docker logs -f chatim_user_service | grep migration

# 输出：
# → Running migration: 003_add_user_avatar
# ✓ Migration 003_add_user_avatar executed successfully
```

✅ **完成！** 数据库字段已自动添加。

---

## 迁移文件模板

```sql
-- migrations/NNN_description.sql
-- 说明这个迁移的目的（可选）

-- 迁移内容（支持多条SQL）
ALTER TABLE table_name ...;
CREATE TABLE IF NOT EXISTS ...;
DROP INDEX IF EXISTS ...;

-- 必须：记录迁移已执行
INSERT IGNORE INTO `schema_migrations` (`version`) VALUES ('NNN_description');
```

---

## 常见操作

| 操作 | SQL |
|------|-----|
| 添加列 | `ALTER TABLE t ADD COLUMN IF NOT EXISTS c TYPE;` |
| 删除列 | `ALTER TABLE t DROP COLUMN IF EXISTS c;` |
| 修改列类型 | `ALTER TABLE t MODIFY COLUMN c NEW_TYPE;` |
| 添加索引 | `ALTER TABLE t ADD INDEX IF NOT EXISTS idx_name (col);` |
| 删除索引 | `ALTER TABLE t DROP INDEX IF EXISTS idx_name;` |
| 创建表 | `CREATE TABLE IF NOT EXISTS t (...);` |
| 删除表 | `DROP TABLE IF EXISTS t;` |

---

## 文件名规则

```
✅ 正确
migrations/001_init_schema.sql
migrations/002_add_user_status.sql
migrations/003_create_analytics.sql

❌ 错误
migrations/1_init.sql              (版本号应该是 3 位)
migrations/add_user.sql            (缺少版本号)
migrations/002_add_user_status     (缺少 .sql 后缀)
migrations/002_002_add_user.sql    (重复的版本号)
```

---

## 验证迁移执行

```bash
# 进入 MySQL
docker exec -it chatim_mysql mysql -u chatim_user -p chatim

# 查看迁移历史
mysql> SELECT * FROM schema_migrations;

# 验证新列已添加
mysql> DESCRIBE users;
```

---

## 关键要点

1. ⭐ **所有迁移自动执行** - 无需手动操作
2. ⭐ **幂等性设计** - 可以安全地重复执行
3. ⭐ **版本追踪** - 完整的迁移历史记录
4. ⭐ **按顺序执行** - 按文件名（版本号）排序

---

## 工作流程

```
你创建 003_xxx.sql
      ↓
docker-compose restart user-service
      ↓
User Service 启动
      ↓
migrations.RunMigrations(db) 自动执行
      ↓
✅ 完成
```

---

## 文档链接

- **完整指南**: `DATABASE_MIGRATION.md` (详细的最佳实践、故障排除等)
- **快速指南**: `MIGRATION_GUIDE.md` (bash 脚本示例)
- **本文档**: `MIGRATION_SUMMARY.md` (快速参考)

---

现在你已经完全解决了初始化脚本无法增量更新的问题！🎉
