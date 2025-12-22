# ChatIM Web

这是一个**无需构建**的单页前端（静态 HTML/CSS/JS），直接对接 [ChatIM/API_REFERENCE.md](../API_REFERENCE.md) 中的接口。

## 使用方式

1. 启动 ChatIM API Gateway（确保 HTTP 端口可访问，并且同源静态路由已启用）
2. 浏览器打开（按你的启动方式二选一）：
   - docker-compose：`http://localhost:8081/`（compose 里是 `8081:8080`）
   - 本地直接运行网关：通常是 `http://localhost:8080/`（取决于你的 `cfg.Server.APIPort`）

> 网关代码在 [cmd/api/main.go](../cmd/api/main.go) 中已经配置：
> - `GET /` -> `./web/index.html`
> - `GET /web/*` -> `./web` 静态资源

## 功能覆盖

- 登录/注册、`/users/me`
- 会话列表 + 置顶/取消/删除
- 私聊/群聊发送
- `/messages` 按会话拉取、未读数、未读拉取、全部未读
- 好友请求/好友列表
- 群组创建/列表/详情/成员/退出群、加群申请/处理/查询
- 搜索用户/群组
- 上传签名
- WebSocket 实时推送（`/ws?token=...`）

## 注意

- Token 会保存在 `localStorage`，你也可以手动粘贴。
- 默认 API Base 为同源的 `/api/v1`；若前后端不同域，可在页面中修改为 `http://localhost:8080/api/v1`。
