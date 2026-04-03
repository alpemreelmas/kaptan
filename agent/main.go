package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alpemreelmas/kaptan/agent/server"
	"github.com/alpemreelmas/kaptan/agent/updater"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr string `yaml:"listen_addr"`
	Update     struct {
		Interval string `yaml:"interval"`
	} `yaml:"update"`
	TLS struct {
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
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	interval := 1 * time.Hour
	if cfg.Update.Interval != "" {
		if d, err := time.ParseDuration(cfg.Update.Interval); err == nil {
			interval = d
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go updater.Start(ctx, interval)

	slog.Info("reis starting", "addr", cfg.ListenAddr)
	if err := server.Run(cfg.ListenAddr, cfg.TLS.Cert, cfg.TLS.Key, cfg.TLS.CA); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return home + "/.reis/config.yaml"
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
