// Package config provides application configuration management.
// It supports loading configuration from environment variables and
// provides sensible defaults for all settings.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds all application configuration.
type Config struct {
	mu *sync.RWMutex

	// Server configuration
	Server ServerConfig `json:"server"`
	// Database/storage configuration
	Store StoreConfig `json:"store"`
	// Session configuration
	Session SessionConfig `json:"session"`
	// Analytics configuration
	Analytics AnalyticsConfig `json:"analytics"`
	// Logging configuration
	Logging LoggingConfig `json:"logging"`
	// Export configuration
	Export ExportConfig `json:"export"`
	// Rate limiting configuration
	RateLimit RateLimitConfig `json:"rate_limit"`
	// Storage configuration
	Storage StorageConfig `json:"storage"`
}

// ServerConfig holds server-specific settings.
type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ReadTimeout  int    `json:"read_timeout_seconds"`
	WriteTimeout int    `json:"write_timeout_seconds"`
	IdleTimeout  int    `json:"idle_timeout_seconds"`
	ShutdownTimeout int  `json:"shutdown_timeout_seconds"`
}

// Addr returns the server address string.
func (s *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// StoreConfig holds storage layer settings.
type StoreConfig struct {
	MaxEventsPerUser   int           `json:"max_events_per_user"`
	MaxSessionsPerUser int           `json:"max_sessions_per_user"`
	MaxPathLength      int           `json:"max_path_length"`
	FlushInterval      int           `json:"flush_interval_seconds"`
	EvictionInterval   int           `json:"eviction_interval_seconds"`
}

// SessionConfig holds session-related settings.
type SessionConfig struct {
	TimeoutMinutes     int `json:"timeout_minutes"`
	MaxIdleMinutes     int `json:"max_idle_minutes"`
	MinEventsForSession int `json:"min_events_for_session"`
}

// Timeout returns the session timeout duration.
func (sc *SessionConfig) Timeout() time.Duration {
	return time.Duration(sc.TimeoutMinutes) * time.Minute
}

// AnalyticsConfig holds analytics-related settings.
type AnalyticsConfig struct {
	DefaultTimeRangeHours int `json:"default_time_range_hours"`
	MaxTimeRangeHours     int `json:"max_time_range_hours"`
	HotPathLimit          int `json:"hot_path_limit"`
	ConversionCacheSeconds int `json:"conversion_cache_seconds"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level   string `json:"level"`
	Format  string `json:"format"`
	Output  string `json:"output"`
	File    string `json:"file_path"`
	MaxSize int    `json:"max_size_mb"`
}

// ExportConfig holds export-related settings.
type ExportConfig struct {
	MaxRecords    int  `json:"max_records"`
	TimeoutSecond int  `json:"timeout_seconds"`
	Compress      bool `json:"compress"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Enabled    bool  `json:"enabled"`
	MaxPerSec  int   `json:"max_per_second"`
	MaxBurst   int   `json:"max_burst"`
	WindowSec  int   `json:"window_seconds"`
}

// StorageConfig holds storage-layer settings (paths for persistent files).
type StorageConfig struct {
	URLPath          string `json:"url_path"`
	LogPath          string `json:"log_path"`
	SyncIntervalSec  int    `json:"sync_interval_sec"`
	FlushOnWriteFlag bool   `json:"flush_on_write"`
}

// URLFilePath returns the file path used for persisting short URLs.
func (s StorageConfig) URLFilePath(path string) StorageConfig {
	s.URLPath = path
	return s
}

// LogFilePath returns the file path used for persisting access logs.
func (s StorageConfig) LogFilePath(path string) StorageConfig {
	s.LogPath = path
	return s
}

// SyncInterval sets the sync interval in seconds.
func (s StorageConfig) SyncInterval(d time.Duration) StorageConfig {
	s.SyncIntervalSec = int(d.Seconds())
	return s
}

// FlushOnWrite sets whether to flush on each write.
func (s StorageConfig) FlushOnWrite(b bool) StorageConfig {
	s.FlushOnWriteFlag = b
	return s
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		mu: &sync.RWMutex{},
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30,
			WriteTimeout:    30,
			IdleTimeout:     120,
			ShutdownTimeout: 30,
		},
		Store: StoreConfig{
			MaxEventsPerUser:   100000,
			MaxSessionsPerUser: 1000,
			MaxPathLength:      100,
			FlushInterval:      5,
			EvictionInterval:   3600,
		},
		Session: SessionConfig{
			TimeoutMinutes:      30,
			MaxIdleMinutes:      60,
			MinEventsForSession: 1,
		},
		Analytics: AnalyticsConfig{
			DefaultTimeRangeHours: 24,
			MaxTimeRangeHours:     720,
			HotPathLimit:          20,
			ConversionCacheSeconds: 300,
		},
		Logging: LoggingConfig{
			Level:   "INFO",
			Format:  "text",
			Output:  "stdout",
			File:    "",
			MaxSize: 100,
		},
		Export: ExportConfig{
			MaxRecords:    100000,
			TimeoutSecond: 60,
			Compress:      false,
		},
		RateLimit: RateLimitConfig{
			Enabled:   false,
			MaxPerSec: 100,
			MaxBurst:  200,
			WindowSec: 60,
		},
		Storage: StorageConfig{
			URLPath:          "./data/urls.json",
			LogPath:          "./data/access.log",
			SyncIntervalSec:  5,
			FlushOnWriteFlag: false,
		},
	}
}

