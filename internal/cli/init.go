package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	stackSource string
	stackRef    string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <cluster-name>",
		Short: "Initialize a new cluster",
		Long:  "Initialize a new cluster by pulling the stack and creating a cluster-specific site.yaml configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName := args[0]
			return initProject(clusterName)
		},
	}

	cmd.Flags().StringVar(&stackSource, "stack-source", "https://github.com/bamaas/klabctl", "Git repository URL for the stack")
	cmd.Flags().StringVar(&stackRef, "stack-ref", "main", "Stack reference (branch, tag, or commit)")

	return cmd
}

func initProject(clusterName string) error {
	fmt.Printf("🚀 Initializing cluster '%s'...\n", clusterName)

	// Create cluster directory: clusters/<cluster-name>/
	clusterDir := filepath.Join("clusters", clusterName)
	if _, err := os.Stat(clusterDir); err == nil {
		return fmt.Errorf("cluster directory '%s' already exists", clusterDir)
	}
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return fmt.Errorf("failed to create cluster directory: %w", err)
	}

	// Check if site.yaml already exists
	siteYamlPath := filepath.Join(clusterDir, "site.yaml")
	if _, err := os.Stat(siteYamlPath); err == nil {
		return fmt.Errorf("cluster '%s' already exists (site.yaml found at %s)", clusterName, siteYamlPath)
	}

	// Ensure stack is available using pull.go functionality
	if err := EnsureStackAvailable(stackSource, stackRef, false); err != nil {
		return err
	}

	// Generate site.yaml in cluster directory
	// fmt.Println("Generating site.yaml...")
	if _, err := generateSiteYaml(siteYamlPath, clusterName, stackSource, stackRef); err != nil {
		return fmt.Errorf("failed to generate site.yaml: %w", err)
	}
	fmt.Printf("✓ Generated %s\n", siteYamlPath)

	// Create .gitignore at root (only if it doesn't exist)
	// fmt.Println("Creating .gitignore...")
	gitignorePath := ".gitignore"
	created, err := createGitignore(gitignorePath)
	if err != nil {
		fmt.Printf("Warning: failed to create .gitignore: %v\n", err)
	} else if created {
		fmt.Printf("✓ Generated .gitignore at %s\n", gitignorePath)
	} else {
		fmt.Printf("✓ .gitignore already exists")
	}

	fmt.Println()
	fmt.Println("\n✨ Cluster initialized successfully!")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit %s to configure your cluster\n", siteYamlPath)
	fmt.Printf("  2. Run 'klabctl render --site %s' to generate manifests\n", siteYamlPath)
	fmt.Println("  3. Deploy your cluster!")
	fmt.Println()

	return nil
}

// createGitignore creates a .gitignore file for the project
// Returns true if the file was created, false if it already existed, and any error
func createGitignore(outputPath string) (bool, error) {
	// Don't overwrite existing .gitignore
	if _, err := os.Stat(outputPath); err == nil {
		return false, nil
	}

	gitignoreContent := `
.klabctl
`

	if err := os.WriteFile(outputPath, []byte(gitignoreContent), 0644); err != nil {
		return false, err
	}

	return true, nil
}
