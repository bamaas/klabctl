package cli

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/spf13/cobra"
)

// TemplateData holds the data used for templating
type TemplateData struct {
	Site     *config.Site
	Versions map[string]string
}

// RenderKustomizationTemplate renders the kustomization.yaml.tmpl template
func RenderKustomizationTemplate(site *config.Site, templatePath, outputPath string) error {
	// Default versions for common components
	versions := map[string]string{
		"external-dns": "6.15.0",
		"cert-manager": "1.15.0",
		"nginx":        "15.0.0",
		"metallb":      "0.14.0",
	}

	// Create template with custom functions
	funcMap := template.FuncMap{
		"quote": func(s string) string {
			return fmt.Sprintf(`"%s"`, s)
		},
	}

	tmpl, err := template.New("kustomization.yaml.tmpl").Funcs(funcMap).ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	data := TemplateData{
		Site:     site,
		Versions: versions,
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	if err := tmpl.Execute(outputFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// RenderKustomizationTemplateToString renders the template to a string
func RenderKustomizationTemplateToString(site *config.Site, templatePath string) (string, error) {

	// Create template with custom functions
	funcMap := template.FuncMap{
		"quote": func(s string) string {
			return fmt.Sprintf(`"%s"`, s)
		},
	}

	tmpl, err := template.New("kustomization.yaml.tmpl").Funcs(funcMap).ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	data := TemplateData{
		Site:     site,
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return result.String(), nil
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

			// Render the kustomization template
			templatePath := "templates/kustomization.yaml.tmpl"
			outputPath := "kustomization.yaml"

			if err := RenderKustomizationTemplate(site, templatePath, outputPath); err != nil {
				return fmt.Errorf("failed to render template: %w", err)
			}

			fmt.Printf("Rendered kustomization.yaml from template\n")
			return nil
		},
	}
	return cmd
}
