package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Port         int      `mapstructure:"port"`
	Backends     []string `mapstructure:"backends"`
	RateLimiting struct {
		Enabled  bool   `mapstructure:"enabled"`
		Type     string `mapstructure:"type"`
		Postgres struct {
			Host     string `mapstructure:"host"`
			Port     int    `mapstructure:"port"`
			User     string `mapstructure:"user"`
			Password string `mapstructure:"password"`
			DBName   string `mapstructure:"dbname"`
		} `mapstructure:"postgres"`
		Default struct {
			Capacity int64   `mapstructure:"capacity"`
			Rate     float64 `mapstructure:"rate"`
		} `mapstructure:"default"`
	} `mapstructure:"rate_limiting"`
	Balancing struct {
		Algorithm string `mapstructure:"algorithm"`
	} `mapstructure:"balancing"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetDefault("port", 8080)
	v.SetDefault("rate_limiting.enabled", false)
	v.SetDefault("rate_limiting.type", "inmemory")

	v.SetConfigFile(configPath)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid port number: %d", cfg.Port)
	}

	if len(cfg.Backends) == 0 {
		return nil, fmt.Errorf("no backends specified")
	}

	return &cfg, nil
}

func GetConfigPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return filepath.Join("configs", "config.yaml")
}
