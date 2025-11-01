package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate cluster GitOps skeleton from site.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := config.LoadSiteFromFile(sitePath)
			if err != nil {
				return err
			}

			// Ensure stack is available before rendering
			if site.Spec.Stack.Source == "" || site.Spec.Stack.Ref == "" {
				return fmt.Errorf("stack.source and stack.version are required in site.yaml")
			}

			if err := EnsureStackAvailable(site.Spec.Stack.Source, site.Spec.Stack.Ref, false); err != nil {
				return fmt.Errorf("failed to ensure stack is available: %w", err)
			}

			// Generate infrastructure if configured (check if provider is set)
			if site.Spec.Infra.Provider.Name != "" {

				// Copy infra base from cache
				if err := copyInfraBase(site); err != nil {
					return fmt.Errorf("failed to copy infra base: %w", err)
				}

				terraformDir := filepath.Join("clusters", site.Metadata.Name, "infra", "generated")
				if err := os.MkdirAll(terraformDir, 0755); err != nil {
					return fmt.Errorf("create terraform dir: %w", err)
				}

				if err := generateTerraformRoot(terraformDir, site); err != nil {
					return fmt.Errorf("generate terraform root: %w", err)
				}

				// fmt.Printf("✓ Copied infra base from cache\n")
				fmt.Printf("✓ Generated infrastructure configuration\n")
			}

			// Define path to components directory
			appsPath := filepath.Join("clusters", site.Metadata.Name, "apps")

			// Create components directory if it doesn't exist
			if err := os.MkdirAll(appsPath, 0755); err != nil {
				return fmt.Errorf("failed to create apps directory: %w", err)
			}

			// Render all templates for each component
			renderedCount := 0
			copiedCount := 0
			for componentName, component := range site.Spec.Apps.Catalog {
				if !component.Enabled {
					continue // Skip disabled components
				}

				// Copy app base from cache to cluster directory
				// fmt.Printf("Copying base for %s...\n", componentName)
				if err := copyAppBase(site, componentName); err != nil {
					return fmt.Errorf("failed to copy base for %s: %w", componentName, err)
				}
				copiedCount++

				// Create component directory structure
				project := site.Spec.Apps.Catalog[componentName].Project
				namespace := site.Spec.Apps.Catalog[componentName].Namespace
				if project == "" {
					return fmt.Errorf("project is required for app %s", componentName)
				}
				if namespace == "" {
					return fmt.Errorf("namespace is required for app %s", componentName)
				}
				componentPath := filepath.Join(appsPath, project, namespace, componentName)
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

				// create custom/ directory if it doesn't exist
				if err := os.MkdirAll(customPath, 0755); err != nil {
					return fmt.Errorf("failed to create custom directory for %s: %w", componentName, err)
				}

				// create custom/values.yaml if it doesn't exist
				customValuesPath := filepath.Join(customPath, "values.yaml")
				if _, err := os.Stat(customValuesPath); os.IsNotExist(err) {
					if err := createCustomValuesTemplate(site, customValuesPath); err != nil {
						return fmt.Errorf("failed to create custom values template for %s: %w", componentName, err)
					}
				}

				// Create custom kustomization.yaml if it doesn't exist
				customKustomizationPath := filepath.Join(customPath, "kustomization.yaml")
				if _, err := os.Stat(customKustomizationPath); os.IsNotExist(err) {
					if err := createCustomKustomizationTemplate(site, customKustomizationPath); err != nil {
						return fmt.Errorf("failed to create custom kustomization template for %s: %w", componentName, err)
					}
				}

				// Find all templates for this component
				componentTemplates, err := FindAppTemplates(site, componentName)
				if err != nil {
					return fmt.Errorf("failed to find templates for component %s: %w", componentName, err)
				}

				// Render generated/kustomization.yaml
				generatedKustomizationPath := filepath.Join(generatedPath, "kustomization.yaml")

				// If no app specific templates found, use base template
				if len(componentTemplates) == 0 {
					templateName := "base.kustomization.yaml.tmpl"
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

			// fmt.Printf("✓ Copied %d app base(s) from cache\n", copiedCount)
			fmt.Printf("✓ Generated %d application components\n", renderedCount)
			return nil
		},
	}

	return cmd
}

