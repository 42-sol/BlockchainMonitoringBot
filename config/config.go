package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Bot    BotConfig    `yaml:"bot"`
	Logger LoggerConfig `yaml:"logger"`
}

type BotConfig struct {
	ScanIntervalMinute int `yaml:"scan_interval_minute"`
}

type LoggerConfig struct {
	MaxLogs int `yaml:"max_logs"`
}

var AppConfig Config

func Load() {
	f, _ := os.Open("config.yml")
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	decoder.Decode(&AppConfig)
}
