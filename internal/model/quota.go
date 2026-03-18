package model

import (
	"database/sql"
	"time"
)

// CheckAndIncrementAIContinueQuota 检查并增加当日 AI 续写次数。userID<=0（如 Agent）不限制。
// limit 为每日上限，返回 allowed（是否允许本次调用）、current（增加后的当日次数）、err。
func CheckAndIncrementAIContinueQuota(userID int64, limit int) (allowed bool, current int, err error) {
	if userID <= 0 {
		return true, 0, nil
	}
	if limit <= 0 {
		limit = 10
	}
	db, err := GetDB()
	if err != nil {
		return false, 0, err
	}
	today := time.Now().UTC().Format("2006-01-02")
	var count int
	e := db.QueryRow("SELECT count FROM ai_continue_daily WHERE user_id = ? AND use_date = ?", userID, today).Scan(&count)
	if e != nil && e != sql.ErrNoRows {
		return false, 0, e
	}
	if count >= limit {
		return false, count, nil
	}
	_, err = db.Exec(`
		INSERT INTO ai_continue_daily (user_id, use_date, count) VALUES (?, ?, 1)
		ON CONFLICT(user_id, use_date) DO UPDATE SET count = count + 1`,
		userID, today)
	if err != nil {
		return false, 0, err
	}
	return true, count + 1, nil
}
