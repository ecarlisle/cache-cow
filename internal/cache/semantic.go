package cache

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type EmbeddingRequest struct {
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

type SemanticCache struct {
	client    *http.Client
	endpoint  string
	enabled   bool
	db        *sql.DB
	threshold float32
	maxEntries int

	mu        sync.RWMutex
	vectors   [][]float32
	responses []semanticEntry
	models    []string
}

type semanticEntry struct {
	Response  json.RawMessage
	Model     string
	CachedAt  int64
}

func NewSemanticCache(endpoint, dbPath string, threshold float64, maxEntries int) *SemanticCache {
	enabled := endpoint != "" && dbPath != ""

	if !enabled {
		return &SemanticCache{enabled: false}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("semantic cache: opening db: %v", err)
		return &SemanticCache{enabled: false}
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS semantic_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			embedding BLOB NOT NULL,
			response TEXT NOT NULL,
			model TEXT NOT NULL,
			cached_at INTEGER NOT NULL
		)
	`); err != nil {
		log.Printf("semantic cache: creating table: %v", err)
		db.Close()
		return &SemanticCache{enabled: false}
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_semantic_cached_at ON semantic_cache(cached_at)`); err != nil {
		log.Printf("semantic cache: creating index: %v", err)
	}

	sc := &SemanticCache{
		client:     &http.Client{Timeout: 5 * time.Second},
		endpoint:   endpoint,
		enabled:    true,
		db:         db,
		threshold:  float32(threshold),
		maxEntries: maxEntries,
	}

	if err := sc.load(); err != nil {
		log.Printf("semantic cache: loading entries: %v", err)
	}

	go sc.periodicClean()

	return sc
}

func (s *SemanticCache) Close() error {
	if !s.enabled {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vectors = nil
	s.responses = nil
	s.models = nil
	return s.db.Close()
}

func (s *SemanticCache) Enabled() bool {
	return s.enabled
}

func (s *SemanticCache) Lookup(text string) (*semanticEntry, error) {
	if !s.enabled {
		return nil, nil
	}

	vec, err := s.Embed(text)
	if err != nil {
		return nil, fmt.Errorf("embedding: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	bestIdx := -1
	bestSim := float32(-1)

	for i := range s.vectors {
		sim := CosineSimilarity(vec, s.vectors[i])
		if sim > bestSim {
			bestSim = sim
			bestIdx = i
		}
	}

	if bestIdx >= 0 && bestSim >= s.threshold {
		log.Printf("  semantic hit: %.4f similarity", bestSim)
		return &s.responses[bestIdx], nil
	}

	return nil, nil
}

func (s *SemanticCache) Store(text string, response json.RawMessage, model string) {
	if !s.enabled {
		return
	}

	vec, err := s.Embed(text)
	if err != nil {
		log.Printf("  semantic store: embedding: %v", err)
		return
	}

	entry := semanticEntry{
		Response: response,
		Model:    model,
		CachedAt: time.Now().Unix(),
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, vec); err != nil {
		log.Printf("  semantic store: encoding: %v", err)
		return
	}

	if _, err := s.db.Exec(
		`INSERT INTO semantic_cache (embedding, response, model, cached_at) VALUES (?, ?, ?, ?)`,
		buf.Bytes(), string(response), model, entry.CachedAt,
	); err != nil {
		log.Printf("  semantic store: db: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.vectors) >= s.maxEntries {
		s.vectors = s.vectors[1:]
		s.responses = s.responses[1:]
		s.models = s.models[1:]
	}

	s.vectors = append(s.vectors, vec)
	s.responses = append(s.responses, entry)
	s.models = append(s.models, model)
}

func (s *SemanticCache) Embed(text string) ([]float32, error) {
	if !s.enabled {
		return nil, fmt.Errorf("semantic cache not configured")
	}

	body := &EmbeddingRequest{Input: text}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Post(s.endpoint+"/embed", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result EmbeddingResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	return result.Embedding, nil
}

func (s *SemanticCache) load() error {
	rows, err := s.db.Query(`SELECT embedding, response, model, cached_at FROM semantic_cache ORDER BY id`)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	for rows.Next() {
		var blob []byte
		var response, model string
		var cachedAt int64

		if err := rows.Scan(&blob, &response, &model, &cachedAt); err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		vec, err := decodeVector(blob)
		if err != nil {
			return fmt.Errorf("decode: %w", err)
		}

		s.vectors = append(s.vectors, vec)
		s.responses = append(s.responses, semanticEntry{
			Response: json.RawMessage(response),
			Model:    model,
			CachedAt: cachedAt,
		})
		s.models = append(s.models, model)
	}

	return nil
}

func (s *SemanticCache) periodicClean() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.db.Exec(`DELETE FROM semantic_cache WHERE cached_at < ?`, time.Now().Unix()-3600)
	}
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

func decodeVector(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("invalid vector length: %d", len(b))
	}
	vec := make([]float32, len(b)/4)
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, &vec); err != nil {
		return nil, err
	}
	return vec, nil
}
