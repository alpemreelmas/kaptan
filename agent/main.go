package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/kaptan/agent/server"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr string `yaml:"listen_addr"`
	TLS        struct {
		Cert string `yaml:"cert"`
		Key  string `yaml:"key"`
		CA   string `yaml:"ca"`
	} `yaml:"tls"`
}

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("kaptan-agent starting on %s", cfg.ListenAddr)
	if err := server.Run(cfg.ListenAddr, cfg.TLS.Cert, cfg.TLS.Key, cfg.TLS.CA); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.kaptan-agent/config.yaml"
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// return defaults if no config file
		if os.IsNotExist(err) {
			return &Config{
				ListenAddr: ":7000",
			}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":7000"
	}
	return &cfg, nil
}
