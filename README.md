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

## 功能

- **登录**：OAuth2 授权码流程，回调后换 Token，Token 存于 sessionStorage，过期前可用 Refresh Token 刷新。
- **故事画布**：每段带标签（时间/视角/新角色/风格），续写为 SecondMe 流式输出。
- **上文悬念**：从最新一段自动提取一句供模型承接。
- **续写控制**：时间（顺承/跳跃）、视角（保持/切换）、新角色、风格（科幻/温情/悬疑/哲思等）。
