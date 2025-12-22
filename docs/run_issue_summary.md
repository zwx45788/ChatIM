# ChatIM 接口故障与修复记录（2025-12-20）

## 背景
PowerShell 自测脚本 `test_all_apis.ps1` 初始通过率 68.75%（11/16）。主要失败点：
- 群组相关接口 500
- 好友请求 404
- 未读消息接口 500

## 根因与修复
1) **MySQL 保留字导致群组 SQL 语法错误**
   - 问题：表名 `groups` 未转义，MySQL 报 1064 语法错误。
   - 现象：/groups 创建/查询接口 500。
   - 修复：在 `internal/group_service/handler/group.go` 中所有 SQL 使用反引号包裹表名 `groups`，并重建 group-service 镜像。

2) **好友请求路由不一致**
   - 问题：API 路由是 `/friends/requests`，脚本用的是单数 `/friends/request`。
   - 现象：发送好友请求 404。
   - 修复：脚本改为 `/friends/requests`，测试通过。

3) **消息未读接口缺列**
   - 问题：`messages` 表缺少 `is_read`、`read_at` 字段及索引。
   - 现象：/messages/unread 与 /messages/unread/pull 500，日志报 Unknown column `is_read`。
   - 修复：对 MySQL 执行：
     - `ALTER TABLE messages ADD COLUMN is_read BOOLEAN DEFAULT FALSE AFTER content;`
     - `ALTER TABLE messages ADD COLUMN read_at TIMESTAMP NULL DEFAULT NULL AFTER is_read;`
     - `CREATE INDEX idx_to_user_read ON messages(to_user_id, is_read);`
     - 重启 message-service。

4) **群成员添加 500（缺少 group_id 传递）**
   - 问题：API Gateway 在 `AddGroupMember` 未从路径设置 `group_id` 到 gRPC 请求。
   - 现象：/groups/{id}/members 返回 500。
   - 修复：在 `internal/api_gateway/handler/handler.go` 读取 path param `group_id` 赋给请求后再调用 gRPC；重建 api-gateway。

5) **群聊消息 404**
   - 问题：脚本调用 `/groups/{id}/messages`，实际规范为 `/groups/messages`，且需要 body 内携带 `group_id`。
   - 现象：发送/拉取群聊消息 404。
   - 修复：脚本改为 POST `/groups/messages`，body 含 `group_id`；重测通过。

## 当前结果
- 自测脚本通过率 100%（20/20）。
- 关键数据：用户注册/登录/好友/群组/消息（私聊、群聊、未读）全部可用。

## 建议
- 在 `init.sql` 中补充 `messages` 表新增字段与索引，避免新环境缺列。
- 保持 `groups` 表名转义一致性，后续查询/迁移注意反引号。
- 路由改动时同步更新测试脚本，避免 404。
