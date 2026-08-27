package config

import (
	"time"
)

type Storage struct {
	urlFilePath  string
	logFilePath  string
	syncInterval time.Duration
	flushOnWrite bool
}

func (s *Storage) URLFilePath(path string) {
	s.urlFilePath = path
}

func (s *Storage) LogFilePath(path string) {
	s.logFilePath = path
}

func (s *Storage) SyncInterval(d time.Duration) {
	s.syncInterval = d
}

func (s *Storage) FlushOnWrite(b bool) {
	s.flushOnWrite = b
}

func (s *Storage) GetURLFilePath() string {
	return s.urlFilePath
}

func (s *Storage) GetLogFilePath() string {
	return s.logFilePath
}

func (s *Storage) GetSyncInterval() time.Duration {
	return s.syncInterval
}

func (s *Storage) GetFlushOnWrite() bool {
	return s.flushOnWrite
}

func Default() *Config {
	cfg := DefaultConfig()
	cfg.Storage = Storage{
		urlFilePath:  "data/urls.json",
		logFilePath:  "data/access.log",
		syncInterval: 5 * time.Second,
		flushOnWrite: true,
	}
	return cfg
}
