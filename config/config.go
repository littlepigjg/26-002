package config

import "time"

type Config struct {
	Storage StorageConfig
}

type StorageConfig struct {
	urlFilePath  string
	logFilePath  string
	syncInterval time.Duration
	flushOnWrite bool
	cacheMaxSize int
}

func (s *StorageConfig) URLFilePath(path string) {
	s.urlFilePath = path
}

func (s *StorageConfig) LogFilePath(path string) {
	s.logFilePath = path
}

func (s *StorageConfig) SyncInterval(d time.Duration) {
	s.syncInterval = d
}

func (s *StorageConfig) FlushOnWrite(b bool) {
	s.flushOnWrite = b
}

func (s *StorageConfig) CacheMaxSize(size int) {
	s.cacheMaxSize = size
}

func (s *StorageConfig) GetURLFilePath() string {
	return s.urlFilePath
}

func (s *StorageConfig) GetLogFilePath() string {
	return s.logFilePath
}

func (s *StorageConfig) GetSyncInterval() time.Duration {
	return s.syncInterval
}

func (s *StorageConfig) GetFlushOnWrite() bool {
	return s.flushOnWrite
}

func (s *StorageConfig) GetCacheMaxSize() int {
	return s.cacheMaxSize
}

func Default() *Config {
	return &Config{
		Storage: StorageConfig{
			urlFilePath:  "./data/urls.json",
			logFilePath:  "./data/access.log",
			syncInterval: 5 * time.Second,
			flushOnWrite: true,
			cacheMaxSize: 1000,
		},
	}
}
