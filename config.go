package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	AirplayName string `json:"airplayName"`
	Port        int    `json:"port"`
}

var appConfig Config

func defaultConfig() Config {
	return Config{
		AirplayName: "GoPlay",
		Port:        7000,
	}
}

func appDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func configPath() string {
	return filepath.Join(appDir(), "config.json")
}

func loadConfig() Config {
	cfg := defaultConfig()
	path := configPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("config: %s not found, creating default", path)
			saveConfig(cfg)
		} else {
			log.Printf("config: failed to read %s: %v (using defaults)", path, err)
		}
		return cfg
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("config: failed to parse %s: %v (using defaults)", path, err)
		return defaultConfig()
	}

	if cfg.AirplayName == "" {
		cfg.AirplayName = "GoPlay"
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		cfg.Port = 7000
	}

	log.Printf("config: loaded (AirPlay name=%q, port=%d)", cfg.AirplayName, cfg.Port)
	return cfg
}

func saveConfig(cfg Config) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("config: failed to serialize config: %v", err)
		return
	}
	if err := os.WriteFile(configPath(), data, 0o644); err != nil {
		log.Printf("config: failed to write %s: %v", configPath(), err)
	}
}

func setupFileLog() {
	path := filepath.Join(appDir(), "goplay.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.Ldate | log.Ltime)
}
