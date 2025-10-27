package cli

import (
	"fmt"
	"os"
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

	// Ensure stack is available using pull.go functionality
	stackCacheDir := filepath.Join(cacheDir, stackVersion)
	if err := EnsureStackAvailable(stackSource, stackVersion, stackCacheDir); err != nil {
		return err
	}

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
			"infra": map[string]interface{}{
				"provider": map[string]interface{}{
					"name": "proxmox",
					"proxmox": map[string]interface{}{
						"endpoint": "https://pve.example.local:8006/api2/json",
						"tokenID":  "root@pam!terraform",
					},
				},
				"talosImage": map[string]interface{}{
					"url":         "https://factory.talos.dev/image/abc123def456/v1.10.3/nocloud-amd64.iso",
					"fileName":    "talos-1.10.3-nocloud-amd64.iso",
					"nodeName":    "pve",
					"datastoreId": "local",
					"overwrite":   false,
					"contentType": "iso",
				},
				"nodeData": map[string]interface{}{
					"controlPlanes": []map[string]interface{}{
						{
							"ip":       "192.168.1.10",
							"hostname": "k8s-cp-1",
							"pveNode":  "pve",
							"pveId":    5000,
							"memory":   8192,
							"cores":    4,
							"diskSize": 40,
						},
					},
					"workers": []map[string]interface{}{
						{
							"ip":       "192.168.1.20",
							"hostname": "k8s-w-1",
							"pveNode":  "pve",
							"pveId":    6000,
							"memory":   16384,
							"cores":    8,
							"diskSize": 100,
						},
					},
				},
				"cluster": map[string]interface{}{
					"name":            clusterName,
					"endpoint":        "https://192.168.1.10:6443",
					"virtualSharedIp": "192.168.1.100",
					"domain":          "cluster.local",
					"defaultGateway":  "192.168.1.1",
				},
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
