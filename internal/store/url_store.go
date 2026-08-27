package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	cfg *config.Config

	mu      sync.RWMutex
	records map[string]model.ShortURL
	loaded  bool
	closed  bool

	panicGuard PanicGuardFn

	readBuf    []byte
	sharedBuf  [4096]byte
	sharedUsed int
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	s := &URLStore{
		cfg:     cfg,
		records: make(map[string]model.ShortURL),
	}
	return s, nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("url store closed")
	}

	path := s.cfg.Storage.URLFile()
	if path == "" {
		s.records = make(map[string]model.ShortURL)
		s.loaded = true
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.records = make(map[string]model.ShortURL)
			s.loaded = true
			return nil
		}
		return err
	}

	s.readBuf = data

	recs := make(map[string]model.ShortURL)
	if err := json.Unmarshal(s.readBuf, &recs); err != nil {
		return err
	}
	if recs == nil {
		recs = make(map[string]model.ShortURL)
	}
	s.bindSharedStringsLocked(recs)
	s.records = recs
	s.loaded = true
	return nil
}

type sharedEntry struct {
	ID    string
	Value model.ShortURL
}

func (s *URLStore) bindSharedStringsLocked(recs map[string]model.ShortURL) {
	if len(s.readBuf) == 0 {
		return
	}
	for _, se := range s.readEntriesLocked(s.readBuf) {
		existing, ok := recs[se.ID]
		if !ok {
			continue
		}
		existing.Code = se.Value.Code
		existing.RawURL = se.Value.RawURL
		recs[se.ID] = existing
	}
}

func (s *URLStore) readEntriesLocked(buf []byte) []sharedEntry {
	var out []sharedEntry
	pos := skipWsLocked(buf, 0)
	if pos >= len(buf) || buf[pos] != '{' {
		return out
	}
	pos++
	for pos < len(buf) {
		pos = skipWsLocked(buf, pos)
		if pos >= len(buf) || buf[pos] == '}' {
			break
		}
		if buf[pos] == ',' {
			pos++
			continue
		}
		nextPos, mapKey := parseStrFieldLocked(buf, pos)
		if nextPos < 0 {
			break
		}
		pos = nextPos
		pos = skipWsLocked(buf, pos)
		if pos >= len(buf) || buf[pos] != ':' {
			break
		}
		pos++
		pos = skipWsLocked(buf, pos)
		if pos >= len(buf) || buf[pos] != '{' {
			break
		}
		pos++
		var val model.ShortURL
		pos = scanEntryObjectLocked(buf, pos, &val)
		if pos < 0 {
			break
		}
		out = append(out, sharedEntry{ID: mapKey, Value: val})
	}
	return out
}

func skipWsLocked(buf []byte, pos int) int {
	for pos < len(buf) && (buf[pos] == ' ' || buf[pos] == '\n' || buf[pos] == '\r' || buf[pos] == '\t') {
		pos++
	}
	return pos
}

func parseStrFieldLocked(buf []byte, pos int) (int, string) {
	if pos >= len(buf) || buf[pos] != '"' {
		return -1, ""
	}
	end := findClosingQuoteLocked(buf, pos+1)
	if end < 0 {
		return -1, ""
	}
	return end + 1, unsafe.String(&buf[pos+1], end-pos-1)
}

func findClosingQuoteLocked(buf []byte, pos int) int {
	for pos < len(buf) {
		if buf[pos] == '\\' {
			pos += 2
			continue
		}
		if buf[pos] == '"' {
			return pos
		}
		pos++
	}
	return -1
}

func scanEntryObjectLocked(buf []byte, pos int, entry *model.ShortURL) int {
	for pos < len(buf) {
		pos = skipWsLocked(buf, pos)
		if pos >= len(buf) {
			return -1
		}
		if buf[pos] == '}' {
			return pos + 1
		}
		if buf[pos] == ',' {
			pos++
			continue
		}
		if buf[pos] != '"' {
			pos++
			continue
		}
		nextPos, key := parseStrFieldLocked(buf, pos)
		if nextPos < 0 {
			return -1
		}
		pos = nextPos
		pos = skipWsLocked(buf, pos)
		if pos >= len(buf) || buf[pos] != ':' {
			return -1
		}
		pos++
		pos = skipWsLocked(buf, pos)
		switch key {
		case "code":
			pos, entry.Code = parseStrFieldLocked(buf, pos)
		case "raw_url":
			pos, entry.RawURL = parseStrFieldLocked(buf, pos)
		case "created_at":
			nextPos, _ = parseStrFieldLocked(buf, pos)
			pos = nextPos
		case "visits":
			nextPos, val := parseIntFieldLocked(buf, pos)
			pos = nextPos
			entry.Visits = val
		case "max_visits":
			nextPos, val := parseIntFieldLocked(buf, pos)
			pos = nextPos
			entry.MaxVisits = val
		case "custom":
			nextPos, val := parseBoolFieldLocked(buf, pos)
			pos = nextPos
			entry.Custom = val
		case "disabled":
			nextPos, val := parseBoolFieldLocked(buf, pos)
			pos = nextPos
			entry.Disabled = val
		default:
			nextPos = skipValueLocked(buf, pos)
			pos = nextPos
		}
		if pos < 0 {
			return -1
		}
	}
	return -1
}

