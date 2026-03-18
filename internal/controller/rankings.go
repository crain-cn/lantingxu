package controller

import (
	"database/sql"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lantingxu/internal/model"
)

var (
	hotCache     []map[string]any
	hotCacheTime time.Time
	hotCacheMu   sync.RWMutex
	hotCacheTTL  = 60 * time.Second
)

func ClearHotCache() {
	hotCacheMu.Lock()
	hotCache = nil
	hotCacheMu.Unlock()
}

func HandleRankingsHot(w http.ResponseWriter, r *http.Request) {
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
		WriteJSON(w, 200, map[string]any{"code": 0, "data": data})
		return
	}
	hotCacheMu.RUnlock()

	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	query := `SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at FROM stories WHERE 1=1`
	args := []any{}
	if status == "completed" || status == "ongoing" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY (like_count + comment_count + chapter_count) DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(query, args...)
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
	if useCache {
		hotCacheMu.Lock()
		hotCache = list
		hotCacheTime = time.Now()
		hotCacheMu.Unlock()
	}
	WriteJSON(w, 200, map[string]any{"code": 0, "data": list})
}

func HandleRankingsNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	query := `SELECT id, title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count, created_at FROM stories WHERE 1=1`
	args := []any{}
	if status == "completed" || status == "ongoing" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(query, args...)
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
	WriteJSON(w, 200, map[string]any{"code": 0, "data": list})
}

func HandleRankingsRecommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	db, err := model.GetDB()
	if err != nil {
		WriteJSON(w, 500, map[string]any{"code": 500, "message": err.Error()})
		return
	}
	limit := 20
	if l, _ := strconv.Atoi(r.URL.Query().Get("limit")); l > 0 && l <= 100 {
		limit = l
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	query := `
		SELECT s.id, s.title, s.opening, s.tags, s.status, s.creator_user_id, s.like_count, s.comment_count, s.chapter_count, s.created_at,
		       COALESCE(SUM(r.score), 0) as rec_score
		FROM stories s
		LEFT JOIN recommendation_weights r ON r.story_id = s.id
		WHERE 1=1`
	args := []any{}
	if status == "completed" || status == "ongoing" {
		query += ` AND s.status = ?`
		args = append(args, status)
	}
	query += ` GROUP BY s.id ORDER BY rec_score DESC, s.created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(query, args...)
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
		var recScore float64
		_ = rows.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt, &recScore)
		creatorUserId := int64(0)
		if creatorID.Valid {
			creatorUserId = creatorID.Int64
		}
		list = append(list, map[string]any{
			"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
			"creatorUserId": creatorUserId, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt, "recommendScore": recScore,
		})
	}
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
				var id int64
				var creatorID sql.NullInt64
				var title, opening, tags, status, createdAt string
				var lc, cc, chc int
				_ = rows2.Scan(&id, &title, &opening, &tags, &status, &creatorID, &lc, &cc, &chc, &createdAt)
				creatorUserId := int64(0)
				if creatorID.Valid {
					creatorUserId = creatorID.Int64
				}
				if !seen[id] {
					seen[id] = true
					list = append(list, map[string]any{
						"id": id, "title": title, "opening": opening, "tags": tags, "status": status,
						"creatorUserId": creatorUserId, "likeCount": lc, "commentCount": cc, "chapterCount": chc, "createdAt": createdAt,
					})
				}
			}
			rows2.Close()
		}
	}
	rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	WriteJSON(w, 200, map[string]any{"code": 0, "data": list})
}
