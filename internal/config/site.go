package config

import (
	"fmt"
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
	Stack Stack `yaml:"stack"`
	Infra Infra `yaml:"infra"`
	Apps  Apps  `yaml:"apps"`
}

// Stack defines the stack source configuration
type Stack struct {
	Source string `yaml:"source"`
	Ref    string `yaml:"ref"`
}

// Infra defines infrastructure configuration
type Infra struct {
	// Provider specifies which provider to use (e.g., "proxmox", "azure", "aws")
	Provider string `yaml:"provider"`

	// Providers contains all provider configurations
	// Each provider has its own complete configuration including cluster, nodeData, etc.
	Providers map[string]map[string]interface{} `yaml:"providers"`
}

// GetActiveProviderConfig returns the configuration for the active provider
func (i *Infra) GetActiveProviderConfig() (map[string]interface{}, error) {
	if i.Provider == "" {
		return nil, fmt.Errorf("no provider specified")
	}

	if i.Providers == nil {
		return nil, fmt.Errorf("no providers configured")
	}

	config, ok := i.Providers[i.Provider]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found in providers", i.Provider)
	}

	return config, nil
}

// NodeConfig defines configuration for a single node
type NodeConfig struct {
	IP            string `yaml:"ip" json:"ip"`
	Hostname      string `yaml:"hostname" json:"hostname"`
	PveNode       string `yaml:"pveNode" json:"pve_node"`
	PveId         int    `yaml:"pveId" json:"pve_id"`
	Memory        int    `yaml:"memory" json:"memory"`
	Cores         int    `yaml:"cores" json:"cores"`
	DiskSize      int    `yaml:"diskSize" json:"disk_size"`
	InstallDisk   string `yaml:"installDisk,omitempty" json:"install_disk,omitempty"`
	StartOnBoot   bool   `yaml:"startOnBoot,omitempty" json:"start_on_boot,omitempty"`
	NetworkBridge string `yaml:"networkBridge,omitempty" json:"network_bridge,omitempty"`
	DatastoreId   string `yaml:"datastoreId,omitempty" json:"datastore_id,omitempty"`
}

// Apps defines application configuration
type Apps struct {
	Stack   Stack                `yaml:"stack,omitempty"`
	Catalog map[string]Component `yaml:"catalog"`
}

// Base defines the base application configuration
type Base struct {
	Source string `yaml:"source"`
	Ref    string `yaml:"ref"`
	Path   string `yaml:"path"`
}

// Component defines a component configuration
type Component struct {
	Enabled   bool                   `yaml:"enabled"`
	Project   string                 `yaml:"project"`
	Namespace string                 `yaml:"namespace"`
	Values    map[string]interface{} `yaml:"values"`
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