// TemplateData holds the data used for templating
type TemplateData struct {
	Site          *config.Site
	Versions      map[string]string
	Component     *config.Component
	ComponentName string
	AllComponents map[string]config.Component
}

// readTemplateFromCache reads a template file from the cache
func readTemplateFromCache(site *config.Site, templatePath string) ([]byte, error) {
	// Check if it's an app-specific template (apps/{appName}/templates/{file})
	if strings.HasPrefix(templatePath, "apps/") {
		fullPath := filepath.Join(getStackCacheDir(site), "stack", templatePath)
		return os.ReadFile(fullPath)
	}

	// Otherwise it's a general template (templates/{file})
	fullPath := filepath.Join(getStackTemplatesDir(site), templatePath)
	return os.ReadFile(fullPath)
}

// RenderComponentKustomizationTemplate renders the kustomization.yaml.tmpl template for a specific component from cache
func RenderKustomizationTemplate(site *config.Site, componentName string, component *config.Component, templateName, outputPath string) error {

	// Create template with custom functions
	funcMap := template.FuncMap{
		"quote": func(s string) string {
			return fmt.Sprintf(`"%s"`, s)
		},
	}

	// Read header template first
	headerContent, err := readTemplateFromCache(site, "header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header template: %w", err)
	}

	// Read base template
	baseTemplatePath := "base.kustomization.yaml.tmpl"
	baseContent, err := readTemplateFromCache(site, baseTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read base template %s: %w", baseTemplatePath, err)
	}

	// Read component-specific template
	templateContent, err := readTemplateFromCache(site, templateName)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templateName, err)
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

// RenderTemplate renders any template to a file using cache templates
func RenderTemplate(site *config.Site, componentName string, component *config.Component, templateName, outputPath string) error {
	// Create template with custom functions
	funcMap := template.FuncMap{
		"quote": func(s string) string {
			return fmt.Sprintf(`"%s"`, s)
		},
	}

	// Read header template first
	headerContent, err := readTemplateFromCache(site, "header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header template: %w", err)
	}

	// Read base template (for inheritance)
	baseTemplatePath := "base.kustomization.yaml.tmpl"
	baseContent, err := readTemplateFromCache(site, baseTemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read base template %s: %w", baseTemplatePath, err)
	}

	// Read component-specific template
	templateContent, err := readTemplateFromCache(site, templateName)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templateName, err)
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
	headerContent, err := readTemplateFromCache(site, "header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header kustomization template: %w", err)
	}

	// Read root template
	templateContent, err := readTemplateFromCache(site, "root.kustomization.yaml.tmpl")
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
func createCustomKustomizationTemplate(site *config.Site, outputPath string) error {
	// Read header template first
	headerContent, err := readTemplateFromCache(site, "header.kustomization.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read header kustomization template: %w", err)
	}

	// Read custom template
	templateContent, err := readTemplateFromCache(site, "custom.kustomization.yaml.tmpl")
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

func createCustomValuesTemplate(site *config.Site, outputPath string) error {
	// Read custom values template
	templateContent, err := readTemplateFromCache(site, "custom.values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read custom values template: %w", err)
	}

	tmpl, err := template.New("custom-values").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse custom values template: %w", err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create custom values file: %w", err)
	}
	defer outputFile.Close()

	if err := tmpl.ExecuteTemplate(outputFile, "custom-values", nil); err != nil {
		return fmt.Errorf("failed to execute custom values template: %w", err)
	}

	return nil
}