// Load loads configuration from environment variables, falling back to defaults.
func Load() *Config {
	cfg := DefaultConfig()

	// Server config
	if v := os.Getenv("SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("SERVER_READ_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.ReadTimeout = n
		}
	}
	if v := os.Getenv("SERVER_WRITE_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.WriteTimeout = n
		}
	}

	// Store config
	if v := os.Getenv("STORE_MAX_EVENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Store.MaxEventsPerUser = n
		}
	}
	if v := os.Getenv("STORE_FLUSH_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Store.FlushInterval = n
		}
	}

	// Session config
	if v := os.Getenv("SESSION_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Session.TimeoutMinutes = n
		}
	}

	// Analytics config
	if v := os.Getenv("ANALYTICS_DEFAULT_RANGE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Analytics.DefaultTimeRangeHours = n
		}
	}
	if v := os.Getenv("ANALYTICS_HOT_PATH_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Analytics.HotPathLimit = n
		}
	}

	// Logging config
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = strings.ToUpper(v)
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}
	if v := os.Getenv("LOG_FILE"); v != "" {
		cfg.Logging.File = v
		cfg.Logging.Output = "file"
	}

	return cfg
}

// Get returns the current configuration (thread-safe).
func (c *Config) Get() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := Config{
		Server:    c.Server,
		Store:     c.Store,
		Session:   c.Session,
		Analytics: c.Analytics,
		Logging:   c.Logging,
		Export:    c.Export,
		RateLimit: c.RateLimit,
		Storage:   c.Storage,
	}
	return result
}

// Update temporarily updates a configuration value (thread-safe).
func (c *Config) Update(fn func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Session.TimeoutMinutes <= 0 {
		return fmt.Errorf("session timeout must be positive")
	}
	if c.Store.MaxEventsPerUser <= 0 {
		return fmt.Errorf("max events per user must be positive")
	}
	if c.Analytics.DefaultTimeRangeHours <= 0 {
		return fmt.Errorf("default time range must be positive")
	}
	return nil
}

// ToJSON serializes the config to JSON.
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// String returns a string representation of the config.
func (c *Config) String() string {
	data, err := c.ToJSON()
	if err != nil {
		return fmt.Sprintf("Config{error: %v}", err)
	}
	return string(data)
}

// Default returns the default configuration.
func Default() *Config {
	return DefaultConfig()
}
