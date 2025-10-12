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

	// Read header template first
	headerContent, err := templates.EmbeddedTemplates.ReadFile("apps/header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header template: %w", err)
	}

	// Read base template
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

	// Parse all templates together (header, base, and component-specific)
	tmpl, err := template.New("header").Funcs(funcMap).Parse(string(headerContent))
	if err != nil {
		return fmt.Errorf("failed to parse header template: %w", err)
	}

	tmpl, err = tmpl.New("base").Parse(string(baseContent))
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

	// Read header template first
	headerContent, err := templates.EmbeddedTemplates.ReadFile("apps/header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header template: %w", err)
	}

	// Read base template (for inheritance)
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

	// Parse all templates together (header, base, and component-specific)
	tmpl, err := template.New("header").Funcs(funcMap).Parse(string(headerContent))
	if err != nil {
		return fmt.Errorf("failed to parse header template: %w", err)
	}

	tmpl, err = tmpl.New("base").Parse(string(baseContent))
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

// createRootKustomization creates the root kustomization.yaml that references generated + custom
func createRootKustomization(site *config.Site, componentName, outputPath string) error {
	// Read header template first
	headerContent, err := templates.EmbeddedTemplates.ReadFile("apps/header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header kustomization template: %w", err)
	}

	// Read root template
	templateContent, err := templates.EmbeddedTemplates.ReadFile("apps/root.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read root kustomization template: %w", err)
	}

	// Parse both templates together
	tmpl, err := template.New("header").Parse(string(headerContent))
	if err != nil {
		return fmt.Errorf("failed to parse header template: %w", err)
	}

	tmpl, err = tmpl.New("root-kustomization").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse root kustomization template: %w", err)
	}

	data := struct {
		Site          *config.Site
		ComponentName string
	}{
		Site:          site,
		ComponentName: componentName,
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create root kustomization file: %w", err)
	}
	defer outputFile.Close()

	if err := tmpl.ExecuteTemplate(outputFile, "root-kustomization", data); err != nil {
		return fmt.Errorf("failed to execute root kustomization template: %w", err)
	}

	return nil
}