// FindAppTemplates finds all templates for a specific component in the cache
func FindAppTemplates(site *config.Site, componentName string) ([]string, error) {
	var componentTemplates []string

	// Check for component-specific templates in cache
	componentDir := filepath.Join(getStackAppsDir(site), componentName, "templates")

	// Check if the templates directory exists
	if _, err := os.Stat(componentDir); os.IsNotExist(err) {
		// No templates for this component
		return componentTemplates, nil
	}

	// Walk through the component's templates directory
	err := filepath.WalkDir(componentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Only include .tmpl files
		if !d.IsDir() && strings.HasSuffix(path, ".tmpl") {
			// Convert absolute path to relative path from stack directory
			relPath, err := filepath.Rel(filepath.Join(getStackCacheDir(site), "stack"), path)
			if err != nil {
				return err
			}
			componentTemplates = append(componentTemplates, relPath)
		}
		return nil
	})

	return componentTemplates, err
}

// generateTerraformRoot generates Terraform root module files from site configuration
func generateTerraformRoot(dir string, site *config.Site) error {
	// Use local infra/base module
	moduleSource := "../../base"

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
	if err := renderInfraTemplate(site, "main.tf.tmpl", filepath.Join(dir, "main.tf"), data); err != nil {
		return fmt.Errorf("render main.tf: %w", err)
	}

	// Render terraform.tfvars.json
	if err := renderInfraTemplate(site, "terraform.tfvars.json.tmpl", filepath.Join(dir, "terraform.tfvars.json"), data); err != nil {
		return fmt.Errorf("render terraform.tfvars.json: %w", err)
	}

	return nil
}

// copyAppBase copies an app's base from cache to cluster directory
func copyAppBase(site *config.Site, appName string) error {
	// Source: cache/stack/{version}/stack/apps/{appName}/base
	sourcePath := filepath.Join(getStackCacheDir(site), "stack", "apps", appName, "base")

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("app base not found in cache: %s", appName)
	}

	// Destination: clusters/{site}/apps/{appName}/base
	clusterName := site.Metadata.Name
	project := site.Spec.Apps.Catalog[appName].Project
	if project == "" {
		return fmt.Errorf("project is required for app %s", appName)
	}
	namespace := site.Spec.Apps.Catalog[appName].Namespace
	if namespace == "" {
		return fmt.Errorf("namespace is required for app %s", appName)
	}
	destPath := filepath.Join("clusters", clusterName, "apps", project, namespace, appName, "base")

	// Remove existing base directory
	if err := os.RemoveAll(destPath); err != nil {
		return fmt.Errorf("failed to remove existing base: %w", err)
	}

	// Copy base
	if err := copyDir(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy app base: %w", err)
	}

	return nil
}

// copyInfraBase copies infrastructure base from cache to cluster directory
func copyInfraBase(site *config.Site) error {
	// Determine the infra base path in cache
	// For klabctl, it should be in stack/infra/base
	sourcePath := filepath.Join(getStackCacheDir(site), "stack", "infra", "base")

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("infra base not found in cache at: %s", sourcePath)
	}

	// Destination: clusters/{site}/infra/base
	destPath := filepath.Join("clusters", site.Metadata.Name, "infra", "base")

	// Remove existing base directory
	if err := os.RemoveAll(destPath); err != nil {
		return fmt.Errorf("failed to remove existing infra base: %w", err)
	}

	// Copy base
	if err := copyDir(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy infra base: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source file info to preserve permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := srcFile.WriteTo(dstFile); err != nil {
		return err
	}

	return nil
}

// renderInfraTemplate renders an infrastructure template to a file from cache
func renderInfraTemplate(site *config.Site, templateName, outputPath string, data interface{}) error {
	// Read template content from cache (infra templates are in stack/infra/templates/)
	fullPath := filepath.Join(getStackCacheDir(site), "stack", "infra", "templates", templateName)
	templateContent, err := os.ReadFile(fullPath)
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
