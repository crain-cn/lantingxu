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

// loadSecondMeUserInfo 调 /api/secondme/user/info，返回 data.userId、data.name（不写库）。
func loadSecondMeUserInfo(accessToken string) (userID, name string, err error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/secondme/user/info", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out struct {
		Code int `json:"code"`
		Data struct {
			UserID string `json:"userId"`
			Name   string `json:"name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &out)
	if resp.StatusCode == http.StatusForbidden {
		return "", "", errors.New("secondme user/info 403：请为应用授权 scope user.info")
	}
	if out.Code != 0 {
		return "", "", errors.New("invalid secondme token")
	}
	return strings.TrimSpace(out.Data.UserID), strings.TrimSpace(out.Data.Name), nil
}

// resolveSecondMeUser 用 SecondMe 换发的应用用户 token 调 /api/secondme/user/info，
// 以 data.userId 为稳定身份（见 SecondMe OpenClaw 集成文档），并在本地 find-or-create 用户。
func resolveSecondMeUser(accessToken string) (userID int64, username string, err error) {
	smUID, smName, err := loadSecondMeUserInfo(accessToken)
	if err != nil {
		return 0, "", err
	}
	// 官方约定：业务身份用 data.userId，勿用顶层 id / data.id
	if smUID != "" {
		username = "smu_" + smUID
	} else {
		name := smName
		if name == "" {
			name = "secondme_user"
		}
		username = "secondme_" + strings.ReplaceAll(name, " ", "_")
	}
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

// ResolveTokenToUserID 从 Bearer token 解析出本地 userID（JWT 或 SecondMe token）。无法解析或为 Agent Key 时返回 (0, false)。
func ResolveTokenToUserID(token string) (userID int64, ok bool) {
	if token == "" {
		return 0, false
	}
	uid, _, _, err := model.ParseJWT(token)
	if err == nil {
		return uid, true
	}
	uid, _, err = resolveSecondMeUser(token)
	if err == nil {
		return uid, true
	}
	return 0, false
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
