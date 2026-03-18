package controller

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lantingxu/internal/model"
)

func HandleChapterLike(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := model.UserFromRequest(r)
	if !ok {
		WriteJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/chapters/")
	path = strings.TrimSuffix(path, "/like")
	chapterID, err := strconv.ParseInt(strings.Trim(path, "/"), 10, 64)
	if err != nil || chapterID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效章节 id"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, err = db.Exec("INSERT INTO chapter_likes (chapter_id, user_id) VALUES (?, ?)", chapterID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			WriteJSON(w, 200, map[string]any{"code": 0, "message": "已点赞"})
			return
		}
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var storyID int64
	_ = db.QueryRow("SELECT story_id FROM chapters WHERE id = ?", chapterID).Scan(&storyID)
	if storyID > 0 {
		_, _ = db.Exec("UPDATE stories SET like_count = like_count + 1 WHERE id = ?", storyID)
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "message": "ok"})
}

func HandleChapterComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := model.UserFromRequest(r)
	if !ok {
		WriteJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/chapters/")
	path = strings.TrimSuffix(path, "/comment")
	chapterID, err := strconv.ParseInt(strings.Trim(path, "/"), 10, 64)
	if err != nil || chapterID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效章节 id"})
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "缺少 content"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	res, err := db.Exec("INSERT INTO chapter_comments (chapter_id, user_id, content) VALUES (?, ?, ?)", chapterID, userID, body.Content)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var storyID int64
	_ = db.QueryRow("SELECT story_id FROM chapters WHERE id = ?", chapterID).Scan(&storyID)
	if storyID > 0 {
		_, _ = db.Exec("UPDATE stories SET comment_count = comment_count + 1 WHERE id = ?", storyID)
	}
	cid, _ := res.LastInsertId()
	WriteJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": cid, "chapterId": chapterID}})
}

func HandleChapterCommentsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/chapters/")
	path = strings.TrimSuffix(path, "/comments")
	chapterID, err := strconv.ParseInt(strings.Trim(path, "/"), 10, 64)
	if err != nil || chapterID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效章节 id"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	rows, err := db.Query(`SELECT cc.id, cc.content, cc.user_id, COALESCE(u.username, ''), cc.created_at
		FROM chapter_comments cc LEFT JOIN users u ON u.id = cc.user_id
		WHERE cc.chapter_id = ? AND cc.deleted_at IS NULL ORDER BY cc.created_at ASC`, chapterID)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id int64
		var content, username, createdAt string
		var userID sql.NullInt64
		_ = rows.Scan(&id, &content, &userID, &username, &createdAt)
		uid := int64(0)
		if userID.Valid {
			uid = userID.Int64
		}
		list = append(list, map[string]any{"id": id, "content": content, "userId": uid, "username": username, "createdAt": createdAt})
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "data": list})
}
