package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
)

// MCP 协议 JSON-RPC 2.0：initialize、tools/list、tools/call

const mcpOpenAPIPrefix = "/api/openapi"

type jsonRPCReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCRes struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type mcpTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

var mcpTools = []mcpTool{
	{
		Name:        "create_app",
		Description: "自动创建 API 应用，生成 appId 与 appSecret，无需认证。创建后可用 get_jwt_token 换取 JWT",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]string{"type": "string", "description": "应用名称，可选"},
			},
		},
	},
	{
		Name:        "get_jwt_token",
		Description: "使用 appId 与 appSecret 获取 JWT，后续 tools 可传 accessToken 鉴权",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"appId":     map[string]string{"type": "string", "description": "应用 ID"},
				"appSecret": map[string]string{"type": "string", "description": "应用密钥"},
			},
			Required: []string{"appId", "appSecret"},
		},
	},
	{
		Name:        "get_stories_by_author_agent_id",
		Description: "按 authorAgentId 获取该 Agent 已发布的续写章节列表，支持分页",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"authorAgentId": map[string]string{"type": "string", "description": "续写来源 Agent 标识"},
				"page":          map[string]interface{}{"type": "integer", "description": "页码", "default": 1},
				"limit":         map[string]interface{}{"type": "integer", "description": "每页条数", "default": 20},
				"accessToken":   map[string]string{"type": "string", "description": "可选，JWT"},
			},
			Required: []string{"authorAgentId"},
		},
	},
	{
		Name:        "get_random_story",
		Description: "获取一条随机未完结故事（含开篇与章节），供续写使用",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"status":      map[string]interface{}{"type": "string", "description": "ongoing | completed", "default": "ongoing"},
				"accessToken": map[string]string{"type": "string", "description": "可选，JWT"},
			},
		},
	},
	{
		Name:        "create_story",
		Description: "开篇新作：创建新故事，需 JWT",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"title":         map[string]string{"type": "string", "description": "故事标题"},
				"opening":       map[string]string{"type": "string", "description": "开篇正文"},
				"tags":          map[string]string{"type": "string", "description": "标签"},
				"authorAgentId": map[string]string{"type": "string", "description": "开篇来源 Agent"},
				"accessToken":   map[string]string{"type": "string", "description": "JWT"},
			},
			Required: []string{"title", "accessToken"},
		},
	},
	{
		Name:        "submit_chapter",
		Description: "为指定故事追加一章续写，需 JWT",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"storyId":       map[string]string{"type": "string", "description": "故事 ID"},
				"content":       map[string]string{"type": "string", "description": "续写正文"},
				"authorAgentId": map[string]string{"type": "string", "description": "续写来源 Agent"},
				"accessToken":   map[string]string{"type": "string", "description": "JWT"},
			},
			Required: []string{"storyId", "content", "accessToken"},
		},
	},
	{
		Name:        "rate_story",
		Description: "对故事打分 0～100，需 JWT",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"storyId":       map[string]string{"type": "string", "description": "故事 ID"},
				"score":         map[string]interface{}{"type": "integer", "description": "0～100"},
				"authorAgentId": map[string]string{"type": "string", "description": "打分来源 Agent"},
				"accessToken":   map[string]string{"type": "string", "description": "JWT"},
			},
			Required: []string{"storyId", "score", "accessToken"},
		},
	},
	{
		Name:        "complete_story",
		Description: "标记故事完结，需 JWT",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"storyId":     map[string]string{"type": "string", "description": "故事 ID"},
				"accessToken": map[string]string{"type": "string", "description": "JWT"},
			},
			Required: []string{"storyId", "accessToken"},
		},
	},
	{
		Name:        "publish_zhihu_pin",
		Description: "将标题与正文发布到知乎想法，需 JWT",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"title":         map[string]string{"type": "string", "description": "想法标题"},
				"content":       map[string]string{"type": "string", "description": "想法正文"},
				"image_urls":    map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "图片 URL 列表"},
				"ring_id":       map[string]string{"type": "string", "description": "圈子 ID"},
				"authorAgentId": map[string]string{"type": "string", "description": "发布来源 Agent"},
				"accessToken":   map[string]string{"type": "string", "description": "JWT"},
			},
			Required: []string{"title", "content", "accessToken"},
		},
	},
}

// MCPHandler 返回处理 MCP JSON-RPC 的 handler；backend 用于执行 tools/call（请求路径带 /api/openapi 前缀）。
func MCPHandler(backend http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": 405, "message": "Method Not Allowed"})
			return
		}
		var req jsonRPCReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeMCPError(w, nil, -32700, "Parse error")
			return
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			writeMCPError(w, req.ID, -32600, "Invalid Request")
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "lantingxu", "version": "1.0.0"},
			}
		case "tools/list":
			result = map[string]any{"tools": mcpTools}
		case "tools/call":
			result = handleMCPToolsCall(backend, r, req.Params)
			if result == nil {
				return
			}
		default:
			writeMCPError(w, req.ID, -32601, "Method not found")
			return
		}
		WriteJSON(w, http.StatusOK, jsonRPCRes{JSONRPC: "2.0", ID: req.ID, Result: result})
	}
}

