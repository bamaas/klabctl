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

// loadInfraDefaults loads the default infra values from the stack cache
func loadInfraDefaults(stackVersion string) (map[string]interface{}, error) {
	valuesPath := filepath.Join(".klabctl", "cache", "stack", stackVersion, "stack", "infra", "templates", "values.yaml")
	
	// Read the values file
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		// If values file doesn't exist, return empty map (backward compatibility)
		return make(map[string]interface{}), nil
	}
	
	// Parse YAML
	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse infra values: %w", err)
	}
	
	return values, nil
}

// loadAppDefaults loads default values for a specific app from the stack cache
func loadAppDefaults(stackVersion, appName string) (map[string]interface{}, error) {
	valuesPath := filepath.Join(".klabctl", "cache", "stack", stackVersion, "stack", "apps", appName, "templates", "values.yaml")
	
	// Read the values file
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		// If values file doesn't exist, return empty map
		return make(map[string]interface{}), nil
	}
	
	// Parse YAML
	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse %s values: %w", appName, err)
	}
	
	return values, nil
}

// discoverAppsWithDefaults discovers all apps that have templates/values.yaml in the stack
func discoverAppsWithDefaults(stackVersion string) ([]string, error) {
	appsDir := filepath.Join(".klabctl", "cache", "stack", stackVersion, "stack", "apps")
	
	var apps []string
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		// If apps directory doesn't exist, return empty list
		return apps, nil
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		// Check if this app has a templates/values.yaml
		valuesPath := filepath.Join(appsDir, entry.Name(), "templates", "values.yaml")
		if _, err := os.Stat(valuesPath); err == nil {
			apps = append(apps, entry.Name())
		}
	}
	
	return apps, nil
}

// generateSiteYaml creates a basic site.yaml file
func generateSiteYaml(outputPath, clusterName, stackSource, stackVersion string) error {
	// Load infra defaults from stack
	infraDefaults, err := loadInfraDefaults(stackVersion)
	if err != nil {
		return fmt.Errorf("failed to load infra defaults: %w", err)
	}
	
	// Override cluster name in defaults
	if cluster, ok := infraDefaults["cluster"].(map[string]interface{}); ok {
		cluster["name"] = clusterName
	}
	
	// Discover all apps with defaults
	discoveredApps, err := discoverAppsWithDefaults(stackVersion)
	if err != nil {
		return fmt.Errorf("failed to discover apps: %w", err)
	}
	
	// Build app catalog dynamically
	catalog := make(map[string]interface{})
	
	// Define which apps should be enabled by default
	defaultEnabled := map[string]bool{
		"cilium":        true,
		"metallb":       true,
		"ingress-nginx": true,
		"cert-manager":  false,
		"pihole":        false,
		"external-dns":  false,
	}
	
	for _, appName := range discoveredApps {
		// Load defaults for this app
		appDefaults, err := loadAppDefaults(stackVersion, appName)
		if err != nil {
			return fmt.Errorf("failed to load defaults for %s: %w", appName, err)
		}
		
		// Determine if enabled by default
		enabled, hasDefault := defaultEnabled[appName]
		if !hasDefault {
			enabled = false // Default to disabled for unknown apps
		}
		
		appConfig := map[string]interface{}{
			"enabled": enabled,
		}
		
		// Add values if they exist
		if len(appDefaults) > 0 {
			appConfig["values"] = appDefaults
		}
		
		catalog[appName] = appConfig
	}
	
	// Build spec
	spec := map[string]interface{}{
		"stack": map[string]string{
			"source":  stackSource,
			"version": stackVersion,
		},
		"apps": map[string]interface{}{
			"catalog": catalog,
		},
	}
	
	// Add infra section if defaults were loaded
	if len(infraDefaults) > 0 {
		spec["infra"] = infraDefaults
	}

	site := map[string]interface{}{
		"apiVersion": "klab/v1alpha1",
		"kind":       "Site",
		"metadata": map[string]string{
			"name": clusterName,
		},
		"spec": spec,
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