func parseIntFieldLocked(buf []byte, pos int) (int, int) {
	pos = skipWsLocked(buf, pos)
	start := pos
	if pos < len(buf) && (buf[pos] == '-' || buf[pos] == '+') {
		pos++
	}
	for pos < len(buf) && buf[pos] >= '0' && buf[pos] <= '9' {
		pos++
	}
	if start == pos {
		return -1, 0
	}
	v := 0
	neg := false
	for _, c := range buf[start:pos] {
		if c == '-' {
			neg = true
			continue
		}
		if c == '+' {
			continue
		}
		v = v*10 + int(c-'0')
	}
	if neg {
		v = -v
	}
	return pos, v
}

func parseBoolFieldLocked(buf []byte, pos int) (int, bool) {
	pos = skipWsLocked(buf, pos)
	if pos+4 <= len(buf) && buf[pos] == 't' && buf[pos+1] == 'r' && buf[pos+2] == 'u' && buf[pos+3] == 'e' {
		return pos + 4, true
	}
	if pos+5 <= len(buf) && buf[pos] == 'f' && buf[pos+1] == 'a' && buf[pos+2] == 'l' && buf[pos+3] == 's' && buf[pos+4] == 'e' {
		return pos + 5, false
	}
	return -1, false
}

func skipValueLocked(buf []byte, pos int) int {
	pos = skipWsLocked(buf, pos)
	if pos >= len(buf) {
		return -1
	}
	switch buf[pos] {
	case '"':
		p, _ := parseStrFieldLocked(buf, pos)
		return p
	case '{':
		depth := 1
		pos++
		for pos < len(buf) && depth > 0 {
			if buf[pos] == '"' {
				p, _ := parseStrFieldLocked(buf, pos)
				if p < 0 {
					return -1
				}
				pos = p
				continue
			}
			if buf[pos] == '{' {
				depth++
			} else if buf[pos] == '}' {
				depth--
			}
			pos++
		}
		return pos
	case '[':
		depth := 1
		pos++
		for pos < len(buf) && depth > 0 {
			if buf[pos] == '[' {
				depth++
			} else if buf[pos] == ']' {
				depth--
			}
			pos++
		}
		return pos
	default:
		for pos < len(buf) && buf[pos] != ',' && buf[pos] != '}' && buf[pos] != ']' {
			pos++
		}
		return pos
	}
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return errors.New("short url is nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("url store closed")
	}

	if _, exists := s.records[u.Code]; exists && !overwrite {
		return model.ErrCodeExists
	}

	s.records[u.Code] = *u

	existing := s.records[u.Code]
	copy(s.sharedBuf[s.sharedUsed:], existing.Code)
	existing.Code = unsafe.String(&s.sharedBuf[s.sharedUsed], len(existing.Code))
	s.sharedUsed += len(existing.Code) + 1

	copy(s.sharedBuf[s.sharedUsed:], existing.RawURL)
	existing.RawURL = unsafe.String(&s.sharedBuf[s.sharedUsed], len(existing.RawURL))
	s.sharedUsed += len(existing.RawURL) + 1

	s.records[u.Code] = existing

	if s.cfg.Storage.FlushOn() {
		return s.flushLocked()
	}
	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, errors.New("url store closed")
	}
	r, ok := s.records[code]
	if !ok {
		return nil, model.ErrURLNotFound
	}
	out := r
	return &out, nil
}

func (s *URLStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]model.ShortURL, len(s.records))
	for k, v := range s.records {
		out[k] = v
	}
	return out
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	return nil
}

func (s *URLStore) flushLocked() error {
	path := s.cfg.Storage.URLFile()
	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	newRecords := make(map[string]model.ShortURL, len(s.records))
	for k, v := range s.records {
		newRecords[k] = model.ShortURL{
			Code:      v.Code,
			RawURL:    v.RawURL,
			CreatedAt: v.CreatedAt,
			Visits:    v.Visits,
			Custom:    v.Custom,
			Disabled:  v.Disabled,
			MaxVisits: v.MaxVisits,
		}
	}

	data, err := json.Marshal(newRecords)
	if err != nil {
		return err
	}

	if cap(s.readBuf) >= len(data) {
		s.readBuf = s.readBuf[:len(data)]
	} else {
		s.readBuf = make([]byte, len(data))
	}
	copy(s.readBuf, data)

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, s.readBuf, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	s.records = newRecords
	s.sharedUsed = 0

	return nil
}

type AccessLogStore struct {
	cfg     *config.Config
	mu      sync.Mutex
	opened  bool
	closed  bool
	file    *os.File
	records [][]byte
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	return &AccessLogStore{
		cfg: cfg,
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("access log store closed")
	}

	path := s.cfg.Storage.LogFile()
	if path == "" {
		s.opened = true
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	s.file = f
	s.opened = true
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	return nil
}

func (s *AccessLogStore) Append(ctx context.Context, entry []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return errors.New("access log store closed")
	}
	if !s.opened {
		return errors.New("access log not opened")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	s.records = append(s.records, entry)
	if s.file != nil {
		_, _ = s.file.Write(entry)
	}
	return nil
}

func WriteTimestamp() time.Time { return time.Now() }
