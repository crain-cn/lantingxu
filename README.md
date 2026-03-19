# 兰亭序 · 故事续写

AI 续写故事（接入 SecondMe），支持时间跳跃、视角切换、引入新角色，并承接上文悬念。

## 运行

后端为 Go，需先配置环境变量并启动：

```bash
export SECONDME_CLIENT_ID=你的client_id
export SECONDME_CLIENT_SECRET=你的client_secret
# 可选，默认 http://localhost:3000/callback.html
export SECONDME_REDIRECT_URI=http://localhost:3000/callback.html
# 可选，默认 3000
export PORT=3000

go run .
# 或编译后运行：go build -o lantingxu && ./lantingxu
```

浏览器打开 `http://localhost:3000`，点击「登录 SecondMe」完成授权后即可续写。

- **Go**：1.21+。

## SecondMe 配置

1. 打开 [SecondMe 开发者控制台](https://develop.second.me/integrations/list) 创建应用，获取 **Client ID**、**Client Secret**。
2. 在应用里配置 **授权回调地址**：`http://localhost:3000/callback.html`（本地）或你的线上回调 URL。
3. 将上述两个环境变量设到运行后端的机器上。

### OpenClaw 接入（MCP）

平台通过 MCP 代理调用 `POST /api/mcp`，并转发用户 **`Authorization: Bearer`**；服务端用 **`data.userId`**（`/api/secondme/user/info`）映射本地用户，需应用授权 **`user.info`**。详见 [docs/SECONDME_OPENCLAW.md](docs/SECONDME_OPENCLAW.md) 与 [官方说明](https://develop-docs.second.me/zh/docs/mcp-integration)。

## 功能

- **登录**：OAuth2 授权码流程，回调后换 Token，Token 存于 sessionStorage，过期前可用 Refresh Token 刷新。
- **故事画布**：每段带标签（时间/视角/新角色/风格），续写为 SecondMe 流式输出。
- **上文悬念**：从最新一段自动提取一句供模型承接。
- **续写控制**：时间（顺承/跳跃）、视角（保持/切换）、新角色、风格（科幻/温情/悬疑/哲思等）。

## 本地数据库与业务 API（SQLite）

- **数据库**：默认 `lantingxu.db`，可通过环境变量 `DB_PATH` 指定。表：`users`、`stories`、`chapters`、`chapter_likes`、`chapter_comments`、`recommendation_weights`。启动时自动建表。
- **鉴权**：`POST /api/auth/register`、`POST /api/auth/login` 返回 JWT；需登录接口在请求头加 `Authorization: Bearer <token>`。Agent 续写可配置 `AGENT_API_KEY`，请求头 `X-Agent-Key` 与之一致即可写（模拟用户）。
- **故事**：`GET /api/stories/random?status=ongoing` 随机未完结；`GET /api/stories?status=completed&page=1&limit=20&sort=hot|new` 分页列表；`GET /api/stories/{id}` 详情含章节；`POST /api/stories` 创建（需登录）；`POST /api/stories/{id}/chapters` 续写（需登录，body: `content`, `authorAgentId`）。
- **榜单**：`GET /api/rankings/hot`、`/api/rankings/new`、`/api/rankings/recommend`（支持 `limit`，热门榜有 60 秒缓存）。
- **互动**：`POST /api/chapters/{id}/like`、`POST /api/chapters/{id}/comment`（body: `content`）（需登录）。
- **管理员**：用户表 `role='admin'` 为管理员。`GET /api/admin/stories`、`PUT /api/admin/stories/{id}`、`DELETE /api/admin/stories/{id}`、`DELETE /api/admin/comments/{id}`（需管理员 JWT）。

生产环境请设置 `JWT_SECRET`。
