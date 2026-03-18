package controller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"lantingxu/internal/model"
)

const baseURL = "https://api.mindverse.com/gate/lab"

// resolveSecondMeUser 用 SecondMe access token 调远程 API 校验，并在本地 find-or-create 用户。
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
		Code int `json:"code"`
		Data struct {
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
	username = "secondme_" + strings.ReplaceAll(strings.TrimSpace(name), " ", "_")
	db, err := model.GetDB()
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
	hash, err := model.HashPassword("secondme-nologin")
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

// RequireAuth 需要登录的接口使用；支持 Bearer JWT、SecondMe token、X-Agent-Key。
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			uid, uname, role, err := model.ParseJWT(token)
			if err == nil {
				r = model.WithUser(r, uid, uname, role)
				next(w, r)
				return
			}
			if uid, uname, err2 := resolveSecondMeUser(token); err2 == nil {
				r = model.WithUser(r, uid, uname, "user")
				next(w, r)
				return
			}
		}
		key := r.Header.Get("X-Agent-Key")
		if key != "" && model.CheckAgentKey(key) {
			r = model.WithUser(r, 0, "agent", "user")
			next(w, r)
			return
		}
		WriteJSON(w, http.StatusUnauthorized, map[string]any{"code": 401, "message": "需要登录或有效 API Key"})
	}
}

// RequireAdmin 需在 RequireAuth 之后使用。
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, role, ok := model.UserFromRequest(r)
		if !ok || role != "admin" {
			WriteJSON(w, http.StatusForbidden, map[string]any{"code": 403, "message": "需要管理员权限"})
			return
		}
		next(w, r)
	}
}
