// Package config provides configuration for the shurl short-link service.
package config

import "time"

// Config holds the configuration needed across the system.
type Config struct {
	Storage StorageConfig
}

// StorageConfig controls the persistent storage layer behavior.
type StorageConfig struct {
	urlFilePath  string
	logFilePath  string
	syncInterval time.Duration
	flushOnWrite bool
}

// URLFilePath sets the path where URL data is persisted.
func (s *StorageConfig) URLFilePath(p string) {
	s.urlFilePath = p
}

// LogFilePath sets the path where access logs are persisted.
func (s *StorageConfig) LogFilePath(p string) {
	s.logFilePath = p
}

// SyncInterval sets the interval between background syncs.
func (s *StorageConfig) SyncInterval(d time.Duration) {
	s.syncInterval = d
}

// FlushOnWrite controls whether each write flushes to disk immediately.
func (s *StorageConfig) FlushOnWrite(b bool) {
	s.flushOnWrite = b
}

// URLFilePath returns the configured URL data file path.
func (s *StorageConfig) GetURLFilePath() string { return s.urlFilePath }

// LogFilePath returns the configured access log file path.
func (s *StorageConfig) GetLogFilePath() string { return s.logFilePath }

// SyncInterval returns the configured sync interval.
func (s *StorageConfig) GetSyncInterval() time.Duration { return s.syncInterval }

// FlushOnWrite returns whether writes should be flushed immediately.
func (s *StorageConfig) GetFlushOnWrite() bool { return s.flushOnWrite }

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Storage: StorageConfig{
			urlFilePath:  "./shurl_data.json",
			logFilePath:  "./shurl_access.log",
			syncInterval: 30 * time.Second,
			flushOnWrite: true,
		},
	}
}
