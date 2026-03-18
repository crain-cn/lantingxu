package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const baseURL = "https://api.mindverse.com/gate/lab"

var (
	clientID     = os.Getenv("SECONDME_CLIENT_ID")
	clientSecret = os.Getenv("SECONDME_CLIENT_SECRET")
	redirectURI  = os.Getenv("SECONDME_REDIRECT_URI")
	port         = os.Getenv("PORT")
)

func init() {
	if port == "" {
		port = "3000"
	}
	if redirectURI == "" {
		redirectURI = "http://localhost:" + port + "/callback.html"
	}
}

func main() {
	if _, err := initDB(); err != nil {
		log.Fatal("initDB: ", err)
	}

	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Agent-Key")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h(w, r)
		}
	}

	http.HandleFunc("/api/config", cors(handleConfig))
	http.HandleFunc("/api/oauth/token", cors(handleOAuthToken))
	http.HandleFunc("/api/oauth/refresh", cors(handleOAuthRefresh))
	http.HandleFunc("/api/oauth/me", cors(handleOAuthMe))
	http.HandleFunc("/api/chat/stream", cors(handleChatStream))

	http.HandleFunc("/api/auth/register", cors(handleRegister))
	http.HandleFunc("/api/auth/login", cors(handleLogin))

	http.HandleFunc("/api/stories", cors(handleStories))
	http.HandleFunc("/api/stories/", cors(handleStoriesSlash))

	http.HandleFunc("/api/rankings/hot", cors(handleRankingsHot))
	http.HandleFunc("/api/rankings/new", cors(handleRankingsNew))
	http.HandleFunc("/api/rankings/recommend", cors(handleRankingsRecommend))

	http.HandleFunc("/api/chapters/", cors(handleChapters))

	http.HandleFunc("/api/admin/stories", cors(requireAuth(requireAdmin(handleAdminStoriesList))))
	http.HandleFunc("/api/admin/stories/", cors(handleAdminStoriesSlash))
	http.HandleFunc("/api/admin/comments/", cors(requireAuth(requireAdmin(handleAdminCommentDelete))))

	http.HandleFunc("/", serveStatic)

	addr := ":" + port
	log.Printf("Server http://localhost%s", addr)
	if clientID == "" || clientSecret == "" {
		log.Print("未设置 SECONDME_CLIENT_ID / SECONDME_CLIENT_SECRET，OAuth 与续写将不可用")
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"clientId":   clientID,
		"redirectUri": redirectURI,
		"hasSecret":  clientSecret != "",
	})
}

func handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirect_uri"`
		RedirectUri string `json:"redirectUri"` // 与 callback 统一，二选一
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	code := body.Code
	uri := body.RedirectURI
	if uri == "" {
		uri = body.RedirectUri
	}
	if uri == "" {
		uri = redirectURI
	}
	if code == "" || clientID == "" || clientSecret == "" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 code 或未配置 client_id/client_secret"})
		return
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", uri)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/oauth/token/code", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		SubCode string `json:"subCode"`
		Data    struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int    `json:"expiresIn"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &out)
	if out.Code != 0 {
		writeJSON(w, 400, map[string]any{"code": out.Code, "message": out.Message, "subCode": out.SubCode})
		return
	}
	writeJSON(w, 200, map[string]any{
		"code":         0,
		"accessToken":  out.Data.AccessToken,
		"refreshToken": out.Data.RefreshToken,
		"expiresIn":    out.Data.ExpiresIn,
	})
}

func handleOAuthRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	rt := body.RefreshToken
	if rt == "" || clientID == "" || clientSecret == "" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 refresh_token 或未配置"})
		return
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", rt)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/oauth/token/refresh", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		SubCode string `json:"subCode"`
		Data    struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int    `json:"expiresIn"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &out)
	if out.Code != 0 {
		writeJSON(w, 400, map[string]any{"code": out.Code, "message": out.Message, "subCode": out.SubCode})
		return
	}
	writeJSON(w, 200, map[string]any{
		"accessToken":  out.Data.AccessToken,
		"refreshToken": out.Data.RefreshToken,
		"expiresIn":    out.Data.ExpiresIn,
	})
}

func handleOAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	token := ""
	if s := r.Header.Get("Authorization"); strings.HasPrefix(s, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(s, "Bearer "))
	}
	if token == "" {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "需要 Authorization: Bearer <token>"})
		return
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/secondme/user/info", nil)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Name   string `json:"name"`
			Bio    string `json:"bio"`
			Avatar string `json:"avatar"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &out)
	if out.Code != 0 {
		writeJSON(w, resp.StatusCode, map[string]any{"code": out.Code, "message": out.Message})
		return
	}
	writeJSON(w, 200, map[string]any{
		"code": 0,
		"name":  out.Data.Name,
		"bio":   out.Data.Bio,
		"avatar": out.Data.Avatar,
	})
}

// resolveSecondMeUser 用 SecondMe access token 调 /api/secondme/user/info 校验，并在本地 find-or-create 用户，返回 (userID, username, nil) 或错误。
func resolveSecondMeUser(accessToken string) (userID int64, username string, err error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/secondme/user/info", nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out struct {
		Code    int `json:"code"`
		Data   struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &out)
	if out.Code != 0 {
		return 0, "", errors.New("invalid secondme token")
	}
	name := out.Data.Name
	if name == "" {
		name = "secondme_user"
	}
	// 本地用户名，避免与普通注册冲突
	username = "secondme_" + strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
	db, err := getDB()
	if err != nil {
		return 0, "", err
	}
	var id int64
	err = db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&id)
	if err == nil {
		return id, username, nil
	}
	if err != sql.ErrNoRows {
		return 0, "", err
	}
	hash, err := hashPassword("secondme-nologin")
	if err != nil {
		return 0, "", err
	}
	res, err := db.Exec("INSERT INTO users (username, password_hash, email, role) VALUES (?, ?, '', 'user')", username, hash)
	if err != nil {
		return 0, "", err
	}
	id, _ = res.LastInsertId()
	return id, username, nil
}

func handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		AccessToken string `json:"accessToken"`
		Message     string `json:"message"`
		SessionID   string `json:"sessionId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	token := ""
	if s := r.Header.Get("Authorization"); strings.HasPrefix(s, "Bearer ") {
		token = strings.TrimPrefix(s, "Bearer ")
	}
	if token == "" {
		token = body.AccessToken
	}
	if token == "" {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "需要 Authorization: Bearer <token>"})
		return
	}
	if body.Message == "" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 message"})
		return
	}
	payload := map[string]string{"message": body.Message}
	if body.SessionID != "" {
		payload["sessionId"] = body.SessionID
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/secondme/chat/stream", bytes.NewReader(raw))
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		var j map[string]any
		_ = json.Unmarshal(data, &j)
		if j != nil {
			w.WriteHeader(resp.StatusCode)
			json.NewEncoder(w).Encode(j)
			return
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	io.Copy(w, resp.Body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// handleStories: GET /api/stories -> list, POST /api/stories -> create(需登录)
func handleStories(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/stories" {
		return
	}
	switch r.Method {
	case http.MethodGet:
		handleStoriesList(w, r)
	case http.MethodPost:
		requireAuth(handleStoryCreate)(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// handleStoriesSlash: /api/stories/random, /api/stories/{id}, /api/stories/{id}/chapters
func handleStoriesSlash(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/stories/")
	switch {
	case path == "random":
		handleStoriesRandom(w, r)
	case strings.HasSuffix(path, "/chapters"):
		if r.Method == http.MethodPost {
			requireAuth(handleStoryAddChapter)(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case path != "" && !strings.Contains(path, "/"):
		if r.Method == http.MethodGet {
			handleStoryDetail(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

// handleChapters: POST /api/chapters/{id}/like, POST /api/chapters/{id}/comment, GET /api/chapters/{id}/comments
func handleChapters(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/chapters/")
	switch {
	case strings.HasSuffix(path, "/like"):
		if r.Method == http.MethodPost {
			requireAuth(handleChapterLike)(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasSuffix(path, "/comment"):
		if r.Method == http.MethodPost {
			requireAuth(handleChapterComment)(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	case strings.HasSuffix(path, "/comments"):
		if r.Method == http.MethodGet {
			handleChapterCommentsList(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

// handleAdminStoriesSlash: PUT/DELETE /api/admin/stories/{id}
func handleAdminStoriesSlash(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/stories/")
	path = strings.Trim(path, "/")
	if path == "" || strings.Contains(path, "/") {
		http.NotFound(w, r)
		return
	}
	h := requireAuth(requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut, http.MethodPatch:
			handleAdminStoryUpdate(w, r)
		case http.MethodDelete:
			handleAdminStoryDelete(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	}))
	h(w, r)
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		path := "." + r.URL.Path
		if f, err := os.Open(path); err == nil {
			defer f.Close()
			if st, _ := f.Stat(); st != nil && !st.IsDir() {
				http.ServeContent(w, r, st.Name(), st.ModTime(), f)
				return
			}
		}
	}
	http.FileServer(http.Dir(".")).ServeHTTP(w, r)
}
