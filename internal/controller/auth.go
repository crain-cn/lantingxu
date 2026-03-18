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