// createCustomKustomizationTemplate creates an empty custom kustomization.yaml template for users
func createCustomKustomizationTemplate(outputPath string) error {
	// Read header template first
	headerContent, err := templates.EmbeddedTemplates.ReadFile("apps/header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header kustomization template: %w", err)
	}

	// Read custom template
	templateContent, err := templates.EmbeddedTemplates.ReadFile("apps/custom.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read custom kustomization template: %w", err)
	}

	// Parse both templates together
	tmpl, err := template.New("header").Parse(string(headerContent))
	if err != nil {
		return fmt.Errorf("failed to parse header template: %w", err)
	}

	tmpl, err = tmpl.New("custom-kustomization").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse custom kustomization template: %w", err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create custom kustomization file: %w", err)
	}
	defer outputFile.Close()

	if err := tmpl.ExecuteTemplate(outputFile, "custom-kustomization", nil); err != nil {
		return fmt.Errorf("failed to execute custom kustomization template: %w", err)
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

			// Generate infrastructure if configured
			if site.Spec.Infra.Base.Source != "" {
				fmt.Println("Generating infrastructure...")

				terraformDir := filepath.Join("clusters", site.Metadata.Name, "infra", "generated")
				if err := os.MkdirAll(terraformDir, 0755); err != nil {
					return fmt.Errorf("create terraform dir: %w", err)
				}

				if err := generateTerraformRoot(terraformDir, site); err != nil {
					return fmt.Errorf("generate terraform root: %w", err)
				}

				fmt.Printf("✓ Generated Terraform root in %s\n", terraformDir)
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

				// Create component directory structure
				componentPath := filepath.Join(appsPath, componentName)
				generatedPath := filepath.Join(componentPath, "generated")
				customPath := filepath.Join(componentPath, "custom")

				if err := os.MkdirAll(generatedPath, 0755); err != nil {
					return fmt.Errorf("failed to create generated directory for %s: %w", componentName, err)
				}

				// Create root kustomization.yaml (only if it doesn't exist)
				rootKustomizationPath := filepath.Join(componentPath, "kustomization.yaml")
				if _, err := os.Stat(rootKustomizationPath); os.IsNotExist(err) {
					if err := createRootKustomization(site, componentName, rootKustomizationPath); err != nil {
						return fmt.Errorf("failed to create root kustomization for %s: %w", componentName, err)
					}
					renderedCount++
				}

				// Create custom/ directory with template (only if it doesn't exist)
				customKustomizationPath := filepath.Join(customPath, "kustomization.yaml")
				if _, err := os.Stat(customKustomizationPath); os.IsNotExist(err) {
					if err := os.MkdirAll(customPath, 0755); err != nil {
						return fmt.Errorf("failed to create custom directory for %s: %w", componentName, err)
					}
					if err := createCustomKustomizationTemplate(customKustomizationPath); err != nil {
						return fmt.Errorf("failed to create custom kustomization template for %s: %w", componentName, err)
					}
				}

				// Find all templates for this component
				componentTemplates, err := FindAppTemplates(componentName)
				if err != nil {
					return fmt.Errorf("failed to find templates for component %s: %w", componentName, err)
				}

				// Render generated/kustomization.yaml
				generatedKustomizationPath := filepath.Join(generatedPath, "kustomization.yaml")

				// If no app specific templates found, use base template
				if len(componentTemplates) == 0 {
					templateName := "apps/base.kustomization.yaml.tmpl"
					if err := RenderKustomizationTemplate(site, componentName, &component, templateName, generatedKustomizationPath); err != nil {
						return fmt.Errorf("failed to render base template for component %s: %w", componentName, err)
					}
					renderedCount++
					continue
				}

				// Render all app specific templates into generated/ directory
				for _, templateName := range componentTemplates {
					// Convert template name to output filename
					// e.g., "apps/pihole/kustomization.yaml.tmpl" -> "kustomization.yaml"
					// e.g., "apps/pihole/secret-patch.yaml.tmpl" -> "secret-patch.yaml"
					baseName := filepath.Base(templateName)
					outputFileName := strings.TrimSuffix(baseName, ".tmpl")
					outputPath := filepath.Join(generatedPath, outputFileName)

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

// generateTerraformRoot generates Terraform root module files from site configuration
func generateTerraformRoot(dir string, site *config.Site) error {
	// Build module source from infra base config
	moduleSource := fmt.Sprintf("git::%s//%s?ref=%s",
		site.Spec.Infra.Base.Source,
		site.Spec.Infra.Base.Path,
		site.Spec.Infra.Base.Ref)

	// Set default content type if not specified
	contentType := site.Spec.Infra.TalosImage.ContentType
	if contentType == "" {
		contentType = "iso"
	}

	// Template data
	data := struct {
		ModuleSource          string
		Site                  *config.Site
		TalosImageContentType string
	}{
		ModuleSource:          moduleSource,
		Site:                  site,
		TalosImageContentType: contentType,
	}

	// Render main.tf
	if err := renderInfraTemplate("infra/main.tf.tmpl", filepath.Join(dir, "main.tf"), data); err != nil {
		return fmt.Errorf("render main.tf: %w", err)
	}

	// Render terraform.tfvars.json
	if err := renderInfraTemplate("infra/terraform.tfvars.json.tmpl", filepath.Join(dir, "terraform.tfvars.json"), data); err != nil {
		return fmt.Errorf("render terraform.tfvars.json: %w", err)
	}

	return nil
}

// renderInfraTemplate renders an infrastructure template to a file
func renderInfraTemplate(templateName, outputPath string, data interface{}) error {
	// Read template content
	templateContent, err := templates.EmbeddedTemplates.ReadFile(templateName)
	if err != nil {
		return fmt.Errorf("read template %s: %w", templateName, err)
	}

	// Parse template
	tmpl, err := template.New(filepath.Base(templateName)).Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", templateName, err)
	}

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	// Execute template
	if err := tmpl.Execute(outputFile, data); err != nil {
		return fmt.Errorf("execute template %s: %w", templateName, err)
	}

	return nil
}
