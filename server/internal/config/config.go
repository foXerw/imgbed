package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen       string   `yaml:"listen"`
	StorageDir   string   `yaml:"storageDir"`
	BaseURL      string   `yaml:"baseURL"`
	Token        string   `yaml:"token"`
	MaxDimension int      `yaml:"maxDimension"`
	MaxSizeMB    int64    `yaml:"maxSizeMB"`
	ConvertWebp  bool     `yaml:"convertWebp"`
	AllowedTypes []string `yaml:"allowedTypes"`
}

func Default() *Config {
	return &Config{
		Listen:       ":8080",
		StorageDir:   "./storage",
		BaseURL:      "http://localhost:8080",
		Token:        "change-me",
		MaxDimension: 2560,
		MaxSizeMB:    5,
		ConvertWebp:  false,
		AllowedTypes: []string{"jpeg", "png", "gif", "webp", "svg"},
	}
}

// Load 读取 YAML 配置并叠加在默认值之上。
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
