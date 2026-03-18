package controller

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lantingxu/internal/model"
)

func HandleAdminStoriesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	rows, err := db.Query(`SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at FROM stories ORDER BY id DESC LIMIT ? OFFSET ?`, limit, (page-1)*limit)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id int64
		var creatorID sql.NullInt64
		var title, opening, tags, status, createdAt string
		var lc, cc, chc int
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt)
		creatorUserId := int64(0)
		if creatorID.Valid {
			creatorUserId = creatorID.Int64
		}
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorUserId, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt,
		})
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "data": list, "page": page, "limit": limit})
}

func HandleAdminStoryUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/stories/")
	idStr = strings.TrimSuffix(idStr, "/")
	storyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || storyID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	var body struct {
		Title   *string `json:"title"`
		Opening *string `json:"opening"`
		Tags    *string `json:"tags"`
		Status  *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	if body.Title != nil {
		_, _ = db.Exec("UPDATE stories SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", *body.Title, storyID)
	}
	if body.Opening != nil {
		_, _ = db.Exec("UPDATE stories SET opening = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", *body.Opening, storyID)
	}
	if body.Tags != nil {
		_, _ = db.Exec("UPDATE stories SET tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", *body.Tags, storyID)
	}
	if body.Status != nil && (*body.Status == "ongoing" || *body.Status == "completed") {
		_, _ = db.Exec("UPDATE stories SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", *body.Status, storyID)
	}
	story, _ := GetStoryByID(db, storyID, false)
	WriteJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func HandleAdminStoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/stories/")
	idStr = strings.TrimSuffix(idStr, "/")
	storyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || storyID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, err = db.Exec("DELETE FROM stories WHERE id = ?", storyID)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	ClearHotCache()
	WriteJSON(w, 200, map[string]any{"code": 0, "message": "已删除"})
}

func HandleAdminCommentDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/comments/")
	commentID, err := strconv.ParseInt(strings.TrimSuffix(idStr, "/"), 10, 64)
	if err != nil || commentID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效评论 id"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var chapterID int64
	var deletedAt sql.NullString
	err = db.QueryRow("SELECT chapter_id, deleted_at FROM chapter_comments WHERE id = ?", commentID).Scan(&chapterID, &deletedAt)
	if err != nil {
		WriteJSON(w, 404, map[string]any{"code": 404, "message": "评论不存在"})
		return
	}
	if deletedAt.Valid {
		WriteJSON(w, 200, map[string]any{"code": 0, "message": "已删除"})
		return
	}
	_, _ = db.Exec("UPDATE chapter_comments SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", commentID)
	_, _ = db.Exec("UPDATE stories SET comment_count = comment_count - 1 WHERE id = (SELECT story_id FROM chapters WHERE id = ?)", chapterID)
	WriteJSON(w, 200, map[string]any{"code": 0, "message": "已删除"})
}
