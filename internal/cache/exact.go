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

type ExactCache struct {
	db *sql.DB
	ttl int64
	stmtInsert *sql.Stmt
	stmtGet    *sql.Stmt
	stmtClean  *sql.Stmt
}

type CacheEntry struct {
	Response   json.RawMessage
	Model      string
	CachedAt   int64
}

func NewExactCache(path string, ttl int) (*ExactCache, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening cache db: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS response_cache (
			hash TEXT PRIMARY KEY,
			model TEXT NOT NULL,
			response TEXT NOT NULL,
			cached_at INTEGER NOT NULL
		)
	`); err != nil {
		return nil, fmt.Errorf("creating cache table: %w", err)
	}

	insert, err := db.Prepare(`
		INSERT OR REPLACE INTO response_cache (hash, model, response, cached_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return nil, fmt.Errorf("preparing insert: %w", err)
	}

	get, err := db.Prepare(`
		SELECT response, model, cached_at FROM response_cache WHERE hash = ?
	`)
	if err != nil {
		return nil, fmt.Errorf("preparing get: %w", err)
	}

	clean, err := db.Prepare(`DELETE FROM response_cache WHERE cached_at < ?`)
	if err != nil {
		return nil, fmt.Errorf("preparing clean: %w", err)
	}

	c := &ExactCache{
		db: db,
		ttl: int64(ttl),
		stmtInsert: insert,
		stmtGet:    get,
		stmtClean:  clean,
	}

	go c.periodicClean()

	return c, nil
}

func (c *ExactCache) Close() error {
	c.stmtInsert.Close()
	c.stmtGet.Close()
	c.stmtClean.Close()
	return c.db.Close()
}

func RequestKey(req *types.ChatRequest) (string, error) {
	h := sha256.New()
	h.Write([]byte(req.Model))
	for _, msg := range req.Messages {
		h.Write([]byte(msg.Role))
		h.Write([]byte(msg.Content))
		if msg.Name != nil {
			h.Write([]byte(*msg.Name))
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (c *ExactCache) Get(hash string) (*CacheEntry, error) {
	row := c.stmtGet.QueryRow(hash)
	var resp, model string
	var cachedAt int64
	if err := row.Scan(&resp, &model, &cachedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	now := time.Now().Unix()
	if now-cachedAt > c.ttl {
		c.stmtClean.Exec(cachedAt)
		return nil, nil
	}

	return &CacheEntry{
		Response: json.RawMessage(resp),
		Model:    model,
		CachedAt: cachedAt,
	}, nil
}

func (c *ExactCache) Set(hash string, entry *CacheEntry) error {
	_, err := c.stmtInsert.Exec(hash, entry.Model, string(entry.Response), entry.CachedAt)
	return err
}

func (c *ExactCache) periodicClean() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Unix() - c.ttl
		c.stmtClean.Exec(cutoff)
	}
}
