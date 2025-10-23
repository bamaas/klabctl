package templates

import (
	"embed"
	"fmt"
)

//go:embed apps/*.tmpl apps/*/*.tmpl apps/*/*.yaml infra/*.tmpl site.yaml.tmpl
var EmbeddedTemplates embed.FS

// TemplateNames provides a map of template names to their embedded content
var TemplateNames = map[string]string{
	"apps/kustomization.yaml.tmpl":        "apps/kustomization.yaml.tmpl",
	"apps/pihole/kustomization.yaml.tmpl": "apps/pihole/kustomization.yaml.tmpl",
}

// GetTemplate returns the content of a specific template
func GetTemplate(templateName string) (string, error) {
	content, err := EmbeddedTemplates.ReadFile(templateName)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templateName, err)
	}
	return string(content), nil
}

// TemplateExists checks if a template exists in the embedded filesystem
func TemplateExists(templateName string) bool {
	_, err := EmbeddedTemplates.ReadFile(templateName)
	return err == nil
}
