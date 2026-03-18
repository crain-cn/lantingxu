package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 10

var (
	jwtSecret     = []byte(os.Getenv("JWT_SECRET"))
	agentAPIKey   = os.Getenv("AGENT_API_KEY")
	defaultSecret = []byte("lantingxu-default-secret-change-in-prod")
)

func init() {
	if len(jwtSecret) == 0 {
		jwtSecret = defaultSecret
	}
}

type claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func hashPassword(pwd string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcryptCost)
	return string(b), err
}

func checkPassword(hash, pwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) == nil
}

func issueJWT(userID int64, username, role string) (string, error) {
	c := claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(jwtSecret)
}

func parseJWT(tokenString string) (*claims, error) {
	t, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if c, ok := t.Claims.(*claims); ok && t.Valid {
		return c, nil
	}
	return nil, errors.New("invalid token")
}

type contextKey string

const userContextKey contextKey = "user"

func withUser(r *http.Request, uid int64, username, role string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, &claims{UserID: uid, Username: username, Role: role}))
}

func userFromRequest(r *http.Request) (userID int64, username, role string, ok bool) {
	c, _ := r.Context().Value(userContextKey).(*claims)
	if c == nil {
		return 0, "", "", false
	}
	return c.UserID, c.Username, c.Role, true
}

// requireAuth 需要登录的接口使用；从 Authorization: Bearer <token> 或 X-Agent-Key 解析身份。
// Agent 使用 X-Agent-Key 时，若配置了 AGENT_API_KEY 则视为“模拟用户”，可写；否则仅放行读接口（写接口应再校验）。
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Bearer JWT
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			c, err := parseJWT(token)
			if err == nil {
				r = withUser(r, c.UserID, c.Username, c.Role)
				next(w, r)
				return
			}
			// 非 JWT 时尝试当作 SecondMe OAuth token 校验并映射到本地用户
			if uid, uname, err2 := resolveSecondMeUser(token); err2 == nil {
				r = withUser(r, uid, uname, "user")
				next(w, r)
				return
			}
		}
		// 2) Agent API Key（可选：与本地用户绑定或使用系统 Agent 用户）
		key := r.Header.Get("X-Agent-Key")
		if key != "" && agentAPIKey != "" && key == agentAPIKey {
			// 使用 0 表示 Agent，author_agent_id 由请求体传入
			r = withUser(r, 0, "agent", "user")
			next(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": 401, "message": "需要登录或有效 API Key"})
	}
}

// requireAdmin 需在 requireAuth 之后使用
func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, role, ok := userFromRequest(r)
		if !ok || role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]any{"code": 403, "message": "需要管理员权限"})
			return
		}
		next(w, r)
	}
}

func optionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
			if c, err := parseJWT(token); err == nil {
				r = withUser(r, c.UserID, c.Username, c.Role)
			}
		}
		next(w, r)
	}
}

// 从 DB 校验用户名密码并返回用户信息
func authUser(db *sql.DB, username, password string) (id int64, role string, err error) {
	var hash string
	err = db.QueryRow("SELECT id, password_hash, role FROM users WHERE username = ?", username).Scan(&id, &hash, &role)
	if err != nil {
		if err == sql.ErrNoRows {
			err = errors.New("用户名或密码错误")
		}
		return 0, "", err
	}
	if !checkPassword(hash, password) {
		return 0, "", errors.New("用户名或密码错误")
	}
	return id, role, nil
}
