package model

// BotTickerHistory 返回最近 limit 条 bot（author_agent_id 以 bot_ 开头）的开篇与续写，
// 与 WebSocket 推送结构一致，按时间正序（旧→新）便于与后续实时消息衔接。
func BotTickerHistory(limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	const q = `
SELECT typ, story_id, title, agent FROM (
  SELECT 'create' AS typ, id AS story_id, title, COALESCE(author_agent_id, '') AS agent, created_at AS ts
  FROM stories WHERE author_agent_id LIKE 'bot_%'
  UNION ALL
  SELECT 'chapter', c.story_id, s.title, COALESCE(c.author_agent_id, ''), c.created_at
  FROM chapters c JOIN stories s ON s.id = c.story_id WHERE c.author_agent_id LIKE 'bot_%'
) ORDER BY ts DESC LIMIT ?`
	rows, err := db.Query(q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		typ     string
		storyID int64
		title   string
		agent   string
	}
	var rev []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.typ, &r.storyID, &r.title, &r.agent); err != nil {
			return nil, err
		}
		rev = append(rev, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		r := rev[i]
		title := r.title
		if title == "" {
			title = "未命名"
		}
		agent := r.agent
		if agent == "" {
			agent = "某用户"
		}
		m := map[string]any{
			"type":      r.typ,
			"agentName": agent,
			"title":     title,
			"storyId":   r.storyID,
		}
		out = append(out, m)
	}
	return out, nil
}
