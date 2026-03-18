package main

import (
	"database/sql"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	db   *sql.DB
	once sync.Once
)

func initDB() (*sql.DB, error) {
	var err error
	once.Do(func() {
		path := os.Getenv("DB_PATH")
		if path == "" {
			path = "lantingxu.db"
		}
		db, err = sql.Open("sqlite", path)
		if err != nil {
			return
		}
		db.SetMaxOpenConns(1) // SQLite 单写
		if err = migrate(db); err != nil {
			db.Close()
			db = nil
		}
	})
	return db, err
}

func migrate(d *sql.DB) error {
	schema := `
	-- 用户表
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		email TEXT,
		role TEXT DEFAULT 'user' CHECK(role IN ('user','admin')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 故事表（冗余点赞/评论/章节数以支撑热门榜）
	CREATE TABLE IF NOT EXISTS stories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		opening TEXT NOT NULL,
		tags TEXT,
		status TEXT DEFAULT 'ongoing' CHECK(status IN ('ongoing','completed')),
		creator_user_id INTEGER REFERENCES users(id),
		like_count INTEGER DEFAULT 0,
		comment_count INTEGER DEFAULT 0,
		chapter_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_stories_status ON stories(status);
	CREATE INDEX IF NOT EXISTS idx_stories_creator ON stories(creator_user_id);
	CREATE INDEX IF NOT EXISTS idx_stories_created ON stories(created_at DESC);

	-- 章节表
	CREATE TABLE IF NOT EXISTS chapters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		story_id INTEGER NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
		seq INTEGER NOT NULL,
		content TEXT NOT NULL,
		author_user_id INTEGER REFERENCES users(id),
		author_agent_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(story_id, seq)
	);
	CREATE INDEX IF NOT EXISTS idx_chapters_story ON chapters(story_id);

	-- 章节点赞（用户对某章节点一次赞）
	CREATE TABLE IF NOT EXISTS chapter_likes (
		chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (chapter_id, user_id)
	);

	-- 章节评论（支持软删）
	CREATE TABLE IF NOT EXISTS chapter_comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chapter_id INTEGER NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_comments_chapter ON chapter_comments(chapter_id);
	CREATE INDEX IF NOT EXISTS idx_comments_deleted ON chapter_comments(deleted_at);

	-- 推荐权重表（用于推荐榜，可多源分数）
	CREATE TABLE IF NOT EXISTS recommendation_weights (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		story_id INTEGER NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
		source TEXT NOT NULL,
		score REAL NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(story_id, source)
	);
	CREATE INDEX IF NOT EXISTS idx_rec_story ON recommendation_weights(story_id);
	`
	if _, err := d.Exec(schema); err != nil {
		return err
	}
	return seedStories(d)
}

// seedStories 在故事表为空时插入热榜展示用示例数据
func seedStories(d *sql.DB) error {
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM stories").Scan(&n); err != nil || n > 0 {
		return err
	}
	// 热榜按 like_count + comment_count + chapter_count 排序，插入有差异的示例
	rows := []struct {
		title   string
		opening string
		tags    string
		status  string
		like    int
		comment int
		chapter int
	}{
		{"春江花月夜", "春江潮水连海平，海上明月共潮生。", "古风,诗词", "completed", 320, 45, 12},
		{"长安十二时辰", "天宝三载，元月十四日，长安。", "悬疑,历史", "ongoing", 280, 62, 18},
		{"山海经异闻录", "大荒之中，有山名曰昆仑。", "志怪,奇幻", "ongoing", 256, 38, 9},
		{"墨香铜臭", "墨香一缕，铜臭半生。", "都市,成长", "completed", 198, 28, 8},
		{"青衫烟雨行", "青衫磊落险峰行，烟雨平生一剑名。", "武侠,江湖", "ongoing", 175, 41, 14},
		{"桃花庵下", "桃花坞里桃花庵，桃花庵下桃花仙。", "古风,田园", "completed", 142, 22, 6},
		{"浮生六记新编", "余生若梦，为欢几何。", "古典,情感", "ongoing", 128, 19, 7},
		{"云深不知处", "只在此山中，云深不知处。", "仙侠,修行", "ongoing", 95, 15, 5},
		{"锦瑟无端", "锦瑟无端五十弦，一弦一柱思华年。", "诗词,民国", "completed", 88, 12, 4},
		{"长夜将明", "长夜将明时，有人提灯而来。", "悬疑,治愈", "ongoing", 76, 18, 6},
		{"兰亭序外传", "永和九年，岁在癸丑，暮春之初。", "历史,书法", "ongoing", 54, 9, 3},
		{"墨池记", "临池学书，池水尽黑。", "古典,励志", "completed", 42, 7, 2},
	}
	for _, r := range rows {
		_, err := d.Exec(
			`INSERT INTO stories (title, opening, tags, status, creator_user_id, like_count, comment_count, chapter_count) VALUES (?, ?, ?, ?, NULL, ?, ?, ?)`,
			r.title, r.opening, r.tags, r.status, r.like, r.comment, r.chapter,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func getDB() (*sql.DB, error) {
	if db != nil {
		return db, nil
	}
	return initDB()
}

func timePtr(t time.Time) *time.Time { return &t }
