# SecondMe OpenClaw 集成（MCP）

对照官方文档：[通过 OpenClaw 使用应用](https://develop-docs.second.me/zh/docs/mcp-integration)。

## 本服务已具备

- **HTTP MCP**：`POST /api/mcp`，JSON-RPC 2.0，支持 `initialize`、`tools/list`、`tools/call`。
- **平台转发鉴权**：SecondMe 代理会在请求头附带 `Authorization: Bearer lba_at_...`；本服务已用该 token 调用 `GET https://api.mindverse.com/gate/lab/api/secondme/user/info`，以 **`data.userId`** 在本地做用户映射（`smu_<userId>`），与文档一致。
- **业务鉴权**：需登录的 OpenAPI（续写、打分等）在收到上述 Bearer 时，与兰亭 JWT 等价解析为本地用户。
- **调试工具**：MCP tool `secondme_user_info` 直接返回用户信息 JSON（需应用 **`user.info`** scope）。

## 控制台配置要点

1. 在 [SecondMe Developer Console](https://develop.second.me/) 创建 **OAuth 应用**，拿到 **OAuth App ID**。
2. 创建 **integration**，`MCP Endpoint` 填你的公网地址，例如：`https://你的域名/api/mcp`。
3. **`mcp.authMode`**：`bearer_token`。
4. **`oauth.requiredScopes`**：若要用用户信息或本地账号映射，至少包含 **`user.info`**（否则 `/api/secondme/user/info` 会 403）。
5. **`mcp.toolAllow`** 与 **`actions[].toolName`** 必须与 `tools/list` 里的 **`tools[].name`** 完全一致。

### 当前 tool 名称列表（复制到 toolAllow）

```
secondme_user_info
create_app
get_jwt_token
get_stories_by_author_agent_id
get_random_story
create_story
submit_chapter
rate_story
complete_story
publish_zhihu_pin
```

### integration JSON 模板（按需改 URL、skill key、appId）

```json
{
  "skill": {
    "key": "lantingxu",
    "displayName": "兰亭序续写",
    "description": "故事续写、随机续写、打分、知乎想法发布等。",
    "keywords": ["story", "续写", "兰亭序", "知乎"]
  },
  "prompts": {
    "activationShort": "兰亭序续写",
    "activationLong": "续写故事、随机续写与打分、完结故事、发布知乎想法；OpenClaw 下使用平台转发的用户 Bearer。",
    "systemSummary": "通过 MCP 调用兰亭序续写能力；鉴权可为兰亭 JWT 或 SecondMe 转发的 Bearer；用户身份以 SecondMe data.userId 映射本地账号。"
  },
  "actions": [
    {
      "name": "当前 SecondMe 用户",
      "description": "查询当前授权用户的 SecondMe 信息（含 userId）。",
      "toolName": "secondme_user_info",
      "displayHint": "我是谁"
    },
    {
      "name": "随机故事",
      "description": "获取一条随机故事供续写或打分。",
      "toolName": "get_random_story",
      "displayHint": "随机一篇"
    },
    {
      "name": "提交续写",
      "description": "为指定故事追加一章。",
      "toolName": "submit_chapter",
      "displayHint": "续写"
    }
  ],
  "mcp": {
    "endpoint": "https://你的域名/api/mcp",
    "timeoutMs": 30000,
    "authMode": "bearer_token",
    "toolAllow": [
      "secondme_user_info",
      "create_app",
      "get_jwt_token",
      "get_stories_by_author_agent_id",
      "get_random_story",
      "create_story",
      "submit_chapter",
      "rate_story",
      "complete_story",
      "publish_zhihu_pin"
    ],
    "headersTemplate": {}
  },
  "oauth": {
    "appId": "你的_OAuth_App_ID",
    "requiredScopes": ["user.info"]
  },
  "environments": {
    "prod": {
      "enabled": true,
      "endpointOverride": "",
      "secrets": {}
    }
  }
}
```

## 联调检查（与文档一致）

1. 直连你的服务：`tools/list` 正常。
2. `actions[].toolName` / `mcp.toolAllow` 与工具名一致。
3. 代理调用时你的服务收到 `Authorization: Bearer lba_at_*`。
4. `secondme_user_info` 或用户映射路径能拿到 **`data.userId`**。
5. 若 403，先查用户对该应用的历史授权是否包含 **`user.info`**。

## 说明

- OpenClaw 实际请求的是平台代理：`POST .../rest/third-party-agent/v1/mcp/{integrationKey}/rpc`，由平台换发 token 再访问你的 MCP；你**不要**依赖主站 `sm-*` token 识别用户。
- 本地直连 MCP（如 Cursor）仍可用 `get_jwt_token` + `accessToken`；与 SecondMe 转发 Bearer 两种方式可并存。
