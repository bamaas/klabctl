package config

// ComponentSchema represents a component's schema definition
type ComponentSchema struct {
	ComponentName string                 `yaml:"componentName"`
	SchemaVersion string                 `yaml:"schemaVersion"`
	Values        map[string]ValueSchema `yaml:"values"`
}

// ValueSchema defines the schema for a single value field
type ValueSchema struct {
	Type        string      `yaml:"type"` // string, boolean, number, object
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default,omitempty"`
	Format      string      `yaml:"format,omitempty"` // ipv4, hostname, email
	Description string      `yaml:"description,omitempty"`
	Example     string      `yaml:"example,omitempty"`
}
