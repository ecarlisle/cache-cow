package cache

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ecarlisle/cache-cow/internal/types"
	_ "modernc.org/sqlite"
)

type ToolCache struct {
	db       *sql.DB
	ttl      int64
	stmtGet  *sql.Stmt
	stmtSet  *sql.Stmt
	stmtUpd  *sql.Stmt
}

type ToolCacheEntry struct {
	Response  json.RawMessage
	Model     string
	CachedAt  int64
}

func NewToolCache(path string, ttl int) (*ToolCache, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening tool cache db: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tool_cache (
			key TEXT PRIMARY KEY,
			response TEXT NOT NULL,
			model TEXT NOT NULL,
			cached_at INTEGER NOT NULL
		)
	`); err != nil {
		return nil, fmt.Errorf("creating tool cache table: %w", err)
	}

	get, err := db.Prepare(`SELECT response, model, cached_at FROM tool_cache WHERE key = ?`)
	if err != nil {
		return nil, fmt.Errorf("preparing tool get: %w", err)
	}

	set, err := db.Prepare(`INSERT OR REPLACE INTO tool_cache (key, response, model, cached_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("preparing tool set: %w", err)
	}

	upd, err := db.Prepare(`UPDATE tool_cache SET cached_at = ? WHERE key = ?`)
	if err != nil {
		return nil, fmt.Errorf("preparing tool touch: %w", err)
	}

	clean, err := db.Prepare(`DELETE FROM tool_cache WHERE cached_at < ?`)
	if err != nil {
		return nil, fmt.Errorf("preparing tool clean: %w", err)
	}
	_ = clean

	return &ToolCache{
		db:      db,
		ttl:     int64(ttl),
		stmtGet: get,
		stmtSet: set,
		stmtUpd: upd,
	}, nil
}

func (tc *ToolCache) Close() error {
	tc.stmtGet.Close()
	tc.stmtSet.Close()
	tc.stmtUpd.Close()
	return tc.db.Close()
}

func ToolRequestKey(messages []types.ChatMessage) (string, bool) {
	h := sha256.New()
	found := false

	for _, msg := range messages {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				h.Write([]byte(tc.Function.Name))
				h.Write([]byte(tc.Function.Arguments))
			}
			found = true
		}
		if msg.Role == "tool" && msg.ToolCallID != nil {
			h.Write([]byte(*msg.ToolCallID))
			h.Write(msg.Content)
			found = true
		}
	}

	if !found {
		return "", false
	}

	return fmt.Sprintf("%x", h.Sum(nil)), true
}

func (tc *ToolCache) Get(key string) (*ToolCacheEntry, error) {
	row := tc.stmtGet.QueryRow(key)
	var resp, model string
	var cachedAt int64
	if err := row.Scan(&resp, &model, &cachedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	now := time.Now().Unix()
	if now-cachedAt > tc.ttl {
		tc.stmtUpd.Exec(time.Now().Unix()-tc.ttl-1, key)
		return nil, nil
	}

	return &ToolCacheEntry{
		Response: json.RawMessage(resp),
		Model:    model,
		CachedAt: cachedAt,
	}, nil
}

func (tc *ToolCache) Set(key string, entry *ToolCacheEntry) error {
	_, err := tc.stmtSet.Exec(key, string(entry.Response), entry.Model, entry.CachedAt)
	return err
}
