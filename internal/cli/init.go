package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	stackSource  string
	stackVersion string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <cluster-name>",
		Short: "Initialize a new klabctl cluster",
		Long:  "Initialize a new cluster by pulling the stack and creating a cluster-specific site.yaml configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName := args[0]
			return initProject(clusterName)
		},
	}

	cmd.Flags().StringVar(&stackSource, "stack-source", "https://github.com/bamaas/klabctl", "Git repository URL for the stack")
	cmd.Flags().StringVar(&stackVersion, "stack-version", "main", "Stack version (branch, tag, or commit)")

	return cmd
}

func initProject(clusterName string) error {
	fmt.Printf("🚀 Initializing cluster '%s'...\n", clusterName)

	// Create cluster directory: clusters/<cluster-name>/
	clusterDir := filepath.Join("clusters", clusterName)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return fmt.Errorf("failed to create cluster directory: %w", err)
	}

	// Check if site.yaml already exists
	siteYamlPath := filepath.Join(clusterDir, "site.yaml")
	if _, err := os.Stat(siteYamlPath); err == nil {
		return fmt.Errorf("cluster '%s' already exists (site.yaml found at %s)", clusterName, siteYamlPath)
	}

	// Create .klabctl directory structure at root
	klabctlDir := ".klabctl"
	cacheDir := filepath.Join(klabctlDir, "cache", "stack")

	fmt.Printf("📦 Pulling stack from %s@%s...\n", stackSource, stackVersion)

	// Pull stack to cache: .klabctl/cache/stack/<version>/
	stackCacheDir := filepath.Join(cacheDir, stackVersion)
	if err := pullStack(stackSource, stackVersion, stackCacheDir); err != nil {
		return fmt.Errorf("failed to pull stack: %w", err)
	}

	fmt.Println("✓ Stack pulled successfully")

	// Create .klabctl/config.yaml to track the stack version
	configPath := filepath.Join(klabctlDir, "config.yaml")
	if err := createKlabctlConfig(configPath, stackSource, stackVersion); err != nil {
		return fmt.Errorf("failed to create klabctl config: %w", err)
	}

	// Generate site.yaml in cluster directory
	fmt.Println("📝 Generating site.yaml...")
	if err := generateSiteYaml(siteYamlPath, clusterName, stackSource, stackVersion); err != nil {
		return fmt.Errorf("failed to generate site.yaml: %w", err)
	}

	fmt.Printf("✓ Generated %s\n", siteYamlPath)

	// Create .gitignore at root (only if it doesn't exist)
	gitignorePath := ".gitignore"
	if err := createGitignore(gitignorePath); err != nil {
		fmt.Printf("Warning: failed to create .gitignore: %v\n", err)
	} else {
		fmt.Println("✓ Generated .gitignore")
	}

	fmt.Println()
	fmt.Println("✨ Cluster initialized successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit %s to configure your cluster\n", siteYamlPath)
	fmt.Printf("  2. Run 'klabctl render --site %s' to generate manifests\n", siteYamlPath)
	fmt.Println("  3. Deploy your cluster!")
	fmt.Println()

	return nil
}

// pullStack clones the stack repository to the cache directory
func pullStack(source, version, destDir string) error {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}

	// Remove existing directory if it exists
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to remove existing cache: %w", err)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Clone repository
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", version, source, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Remove .git directory to save space
	gitDir := filepath.Join(destDir, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		fmt.Printf("Warning: failed to remove .git directory: %v\n", err)
	}

	return nil
}

// createKlabctlConfig creates the .klabctl/config.yaml file
func createKlabctlConfig(configPath, source, version string) error {
	config := map[string]interface{}{
		"stack": map[string]string{
			"source":  source,
			"version": version,
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// createGitignore creates a .gitignore file for the project
func createGitignore(outputPath string) error {
	// Don't overwrite existing .gitignore
	if _, err := os.Stat(outputPath); err == nil {
		return nil
	}

	gitignoreContent := `# klabctl cache
.klabctl/cache/

# Generated manifests (optional - uncomment to gitignore generated files)
# clusters/*/apps/*/generated/
# clusters/*/apps/*/base/
# clusters/*/infra/generated/
# clusters/*/infra/base/
`

	if err := os.WriteFile(outputPath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	return nil
}

// generateSiteYaml creates a basic site.yaml file
func generateSiteYaml(outputPath, clusterName, stackSource, stackVersion string) error {
	site := map[string]interface{}{
		"apiVersion": "klab/v1alpha1",
		"kind":       "Site",
		"metadata": map[string]string{
			"name": clusterName,
		},
		"spec": map[string]interface{}{
			"stack": map[string]string{
				"source":  stackSource,
				"version": stackVersion,
			},
			"apps": map[string]interface{}{
				"catalog": map[string]interface{}{
					"cilium": map[string]interface{}{
						"enabled": true,
					},
					"metallb": map[string]interface{}{
						"enabled": true,
					},
					"ingress-nginx": map[string]interface{}{
						"enabled": true,
						"values": map[string]interface{}{
							"ip": "192.168.1.150",
						},
					},
					"cert-manager": map[string]interface{}{
						"enabled": false,
						"values": map[string]interface{}{
							"letsencrypt": map[string]interface{}{
								"email": "admin@example.com",
							},
							"cloudflare": map[string]interface{}{
								"apiToken": "your-cloudflare-api-token-here",
							},
						},
					},
				},
			},
		},
	}

	data, err := yaml.Marshal(site)
	if err != nil {
		return fmt.Errorf("failed to marshal site.yaml: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write site.yaml: %w", err)
	}

	return nil
}
