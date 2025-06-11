package config

import (
	"fmt"
	"time"
)

type Config struct {
	Redis  RedisConfig  `yaml:"redis"`
	Server ServerConfig `yaml:"server"`
}

type RedisConfig struct {
	URLs       []string      `yaml:"urls"`
	PoolSize   int           `yaml:"pool_size"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
}

type ServerConfig struct {
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

func Default() *Config {
	return &Config{
		Redis: RedisConfig{
			URLs:       []string{"redis://localhost:6379"},
			PoolSize:   10,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
		},
		Server: ServerConfig{
			Port:    8080,
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Config) Validate() error {
	if len(c.Redis.URLs) == 0 {
		return fmt.Errorf("redis URLs cannot be empty")
	}

	if c.Redis.PoolSize <= 0 {
		return fmt.Errorf("redis pool size must be positive")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	return nil
}
