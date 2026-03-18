package main

import (
	"database/sql"
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- 用户 ----------
func handleRegister(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 username 或 password"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	res, err := db.Exec("INSERT INTO users (username, password_hash, email, role) VALUES (?, ?, ?, 'user')",
		body.Username, hash, body.Email)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "用户名已存在"})
			return
		}
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	token, _ := issueJWT(id, body.Username, "user")
	writeJSON(w, 200, map[string]any{"code": 0, "userId": id, "username": body.Username, "token": token})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 username 或 password"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	id, role, err := authUser(db, body.Username, body.Password)
	if err != nil {
		writeJSON(w, 401, map[string]any{"code": 401, "message": err.Error()})
		return
	}
	token, _ := issueJWT(id, body.Username, role)
	writeJSON(w, 200, map[string]any{"code": 0, "userId": id, "username": body.Username, "token": token})
}

// ---------- 故事 ----------
func handleStoriesRandom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "ongoing"
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var id int64
	err = db.QueryRow("SELECT id FROM stories WHERE status = ? ORDER BY RANDOM() LIMIT 1", status).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, 200, map[string]any{"code": 0, "data": nil})
			return
		}
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	story, err := getStoryByID(db, id, true)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func handleStoriesList(w http.ResponseWriter, r *http.Request) {
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
	sort := q.Get("sort") // hot, new, 默认 id
	offset := (page - 1) * limit

	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
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
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()

	var list []map[string]any
	for rows.Next() {
		var id, creatorID int64
		var title, opening, tags, status string
		var likeCount, commentCount, chapterCount int
		var createdAt, updatedAt string
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &likeCount, &commentCount, &chapterCount, &createdAt, &updatedAt)
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorID, "likeCount": likeCount, "commentCount": commentCount, "chapterCount": chapterCount,
			"createdAt": createdAt, "updatedAt": updatedAt,
		})
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": list, "page": page, "limit": limit})
}

func handleStoryDetail(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	story, err := getStoryByID(db, id, true)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, 404, map[string]any{"code": 404, "message": "故事不存在"})
			return
		}
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func getStoryByID(db *sql.DB, id int64, withChapters bool) (map[string]any, error) {
	var title, opening, tags, status string
	var creatorID int64
	var likeCount, commentCount, chapterCount int
	var createdAt, updatedAt string
	err := db.QueryRow(`SELECT title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at, updated_at FROM stories WHERE id = ?`, id).Scan(
		&title, &opening, &tags, &status, &creatorID, &likeCount, &commentCount, &chapterCount, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
		"creatorUserId": creatorID, "likeCount": likeCount, "commentCount": commentCount, "chapterCount": chapterCount,
		"createdAt": createdAt, "updatedAt": updatedAt,
	}
	if !withChapters {
		return out, nil
	}
	rows, err := db.Query(`SELECT id, seq, content, author_user_id, author_agent_id, created_at FROM chapters WHERE story_id = ? ORDER BY seq`, id)
	if err != nil {
		return out, nil
	}
	defer rows.Close()
	var chapters []map[string]any
	for rows.Next() {
		var chID, seq int64
		var content, agentID string
		var authorID sql.NullInt64
		var chCreated string
		_ = rows.Scan(&chID, &seq, &content, &authorID, &agentID, &chCreated)
		uid := int64(0)
		if authorID.Valid {
			uid = authorID.Int64
		}
		chapters = append(chapters, map[string]any{
			"id": chID, "seq": seq, "content": content, "authorUserId": uid, "authorAgentId": agentID, "createdAt": chCreated,
		})
	}
	out["chapters"] = chapters
	return out, nil
}

func handleStoryCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := userFromRequest(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	var body struct {
		Title   string `json:"title"`
		Opening string `json:"opening"`
		Tags    string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 title"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	res, err := db.Exec(`INSERT INTO stories (title, opening, tags, status, creator_user_id) VALUES (?, ?, ?, 'ongoing', ?)`,
		body.Title, body.Opening, body.Tags, userID)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	story, _ := getStoryByID(db, id, false)
	writeJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func handleStoryAddChapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := userFromRequest(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/stories/")
	idStr = strings.TrimSuffix(idStr, "/chapters")
	idStr = strings.Trim(idStr, "/")
	storyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || storyID <= 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	var body struct {
		Content       string `json:"content"`
		AuthorAgentID string `json:"authorAgentId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Content == "" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 content"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var nextSeq int
	err = db.QueryRow("SELECT COALESCE(MAX(seq),0)+1 FROM chapters WHERE story_id = ?", storyID).Scan(&nextSeq)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	authorUID := userID
	if userID == 0 {
		authorUID = 0 // Agent 续写时可为 0
	}
	res, err := db.Exec(`INSERT INTO chapters (story_id, seq, content, author_user_id, author_agent_id) VALUES (?, ?, ?, ?, ?)`,
		storyID, nextSeq, body.Content, authorUID, body.AuthorAgentID)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, _ = db.Exec("UPDATE stories SET chapter_count = chapter_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", storyID)
	chID, _ := res.LastInsertId()
	writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": chID, "seq": nextSeq, "storyId": storyID}})
}

// ---------- 榜单 ----------
var (
	hotCache      []map[string]any
	hotCacheTime  time.Time
	hotCacheMu    sync.RWMutex
	hotCacheTTL   = 60 * time.Second
)

func handleRankingsHot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	useCache := (status == "" || status == "all")
	hotCacheMu.RLock()
	if useCache && time.Since(hotCacheTime) < hotCacheTTL && len(hotCache) > 0 {
		data := hotCache
		hotCacheMu.RUnlock()
		writeJSON(w, 200, map[string]any{"code": 0, "data": data})
		return
	}
	hotCacheMu.RUnlock()

	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	sql := `SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at FROM stories WHERE 1=1`
	args := []any{}
	if status == "completed" || status == "ongoing" {
		sql += ` AND status = ?`
		args = append(args, status)
	}
	sql += ` ORDER BY (like_count + comment_count + chapter_count) DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(sql, args...)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id, creatorID int64
		var title, opening, tags, status, createdAt string
		var lc, cc, chc int
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt)
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorID, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt,
		})
	}
	if useCache {
		hotCacheMu.Lock()
		hotCache = list
		hotCacheTime = time.Now()
		hotCacheMu.Unlock()
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": list})
}

func handleRankingsNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	sql := `SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at FROM stories WHERE 1=1`
	args := []any{}
	if status == "completed" || status == "ongoing" {
		sql += ` AND status = ?`
		args = append(args, status)
	}
	sql += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(sql, args...)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id, creatorID int64
		var title, opening, tags, status, createdAt string
		var lc, cc, chc int
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt)
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorID, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt,
		})
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": list})
}

func handleRankingsRecommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	sql := `
		SELECT s.id, s.title, s.opening, s.tags, s.status, s.creator_user_id, s.like_count, s.comment_count, s.chapter_count, s.created_at,
		       COALESCE(SUM(r.score), 0) as rec_score
		FROM stories s
		LEFT JOIN recommendation_weights r ON r.story_id = s.id
		WHERE 1=1`
	args := []any{}
	if status == "completed" || status == "ongoing" {
		sql += ` AND s.status = ?`
		args = append(args, status)
	}
	sql += ` GROUP BY s.id ORDER BY rec_score DESC, s.created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(sql, args...)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id, creatorID int64
		var title, opening, tags, status, createdAt string
		var lc, cc, chc int
		var recScore float64
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt, &recScore)
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorID, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt, "recommendScore": recScore,
		})
	}
	// 若结果太少，用随机新书补足（简化版“个性化”）
	if len(list) < limit {
		sql2 := `SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at FROM stories WHERE 1=1`
		args2 := []any{}
		if status == "completed" || status == "ongoing" {
			sql2 += ` AND status = ?`
			args2 = append(args2, status)
		}
		sql2 += ` ORDER BY RANDOM() LIMIT ?`
		args2 = append(args2, limit-len(list))
		rows2, _ := db.Query(sql2, args2...)
		if rows2 != nil {
			seen := make(map[int64]bool)
			for _, s := range list {
				seen[int64(s["id"].(int64))] = true
			}
			for rows2.Next() {
				var id, creatorID int64
				var title, opening, tags, status, createdAt string
				var lc, cc, chc int
				_ = rows2.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt)
				if !seen[id] {
					seen[id] = true
					list = append(list, map[string]any{
						"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
						"creatorUserId": creatorID, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt,
					})
				}
			}
			rows2.Close()
		}
	}
	// 打乱顺序避免总是同一批
	rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	writeJSON(w, 200, map[string]any{"code": 0, "data": list})
}

// ---------- 互动 ----------
func handleChapterLike(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := userFromRequest(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/chapters/")
	path = strings.TrimSuffix(path, "/like")
	chapterID, err := strconv.ParseInt(strings.Trim(path, "/"), 10, 64)
	if err != nil || chapterID <= 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效章节 id"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, err = db.Exec("INSERT INTO chapter_likes (chapter_id, user_id) VALUES (?, ?)", chapterID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, 200, map[string]any{"code": 0, "message": "已点赞"})
			return
		}
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var storyID int64
	_ = db.QueryRow("SELECT story_id FROM chapters WHERE id = ?", chapterID).Scan(&storyID)
	if storyID > 0 {
		_, _ = db.Exec("UPDATE stories SET like_count = like_count + 1 WHERE id = ?", storyID)
	}
	writeJSON(w, 200, map[string]any{"code": 0, "message": "ok"})
}

func handleChapterComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, _, ok := userFromRequest(r)
	if !ok {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "需要登录"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/chapters/")
	path = strings.TrimSuffix(path, "/comment")
	chapterID, err := strconv.ParseInt(strings.Trim(path, "/"), 10, 64)
	if err != nil || chapterID <= 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效章节 id"})
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "缺少 content"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	res, err := db.Exec("INSERT INTO chapter_comments (chapter_id, user_id, content) VALUES (?, ?, ?)", chapterID, userID, body.Content)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var storyID int64
	_ = db.QueryRow("SELECT story_id FROM chapters WHERE id = ?", chapterID).Scan(&storyID)
	if storyID > 0 {
		_, _ = db.Exec("UPDATE stories SET comment_count = comment_count + 1 WHERE id = ?", storyID)
	}
	cid, _ := res.LastInsertId()
	writeJSON(w, 200, map[string]any{"code": 0, "data": map[string]any{"id": cid, "chapterId": chapterID}})
}

// ---------- 管理员：故事/评论审核、修改、删除 ----------
func handleAdminStoriesList(w http.ResponseWriter, r *http.Request) {
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
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	rows, err := db.Query(`SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at FROM stories ORDER BY id DESC LIMIT ? OFFSET ?`, limit, (page-1)*limit)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id, creatorID int64
		var title, opening, tags, status, createdAt string
		var lc, cc, chc int
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt)
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorID, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt,
		})
	}
	writeJSON(w, 200, map[string]any{"code": 0, "data": list, "page": page, "limit": limit})
}

func handleAdminStoryUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/stories/")
	idStr = strings.TrimSuffix(idStr, "/")
	storyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || storyID <= 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	var body struct {
		Title   *string `json:"title"`
		Opening *string `json:"opening"`
		Tags    *string `json:"tags"`
		Status  *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效请求体"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
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
	story, _ := getStoryByID(db, storyID, false)
	writeJSON(w, 200, map[string]any{"code": 0, "data": story})
}

func handleAdminStoryDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/stories/")
	idStr = strings.TrimSuffix(idStr, "/")
	storyID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || storyID <= 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效故事 id"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	_, err = db.Exec("DELETE FROM stories WHERE id = ?", storyID)
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	hotCacheMu.Lock()
	hotCache = nil
	hotCacheMu.Unlock()
	writeJSON(w, 200, map[string]any{"code": 0, "message": "已删除"})
}

func handleAdminCommentDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/comments/")
	commentID, err := strconv.ParseInt(strings.TrimSuffix(idStr, "/"), 10, 64)
	if err != nil || commentID <= 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "无效评论 id"})
		return
	}
	db, err := getDB()
	if err != nil {
		writeJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	var chapterID int64
	var deletedAt sql.NullString
	err = db.QueryRow("SELECT chapter_id, deleted_at FROM chapter_comments WHERE id = ?", commentID).Scan(&chapterID, &deletedAt)
	if err != nil {
		writeJSON(w, 404, map[string]any{"code": 404, "message": "评论不存在"})
		return
	}
	if deletedAt.Valid {
		writeJSON(w, 200, map[string]any{"code": 0, "message": "已删除"})
		return
	}
	_, _ = db.Exec("UPDATE chapter_comments SET deleted_at = CURRENT_TIMESTAMP WHERE id = ?", commentID)
	_, _ = db.Exec("UPDATE stories SET comment_count = comment_count - 1 WHERE id = (SELECT story_id FROM chapters WHERE id = ?)", chapterID)
	writeJSON(w, 200, map[string]any{"code": 0, "message": "已删除"})
}
