package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigFile = "config.yaml"
	DefaultCSVFile    = "devices.csv"
	DefaultReportFile = "tacacs_report.csv"
)

// Creds holds connection credentials and AAA test creds
type Creds struct {
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Port        int    `yaml:"port"`
	Debug       bool   `yaml:"debug"`
	Timeout     int    `yaml:"timeout"`
	Retries     int    `yaml:"retries"`
	AAAUsername string `yaml:"aaa_username"`
	AAAPassword string `yaml:"aaa_password"`
}

// Files holds file path configuration
type Files struct {
	CSVFile    string `yaml:"csv_file"`
	ReportFile string `yaml:"report_file"`
}

// Config represents the full config.yaml structure
type Config struct {
	Creds        Creds                  `yaml:"creds"`
	Files        Files                  `yaml:"files"`
	TacacsConfig map[string]interface{} `yaml:"tacacs_config"`
}

// LoadConfig loads and parses the YAML configuration file
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigFile
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file not found: %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Apply defaults if unset
	if cfg.Files.CSVFile == "" {
		cfg.Files.CSVFile = DefaultCSVFile
	}
	if cfg.Files.ReportFile == "" {
		cfg.Files.ReportFile = DefaultReportFile
	}

	return &cfg, nil
}
