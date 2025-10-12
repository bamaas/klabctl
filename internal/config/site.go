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
	Infra Infra `yaml:"infra"`
	Apps  Apps  `yaml:"apps"`
}

// Infra defines infrastructure configuration
type Infra struct {
	Base Base `yaml:"base"`

	Provider struct {
		Name    string `yaml:"name"`
		Proxmox struct {
			Endpoint string `yaml:"endpoint"`
			TokenID  string `yaml:"tokenID"`
		} `yaml:"proxmox"`
	} `yaml:"provider"`

	TalosImage struct {
		URL         string `yaml:"url"`
		FileName    string `yaml:"fileName"`
		NodeName    string `yaml:"nodeName"`
		DatastoreId string `yaml:"datastoreId"`
		Overwrite   bool   `yaml:"overwrite"`
		ContentType string `yaml:"contentType"`
	} `yaml:"talosImage"`

	NodeData struct {
		ControlPlanes []NodeConfig `yaml:"controlPlanes"`
		Workers       []NodeConfig `yaml:"workers"`
	} `yaml:"nodeData"`

	Cluster struct {
		Name            string `yaml:"name"`
		Endpoint        string `yaml:"endpoint"`
		VirtualSharedIp string `yaml:"virtualSharedIp"`
		Domain          string `yaml:"domain"`
		DefaultGateway  string `yaml:"defaultGateway"`
	} `yaml:"cluster"`
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
	Base    Base                 `yaml:"base"`
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
	Enabled bool                   `yaml:"enabled"`
	Values  map[string]interface{} `yaml:"values"`
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
