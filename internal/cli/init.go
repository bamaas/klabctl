package cli

import (
	"fmt"
	"os"
	"text/template"

	"github.com/bamaas/klabctl/internal/templates"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new klabctl project",
		Long:  "Creates a sample site.yaml configuration file to get started with klabctl",
		RunE: func(cmd *cobra.Command, args []string) error {
			return initProject(cmd, args)
		},
	}
	return cmd
}

func initProject(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Initializing new klabctl project...")
	if err := renderSiteYaml(); err != nil {
		return fmt.Errorf("failed to render site.yaml: %w", err)
	}
	fmt.Println()
	fmt.Println("This will create:")
	fmt.Println("  • site.yaml - Main configuration file")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit site.yaml to match your infrastructure")
	fmt.Println("  2. Run 'klabctl vendor' to fetch base components")
	fmt.Println("  3. Run 'klabctl render' to generate cluster manifests")
	fmt.Println("  4. Run 'klabctl provision' to provision infrastructure")
	fmt.Println()
	fmt.Println("✨ Happy cluster building!")
	return nil
}

func renderSiteYaml() error {
	outputPath := "site.yaml"

	// Check if site.yaml already exists
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("file %s already exists, not overwriting", outputPath)
	}

	// Read the site.yaml.tmpl template from the embedded templates
	templateContent, err := templates.EmbeddedTemplates.ReadFile("site.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read embedded template %s: %w", "site.yaml.tmpl", err)
	}

	// Parse the template
	tmpl, err := template.New("site.yaml").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", "site.yaml.tmpl", err)
	}

	// Create the output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	// Execute the template
	if err := tmpl.Execute(outputFile, nil); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", "site.yaml.tmpl", err)
	}
	return nil
}
