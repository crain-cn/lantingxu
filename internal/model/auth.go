package model

import (
	"context"
	"database/sql"
	"errors"
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
