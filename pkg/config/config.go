package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// RegexRule maps a compiled-at-runtime regex pattern to an AWS region.
type RegexRule struct {
	Pattern string `yaml:"pattern"`
	Region  string `yaml:"region"`
}

// AwsConfig holds user-defined extensions for AWS region resolution.
type AwsConfig struct {
	// Extra key→region pairs merged on top of the built-in ISO map.
	// Example:
	//   extra_regions:
	//     PROD: eu-central-1
	//     STG:  eu-west-1
	ExtraRegions map[string]string `yaml:"extra_regions"`

	// Regex rules evaluated in order when no key match is found.
	// The first matching pattern wins.
	// Example:
	//   region_rules:
	//     - pattern: "(?i)prod.*de"
	//       region: eu-central-1
	//     - pattern: "(?i).*milan.*"
	//       region: eu-south-1
	RegionRules []RegexRule `yaml:"region_rules"`
}

// Config is the top-level heimdall configuration.
type Config struct {
	Aws AwsConfig `yaml:"aws"`
}

var (
	once     sync.Once
	instance *Config
)

// Load reads ~/.config/heimdall.yaml once and caches the result.
// If the file does not exist an empty Config is returned without error.
func Load() *Config {
	once.Do(func() {
		instance = &Config{}

		path, err := configPath()
		if err != nil {
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			// File not found is fine — user just hasn't created one yet.
			return
		}

		_ = yaml.Unmarshal(data, instance)
	})
	return instance
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "heimdall.yaml"), nil
}
