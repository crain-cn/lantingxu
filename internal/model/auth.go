package model

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
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

func HashPassword(pwd string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcryptCost)
	return string(b), err
}

func checkPassword(hash, pwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) == nil
}

func IssueJWT(userID int64, username, role string) (string, error) {
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

// ParseJWT 解析 token，返回 userID, username, role；失败返回 error。
func ParseJWT(tokenString string) (userID int64, username, role string, err error) {
	t, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return 0, "", "", err
	}
	c, ok := t.Claims.(*claims)
	if !ok || !t.Valid {
		return 0, "", "", errors.New("invalid token")
	}
	return c.UserID, c.Username, c.Role, nil
}

type contextKey string

const userContextKey contextKey = "user"

func WithUser(r *http.Request, uid int64, username, role string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, &claims{UserID: uid, Username: username, Role: role}))
}

func UserFromRequest(r *http.Request) (userID int64, username, role string, ok bool) {
	c, _ := r.Context().Value(userContextKey).(*claims)
	if c == nil {
		return 0, "", "", false
	}
	return c.UserID, c.Username, c.Role, true
}

func CheckAgentKey(key string) bool {
	return agentAPIKey != "" && key == agentAPIKey
}

// ValidateAppCredentials 校验 appId + appSecret，成功返回 true。
func ValidateAppCredentials(appId, appSecret string) bool {
	if appId == "" || appSecret == "" {
		return false
	}
	db, err := GetDB()
	if err != nil {
		return false
	}
	var hash string
	err = db.QueryRow("SELECT app_secret_hash FROM api_apps WHERE app_id = ?", appId).Scan(&hash)
	if err != nil || hash == "" {
		return false
	}
	return checkPassword(hash, appSecret)
}

// IssueJWTForApp 为 API 应用签发 JWT（无真实用户，username 为 appId，role 为 user）。
func IssueJWTForApp(appId string) (string, error) {
	return IssueJWT(0, appId, "user")
}

// CreateAPIApp 创建新 API 应用，自动生成 appId 与 appSecret，写入 api_apps 表。返回明文 appId、appSecret。
func CreateAPIApp(name string) (appId, appSecret string, err error) {
	idBuf := make([]byte, 8)
	if _, err = rand.Read(idBuf); err != nil {
		return "", "", err
	}
	appId = "bot_" + hex.EncodeToString(idBuf)
	secretBuf := make([]byte, 16)
	if _, err = rand.Read(secretBuf); err != nil {
		return "", "", err
	}
	appSecret = hex.EncodeToString(secretBuf)
	hash, err := HashPassword(appSecret)
	if err != nil {
		return "", "", err
	}
	db, err := GetDB()
	if err != nil {
		return "", "", err
	}
	_, err = db.Exec("INSERT INTO api_apps (app_id, app_secret_hash, name) VALUES (?, ?, ?)", appId, hash, name)
	if err != nil {
		return "", "", fmt.Errorf("insert api_app: %w", err)
	}
	return appId, appSecret, nil
}

func AuthUser(db *sql.DB, username, password string) (id int64, role string, err error) {
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
