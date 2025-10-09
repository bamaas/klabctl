package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Site represents the site configuration structure
type Site struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

// Metadata contains basic metadata about the site
type Metadata struct {
	Name string `yaml:"name"`
}

// Spec contains the main configuration specification
type Spec struct {
	Apps Apps `yaml:"apps"`
}

// Apps defines application configuration
type Apps struct {
	ModuleSource string   `yaml:"moduleSource"`
	ModuleRef    string   `yaml:"moduleRef"`
	ModulePath   string   `yaml:"modulePath"`
	Components   []string `yaml:"components,omitempty"`
}

// ParseSite parses a YAML byte slice into a Site struct
func ParseSite(data []byte) (*Site, error) {
	var site Site
	if err := yaml.Unmarshal(data, &site); err != nil {
		return nil, fmt.Errorf("failed to parse site YAML: %w", err)
	}

	// TODO: validate site

	return &site, nil
}

// LoadSiteFromFile loads and parses a site configuration from a file
func LoadSiteFromFile(filename string) (*Site, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	return ParseSite(data)
}

// LoadSiteFromReader loads and parses a site configuration from an io.Reader
func LoadSiteFromReader(reader io.Reader) (*Site, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}

	return ParseSite(data)
}

// ToYAML converts a Site struct back to YAML format
func (s *Site) ToYAML() ([]byte, error) {
	return yaml.Marshal(s)
}
