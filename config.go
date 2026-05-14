package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all configuration for the demo server.
type Config struct {
	// CloudFront distribution domain, e.g. "d1234abcd.cloudfront.net"
	DistributionDomain string `json:"distribution_domain"`

	// CloudFront Key Group key pair ID (from AWS console)
	KeyPairID string `json:"key_pair_id"`

	// Path to the CloudFront private key PEM file
	PrivateKeyPath string `json:"private_key_path"`

	// Default TTL in seconds for signed URLs (default: 3600)
	DefaultTTLSeconds int `json:"default_ttl_seconds"`

	// Server listen address (default: ":8080")
	ListenAddr string `json:"listen_addr"`
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Apply defaults
	if cfg.DefaultTTLSeconds <= 0 {
		cfg.DefaultTTLSeconds = 3600
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}

	// Validate required fields
	if cfg.DistributionDomain == "" {
		return nil, fmt.Errorf("distribution_domain is required")
	}
	if cfg.KeyPairID == "" {
		return nil, fmt.Errorf("key_pair_id is required")
	}
	if cfg.PrivateKeyPath == "" {
		return nil, fmt.Errorf("private_key_path is required")
	}

	return &cfg, nil
}
