package config

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Home       string        `toml:"-"`
	Path       string        `toml:"-"`
	DBPath     string        `toml:"db_path"`
	RawDir     string        `toml:"raw_dir"`
	ReportsDir string        `toml:"reports_dir"`
	LogDir     string        `toml:"log_dir"`
	Mode       string        `toml:"mode"`
	Cache      CacheConfig   `toml:"cache"`
	Sources    SourcesConfig `toml:"sources"`
	Reports    ReportConfig  `toml:"reports"`
	MCP        MCPConfig     `toml:"mcp"`
	Privacy    PrivacyConfig `toml:"privacy"`
}

type CacheConfig struct {
	MaxAge string `toml:"max_age"`
}

type SourcesConfig struct {
	FMCSA   SourceEnabledConfig `toml:"fmcsa"`
	Socrata SourceEnabledConfig `toml:"socrata"`
	Mirror  MirrorConfig        `toml:"mirror"`
}

type SourceEnabledConfig struct {
	Enabled bool `toml:"enabled"`
}

type MirrorConfig struct {
	Enabled     bool   `toml:"enabled"`
	URL         string `toml:"url"`
	ChecksumURL string `toml:"checksum_url"`
}

type ReportConfig struct {
	DefaultFormat string `toml:"default_format"`
}

type MCPConfig struct {
	Transport string `toml:"transport"`
	Host      string `toml:"host"`
	Port      int    `toml:"port"`
}

type PrivacyConfig struct {
	Telemetry bool `toml:"telemetry"`
}

type Overrides struct {
	Home       string
	ConfigPath string
	DBPath     string
}

func Default(overrides Overrides) (Config, error) {
	home := overrides.Home
	if home == "" {
		home = os.Getenv("OHG_HOME")
	}
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return Config{}, err
		}
		home = filepath.Join(userHome, ".openhaulguard")
	}
	cfgPath := overrides.ConfigPath
	if cfgPath == "" {
		cfgPath = os.Getenv("OHG_CONFIG")
	}
	if cfgPath == "" {
		cfgPath = filepath.Join(home, "config.toml")
	}
	dbPath := overrides.DBPath
	if dbPath == "" {
		dbPath = os.Getenv("OHG_DB_PATH")
	}
	if dbPath == "" {
		dbPath = filepath.Join(home, "ohg.db")
	}
	return Config{
		Home:       home,
		Path:       cfgPath,
		DBPath:     dbPath,
		RawDir:     filepath.Join(home, "raw"),
		ReportsDir: filepath.Join(home, "reports"),
		LogDir:     filepath.Join(home, "logs"),
		Mode:       envOr("OHG_MODE", "local"),
		Cache:      CacheConfig{MaxAge: "24h"},
		Sources: SourcesConfig{
			FMCSA:   SourceEnabledConfig{Enabled: true},
			Socrata: SourceEnabledConfig{Enabled: true},
			Mirror: MirrorConfig{
				Enabled:     true,
				URL:         "https://downloads.openhaulguard.org/bootstrap/mc_dot_index.parquet",
				ChecksumURL: "https://downloads.openhaulguard.org/bootstrap/mc_dot_index.sha256",
			},
		},
		Reports: ReportConfig{DefaultFormat: "table"},
		MCP:     MCPConfig{Transport: "stdio", Host: "127.0.0.1", Port: 8798},
		Privacy: PrivacyConfig{Telemetry: false},
	}, nil
}

func Load(overrides Overrides) (Config, error) {
	cfg, err := Default(overrides)
	if err != nil {
		return Config{}, err
	}
	if _, err := os.Stat(cfg.Path); err == nil {
		if _, err := toml.DecodeFile(cfg.Path, &cfg); err != nil {
			return Config{}, err
		}
		cfg.Home, _ = filepath.Abs(cfg.Home)
		if cfg.Home == "" {
			cfg.Home = overrides.Home
		}
		if cfg.Home == "" {
			cfg.Home = os.Getenv("OHG_HOME")
		}
		if cfg.Home == "" {
			userHome, _ := os.UserHomeDir()
			cfg.Home = filepath.Join(userHome, ".openhaulguard")
		}
		cfg.Path = firstNonEmpty(overrides.ConfigPath, os.Getenv("OHG_CONFIG"), cfg.Path)
		cfg.DBPath = firstNonEmpty(overrides.DBPath, os.Getenv("OHG_DB_PATH"), cfg.DBPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	cfg.Mode = envOr("OHG_MODE", cfg.Mode)
	if cfg.Cache.MaxAge == "" {
		cfg.Cache.MaxAge = "24h"
	}
	if cfg.Reports.DefaultFormat == "" {
		cfg.Reports.DefaultFormat = "table"
	}
	if cfg.MCP.Transport == "" {
		cfg.MCP.Transport = "stdio"
	}
	if cfg.MCP.Host == "" {
		cfg.MCP.Host = "127.0.0.1"
	}
	if cfg.MCP.Port == 0 {
		cfg.MCP.Port = 8798
	}
	if cfg.RawDir == "" {
		cfg.RawDir = filepath.Join(cfg.Home, "raw")
	}
	if cfg.ReportsDir == "" {
		cfg.ReportsDir = filepath.Join(cfg.Home, "reports")
	}
	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join(cfg.Home, "logs")
	}
	return cfg, nil
}

func (c Config) EnsureDirs() error {
	for _, path := range []string{c.Home, filepath.Dir(c.Path), filepath.Dir(c.DBPath), c.RawDir, c.ReportsDir, c.LogDir} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) Save() error {
	if err := c.EnsureDirs(); err != nil {
		return err
	}
	f, err := os.OpenFile(c.Path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

func (c Config) MaxAgeDuration() time.Duration {
	d, err := time.ParseDuration(c.Cache.MaxAge)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
