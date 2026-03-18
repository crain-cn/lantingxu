package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"lantingxu/internal/model"
)

func HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "缺少 username 或 password"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	hash, err := model.HashPassword(body.Password)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	res, err := db.Exec("INSERT INTO users (username, password_hash, email, role) VALUES (?, ?, ?, 'user')",
		body.Username, hash, body.Email)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			WriteJSON(w, 400, map[string]any{"code": 400, "message": "用户名已存在"})
			return
		}
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	token, _ := model.IssueJWT(id, body.Username, "user")
	WriteJSON(w, 200, map[string]any{"code": 0, "userId": id, "username": body.Username, "token": token})
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "缺少 username 或 password"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	id, role, err := model.AuthUser(db, body.Username, body.Password)
	if err != nil {
		WriteJSON(w, 401, map[string]any{"code": 401, "message": err.Error()})
		return
	}
	token, _ := model.IssueJWT(id, body.Username, role)
	WriteJSON(w, 200, map[string]any{"code": 0, "userId": id, "username": body.Username, "token": token})
}

// HandleJWTToken 使用 appId + appSecret 换取 JWT，供 OpenAPI 等外部调用方使用。
func HandleJWTToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		AppId     string `json:"appId"`
		AppSecret string `json:"appSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	if body.AppId == "" || body.AppSecret == "" {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "缺少 appId 或 appSecret"})
		return
	}
	if !model.ValidateAppCredentials(body.AppId, body.AppSecret) {
		WriteJSON(w, 401, map[string]any{"code": 401, "message": "appId 或 appSecret 错误"})
		return
	}
	token, err := model.IssueJWTForApp(body.AppId)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	WriteJSON(w, 200, map[string]any{
		"code":         0,
		"accessToken":  token,
		"tokenType":    "Bearer",
		"expiresIn":    7 * 24 * 3600,
	})
}

// HandleCreateApp 创建新 API 应用，自动生成 appId、appSecret，无需认证（供 MCP create_app 等调用）。
func HandleCreateApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	appId, appSecret, err := model.CreateAPIApp(body.Name)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "appId": appId, "appSecret": appSecret})
}
