// 评审结论落在本机 ~/.klar/reviews.sqlite，按仓库和会话记，不写进仓库。
package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Saved struct {
	At      string
	Verdict string
	Text    string
	TurnID  string
}

func DefaultPath(home string) string {
	if home == "" {
		return filepath.Join(".klar", "reviews.sqlite")
	}
	return filepath.Join(home, ".klar", "reviews.sqlite")
}

func open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("建不了评审目录：%s", err.Error())
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("打不开评审记录：%s", err.Error())
	}
	if _, err := db.Exec(`create table if not exists reviews (
		repo text not null,
		sid text not null,
		at text not null,
		verdict text not null,
		text text not null,
		ok integer not null,
		turn_id text not null
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("建不了评审表：%s", err.Error())
	}
	// ok 是早先「有没有投递出去」留下的列，现在恒为 1。老库结构不动，免得迁移把已有结论弄丢。
	_, _ = db.Exec("pragma busy_timeout = 5000")
	return db, nil
}

// Latest 每个会话只留时间上最后一条。
func Latest(path, repo string) (map[string]Saved, error) {
	db, err := open(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`select sid, at, verdict, text, turn_id from reviews where repo = ? order by at, rowid`, repo)
	if err != nil {
		return nil, fmt.Errorf("读评审记录失败：%s", err.Error())
	}
	defer rows.Close()
	out := map[string]Saved{}
	for rows.Next() {
		var sid string
		var saved Saved
		if err := rows.Scan(&sid, &saved.At, &saved.Verdict, &saved.Text, &saved.TurnID); err != nil {
			return nil, fmt.Errorf("读评审记录失败：%s", err.Error())
		}
		out[sid] = saved
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读评审记录失败：%s", err.Error())
	}
	return out, nil
}

// Insert 追加一条。ok 是历史列，恒为 1。意见正文不进日志。
func Insert(path, repo, sid, verdict, text string, turnID string) error {
	db, err := open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	at := time.Now().Format("2006-01-02T15:04:05.000000000Z07:00")
	_, err = db.Exec(
		`insert into reviews (repo, sid, at, verdict, text, ok, turn_id) values (?, ?, ?, ?, ?, 1, ?)`,
		repo, sid, at, verdict, text, turnID,
	)
	if err != nil {
		return fmt.Errorf("写入评审记录失败：%s", err.Error())
	}
	log.Printf("记下评审 %s %s %s 回合 %s", repo, sid, verdict, turnID)
	return nil
}
