package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/bamaas/klabctl/internal/templates"
	"github.com/spf13/cobra"
)

// TemplateData holds the data used for templating
type TemplateData struct {
	Site          *config.Site
	Versions      map[string]string
	Component     *config.Component
	ComponentName string
	AllComponents map[string]config.Component
}

// RenderComponentKustomizationTemplate renders the kustomization.yaml.tmpl template for a specific component from embedded files
func RenderKustomizationTemplate(site *config.Site, componentName string, component *config.Component, templateName, outputPath string) error {

	// Create template with custom functions
	funcMap := template.FuncMap{
		"quote": func(s string) string {
			return fmt.Sprintf(`"%s"`, s)
		},
	}

	// Read base template first
	baseTemplatePath := "apps/base.kustomization.yaml.tmpl"
	baseContent, err := templates.EmbeddedTemplates.ReadFile(baseTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read base template %s: %w", baseTemplatePath, err)
	}

	// Read component-specific template
	templateContent, err := templates.EmbeddedTemplates.ReadFile(templateName)
	if err != nil {
		return fmt.Errorf("failed to read embedded template %s: %w", templateName, err)
	}

	// Parse both templates together so "base" template is available
	tmpl, err := template.New("base").Funcs(funcMap).Parse(string(baseContent))
	if err != nil {
		return fmt.Errorf("failed to parse base template: %w", err)
	}

	// If using a component-specific template, parse it too
	if templateName != baseTemplatePath {
		tmpl, err = tmpl.New(templateName).Parse(string(templateContent))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}
	}

	data := TemplateData{
		Site:          site,
		Component:     component,
		ComponentName: componentName,
		AllComponents: site.Spec.Apps.Catalog,
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	// Execute the appropriate template
	var executeTemplate *template.Template
	if templateName != baseTemplatePath {
		executeTemplate = tmpl.Lookup(templateName)
	} else {
		executeTemplate = tmpl.Lookup("base")
	}

	if executeTemplate == nil {
		return fmt.Errorf("template not found: %s", templateName)
	}

	if err := executeTemplate.Execute(outputFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// RenderTemplate renders any template to a file using embedded templates
func RenderTemplate(site *config.Site, componentName string, component *config.Component, templateName, outputPath string) error {
	// Create template with custom functions
	funcMap := template.FuncMap{
		"quote": func(s string) string {
			return fmt.Sprintf(`"%s"`, s)
		},
	}

	// Read base template first (for inheritance)
	baseTemplatePath := "apps/base.kustomization.yaml.tmpl"
	baseContent, err := templates.EmbeddedTemplates.ReadFile(baseTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read base template %s: %w", baseTemplatePath, err)
	}

	// Read component-specific template
	templateContent, err := templates.EmbeddedTemplates.ReadFile(templateName)
	if err != nil {
		return fmt.Errorf("failed to read embedded template %s: %w", templateName, err)
	}

	// Parse both templates together so "base" template is available
	tmpl, err := template.New("base").Funcs(funcMap).Parse(string(baseContent))
	if err != nil {
		return fmt.Errorf("failed to parse base template: %w", err)
	}

	// If using a component-specific template, parse it too
	if templateName != baseTemplatePath {
		tmpl, err = tmpl.New(templateName).Parse(string(templateContent))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templateName, err)
		}
	}

	data := TemplateData{
		Site:          site,
		Component:     component,
		ComponentName: componentName,
		AllComponents: site.Spec.Apps.Catalog,
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	// Execute the appropriate template
	var executeTemplate *template.Template
	if templateName != baseTemplatePath {
		executeTemplate = tmpl.Lookup(templateName)
	} else {
		executeTemplate = tmpl.Lookup("base")
	}

	if executeTemplate == nil {
		return fmt.Errorf("template not found: %s", templateName)
	}

	if err := executeTemplate.Execute(outputFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// FindComponentTemplates finds all templates for a specific component
func FindAppTemplates(componentName string) ([]string, error) {
	var componentTemplates []string

	// Check for component-specific templates
	componentDir := fmt.Sprintf("apps/%s/", componentName)

	// Walk through embedded filesystem to find templates for this component
	err := fs.WalkDir(templates.EmbeddedTemplates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Check if this template belongs to the component
		if strings.HasPrefix(path, componentDir) && strings.HasSuffix(path, ".tmpl") {
			componentTemplates = append(componentTemplates, path)
		}
		return nil
	})

	return componentTemplates, err
}

func newRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render cluster GitOps skeleton from site.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := config.LoadSiteFromFile(sitePath)
			if err != nil {
				return err
			}

			// Define path to components directory
			appsPath := filepath.Join("clusters", site.Metadata.Name, "apps")

			// Create components directory if it doesn't exist
			if err := os.MkdirAll(appsPath, 0755); err != nil {
				return fmt.Errorf("failed to create apps directory: %w", err)
			}

			// Render all templates for each component
			renderedCount := 0
			for componentName, component := range site.Spec.Apps.Catalog {
				if !component.Enabled {
					continue // Skip disabled components
				}

				// Create component directory
				componentPath := filepath.Join(appsPath, componentName)
				if err := os.MkdirAll(componentPath, 0755); err != nil {
					return fmt.Errorf("failed to create component directory for %s: %w", componentName, err)
				}

				// Find all templates for this component
				componentTemplates, err := FindAppTemplates(componentName)
				if err != nil {
					return fmt.Errorf("failed to find templates for component %s: %w", componentName, err)
				}

				// If no app specific templates found, use base template
				if len(componentTemplates) == 0 {
					templateName := "apps/base.kustomization.yaml.tmpl"
					outputPath := filepath.Join(componentPath, "kustomization.yaml")
					if err := RenderKustomizationTemplate(site, componentName, &component, templateName, outputPath); err != nil {
						return fmt.Errorf("failed to render base template for component %s: %w", componentName, err)
					}
					renderedCount++
					continue
				}

				// Render all app specific templates
				for _, templateName := range componentTemplates {
					// Convert template name to output filename
					// e.g., "apps/pihole/kustomization.yaml.tmpl" -> "kustomization.yaml"
					// e.g., "apps/pihole/secret-patch.yaml.tmpl" -> "secret-patch.yaml"
					baseName := filepath.Base(templateName)
					outputFileName := strings.TrimSuffix(baseName, ".tmpl")
					outputPath := filepath.Join(componentPath, outputFileName)

					if err := RenderTemplate(site, componentName, &component, templateName, outputPath); err != nil {
						return fmt.Errorf("failed to render template %s for component %s: %w", templateName, componentName, err)
					}
					renderedCount++
				}
			}

			fmt.Printf("Rendered %d component kustomization files\n", renderedCount)
			return nil
		},
	}
	return cmd
}
