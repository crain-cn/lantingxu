package main

import (
	"bytes"
	"encoding/json"
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
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/oauth/token", handleOAuthToken)
	http.HandleFunc("/api/oauth/refresh", handleOAuthRefresh)
	http.HandleFunc("/api/chat/stream", handleChatStream)
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
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	code := body.Code
	uri := body.RedirectURI
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