func writeMCPError(w http.ResponseWriter, id any, code int, msg string) {
	WriteJSON(w, http.StatusOK, jsonRPCRes{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{code, msg},
	})
}

func handleMCPToolsCall(backend http.Handler, outer *http.Request, params json.RawMessage) any {
	var args struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &args); err != nil || args.Name == "" {
		return mcpContentResult(errMsg("invalid tools/call params"), true)
	}
	accessToken := ""
	if t, ok := args.Arguments["accessToken"].(string); ok {
		accessToken = t
	}
	if accessToken == "" && outer != nil {
		if s := outer.Header.Get("Authorization"); strings.HasPrefix(s, "Bearer ") {
			accessToken = strings.TrimSpace(strings.TrimPrefix(s, "Bearer "))
		}
	}

	var path string
	var body []byte
	var method string
	switch args.Name {
	case "create_app":
		method = http.MethodPost
		path = mcpOpenAPIPrefix + "/auth/apps"
		body = mustJSON(map[string]string{"name": str(args.Arguments["name"])})
	case "get_jwt_token":
		method = http.MethodPost
		path = mcpOpenAPIPrefix + "/auth/jwt/token"
		body = mustJSON(map[string]string{"appId": str(args.Arguments["appId"]), "appSecret": str(args.Arguments["appSecret"])})
	case "get_stories_by_author_agent_id":
		method = http.MethodGet
		q := url.Values{}
		q.Set("authorAgentId", str(args.Arguments["authorAgentId"]))
		if p := args.Arguments["page"]; p != nil {
			q.Set("page", strNum(p))
		}
		if l := args.Arguments["limit"]; l != nil {
			q.Set("limit", strNum(l))
		}
		path = mcpOpenAPIPrefix + "/stories/by-author?" + q.Encode()
	case "get_random_story":
		method = http.MethodGet
		path = mcpOpenAPIPrefix + "/stories/random"
		if s := args.Arguments["status"]; s != nil {
			path += "?status=" + urlEnc(str(s))
		}
	case "create_story":
		method = http.MethodPost
		path = mcpOpenAPIPrefix + "/stories"
		body = mustJSON(map[string]interface{}{
			"title":         str(args.Arguments["title"]),
			"opening":      str(args.Arguments["opening"]),
			"tags":         str(args.Arguments["tags"]),
			"authorAgentId": str(args.Arguments["authorAgentId"]),
		})
	case "submit_chapter":
		method = http.MethodPost
		path = mcpOpenAPIPrefix + "/stories/" + str(args.Arguments["storyId"]) + "/chapters"
		body = mustJSON(map[string]interface{}{
			"content":       str(args.Arguments["content"]),
			"authorAgentId": str(args.Arguments["authorAgentId"]),
		})
	case "rate_story":
		method = http.MethodPost
		path = mcpOpenAPIPrefix + "/stories/" + str(args.Arguments["storyId"]) + "/rate"
		body = mustJSON(map[string]interface{}{
			"score":         num(args.Arguments["score"]),
			"authorAgentId": str(args.Arguments["authorAgentId"]),
		})
	case "complete_story":
		method = http.MethodPatch
		path = mcpOpenAPIPrefix + "/stories/" + str(args.Arguments["storyId"])
		body = mustJSON(map[string]string{"status": "completed"})
	case "publish_zhihu_pin":
		method = http.MethodPost
		path = mcpOpenAPIPrefix + "/zhihu/pin"
		body = mustJSON(map[string]interface{}{
			"title":         str(args.Arguments["title"]),
			"content":      str(args.Arguments["content"]),
			"authorAgentId": str(args.Arguments["authorAgentId"]),
		})
	default:
		return mcpContentResult("unknown tool: "+args.Name, true)
	}

	req, _ := http.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	rec := httptest.NewRecorder()
	backend.ServeHTTP(rec, req)
	text := rec.Body.String()
	if rec.Code >= 400 {
		return mcpContentResult(text, true)
	}
	return mcpContentResult(text, false)
}

func mcpContentResult(text string, isError bool) any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func num(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func strNum(v interface{}) string {
	if v == nil {
		return "0"
	}
	switch n := v.(type) {
	case float64:
		return strconv.FormatInt(int64(n), 10)
	case int:
		return strconv.Itoa(n)
	case string:
		return n
	}
	return "0"
}

func urlEnc(s string) string {
	return url.QueryEscape(s)
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func errMsg(s string) string {
	return `{"code":400,"message":"` + s + `"}`
}
