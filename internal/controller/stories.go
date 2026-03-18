package controller

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lantingxu/internal/model"
)

func GetStoryByID(db *sql.DB, id int64, withChapters bool) (map[string]any, error) {
	var title, opening, tags, status string
	var creatorID sql.NullInt64
	var creatorUsername string
	var likeCount, commentCount, chapterCount int
	var scoreAvg sql.NullFloat64
	var scoreCount sql.NullInt64
	var createdAt, updatedAt string
	err := db.QueryRow(`SELECT s.title, s.opening, s.tags, s.status, s.creator_user_id, s.like_count, s.comment_count, s.chapter_count,
		COALESCE(s.score_avg, 0), COALESCE(s.score_count, 0), s.created_at, s.updated_at, COALESCE(u.username, '')
		FROM stories s LEFT JOIN users u ON s.creator_user_id = u.id WHERE s.id = ?`, id).Scan(
		&title, &opening, &tags, &status, &creatorID, &likeCount, &commentCount, &chapterCount,
		&scoreAvg, &scoreCount, &createdAt, &updatedAt, &creatorUsername)
	if err != nil {
		return nil, err
	}
	creatorUserId := int64(0)
	if creatorID.Valid {
		creatorUserId = creatorID.Int64
	}
	avg, cnt := 0.0, 0
	if scoreAvg.Valid {
		avg = scoreAvg.Float64
	}
	if scoreCount.Valid {
		cnt = int(scoreCount.Int64)
	}
	out := map[string]any{
		"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
		"creatorUserId": creatorUserId, "creatorUsername": creatorUsername, "likeCount": likeCount, "commentCount": commentCount, "chapterCount": chapterCount,
		"scoreAvg": avg, "scoreCount": cnt, "createdAt": createdAt, "updatedAt": updatedAt,
	}
	if !withChapters {
		return out, nil
	}
	rows, err := db.Query(`SELECT c.id, c.seq, c.content, c.author_user_id, c.author_agent_id, c.created_at,
		(SELECT COUNT(*) FROM chapter_likes WHERE chapter_id = c.id) AS like_count,
		COALESCE(u.username, '') AS author_username
		FROM chapters c LEFT JOIN users u ON c.author_user_id = u.id WHERE c.story_id = ? ORDER BY c.seq`, id)
	if err != nil {
		return out, nil
	}
	defer rows.Close()
	var chapters []map[string]any
	for rows.Next() {
		var chID, seq int64
		var content, agentID, authorUsername string
		var authorID sql.NullInt64
		var chCreated string
		var likeCount int
		_ = rows.Scan(&chID, &seq, &content, &authorID, &agentID, &chCreated, &likeCount, &authorUsername)
		uid := int64(0)
		if authorID.Valid {
			uid = authorID.Int64
		}
		chapters = append(chapters, map[string]any{
			"id": chID, "seq": seq, "content": content, "authorUserId": uid, "authorUsername": authorUsername, "authorAgentId": agentID, "createdAt": chCreated, "likeCount": likeCount,
		})
	}
	out["chapters"] = chapters
	return out, nil
}

func HandleStoriesRandom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "ongoing"
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var id int64
	err = db.QueryRow("SELECT id FROM stories WHERE status = ? ORDER BY RANDOM() LIMIT 1", status).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			WriteJSON(w, 200, map[string]any{"code": 0, "data": nil})
			return
		}
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	story, err := GetStoryByID(db, id, true)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func HandleStoriesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	status := q.Get("status")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	sort := q.Get("sort")
	offset := (page - 1) * limit

	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}

	baseSQL := "SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at, updated_at FROM stories WHERE 1=1"
	args := []any{}
	if status != "" {
		baseSQL += " AND status = ?"
		args = append(args, status)
	}
	orderBy := " ORDER BY id DESC"
	switch sort {
	case "hot":
		orderBy = " ORDER BY (like_count + comment_count + chapter_count) DESC, id DESC"
	case "new":
		orderBy = " ORDER BY created_at DESC, id DESC"
	}
	baseSQL += orderBy + " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(baseSQL, args...)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		var id int64
		var creatorID sql.NullInt64
		var title, opening, tags, status string
		var likeCount, commentCount, chapterCount int
		var createdAt, updatedAt string
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &likeCount, &commentCount, &chapterCount, &createdAt, &updatedAt)
		creatorUserId := int64(0)
		if creatorID.Valid {
			creatorUserId = creatorID.Int64
		}
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorUserId, "likeCount": likeCount, "commentCount": commentCount, "chapterCount": chapterCount,
			"createdAt": createdAt, "updatedAt": updatedAt,
		})
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "data": list, "page": page, "limit": limit})
}

func HandleStoryDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/stories/")
	if idx := strings.Index(idStr, "/"); idx >= 0 {
		idStr = idStr[:idx]
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	story, err := GetStoryByID(db, id, true)
	if err != nil {
		if err == sql.ErrNoRows {
			WriteJSON(w, 404, map[string]any{"code": 404, "message": "故事不存在"})
			return
		}
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func HandleStoryRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := model.UserFromRequest(r)
	if !ok {
		WriteJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/stories/")
	idStr = strings.TrimSuffix(idStr, "/rate")
	idStr = strings.Trim(idStr, "/")
	storyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || storyID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	var body struct {
		Score int `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	if body.Score < 0 || body.Score > 100 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "分数须在 0～100 之间"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, err = db.Exec(`INSERT INTO story_ratings (user_id, story_id, score) VALUES (?, ?, ?) ON CONFLICT(user_id, story_id) DO UPDATE SET score = excluded.score`,
		userID, storyID, body.Score)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, _ = db.Exec(`UPDATE stories SET score_avg = (SELECT COALESCE(AVG(score),0) FROM story_ratings WHERE story_id = ?), score_count = (SELECT COUNT(*) FROM story_ratings WHERE story_id = ?), updated_at = CURRENT_TIMESTAMP WHERE id = ?`, storyID, storyID, storyID)
	story, _ := GetStoryByID(db, storyID, true)
	WriteJSON(w, 200, map[string]any{"code": 0, "data": story, "myScore": body.Score})
}

func HandleStoryMyRating(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := model.UserFromRequest(r)
	if !ok {
		WriteJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"score": nil}})
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/stories/")
	idStr = strings.TrimSuffix(idStr, "/rating")
	idStr = strings.Trim(idStr, "/")
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
	var score sql.NullInt64
	_ = db.QueryRow("SELECT score FROM story_ratings WHERE user_id = ? AND story_id = ?", userID, storyID).Scan(&score)
	out := map[string]any{"score": nil}
	if score.Valid {
		out["score"] = int(score.Int64)
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "data": out})
}

func HandleStoryCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := model.UserFromRequest(r)
	if !ok {
		WriteJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	var body struct {
		Title   string `json:"title"`
		Opening string `json:"opening"`
		Tags    string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "缺少 title"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	res, err := db.Exec(`INSERT INTO stories (title, opening, tags, status, creator_user_id) VALUES (?, ?, ?, 'ongoing', ?)`,
		body.Title, body.Opening, body.Tags, userID)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	story, _ := GetStoryByID(db, id, false)
	WriteJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func HandleStoryAddChapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := model.UserFromRequest(r)
	if !ok {
		WriteJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/stories/")
	idStr = strings.TrimSuffix(idStr, "/chapters")
	idStr = strings.Trim(idStr, "/")
	storyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || storyID <= 0 {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	var body struct {
		Content       string `json:"content"`
		AuthorAgentID string `json:"authorAgentId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Content == "" {
		WriteJSON(w, 400, map[string]any{"code": 400, "message": "缺少 content"})
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var nextSeq int
	err = db.QueryRow("SELECT COALESCE(MAX(seq),0)+1 FROM chapters WHERE story_id = ?", storyID).Scan(&nextSeq)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	authorUID := userID
	if userID == 0 {
		authorUID = 0
	}
	res, err := db.Exec(`INSERT INTO chapters (story_id, seq, content, author_user_id, author_agent_id) VALUES (?, ?, ?, ?, ?)`,
		storyID, nextSeq, body.Content, authorUID, body.AuthorAgentID)
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, _ = db.Exec("UPDATE stories SET chapter_count = chapter_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", storyID)
	chID, _ := res.LastInsertId()
	WriteJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": chID, "seq": nextSeq, "storyId": storyID}})
}
