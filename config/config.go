package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ScanInterval int `yaml:"scan_interval_minute"`
}

var AppConfig Config

func Load() {
	f, _ := os.Open("config.yml")
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	decoder.Decode(&AppConfig)
}
