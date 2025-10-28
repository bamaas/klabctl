package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get resources",
		Long:  "Get resources and information from the stack.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newGetDefaultsCmd())

	return cmd
}

func newGetDefaultsCmd() *cobra.Command {

	var stackSource string
	var stackVersion string
	var clusterName string

	cmd := &cobra.Command{
		Use:   "defaults",
		Short: "Get default site.yaml configuration from a stack",
		Long: `Get default site.yaml configuration populated with values from a stack's values.yaml files.

This generates a site.yaml with all default values from the specified stack.
Output is printed to stdout. The stack must already be cached (run 'klabctl pull' first).

Examples:
  # Get defaults with default cluster name
  klabctl get defaults

  # Get defaults for specific cluster
  klabctl get defaults --cluster-name production
  klabctl get defaults -c my-cluster

  # Get defaults from specific stack version
  klabctl get defaults --stack-version v1.2.0

  # Combine all options
  klabctl get defaults \
    --cluster-name production \
    --stack-source https://github.com/user/stack.git \
    --stack-version main

  # Save to file
  klabctl get defaults > site.yaml
  klabctl get defaults -c production > clusters/production/site.yaml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getDefaults(stackSource, stackVersion, clusterName)
		},
	}

	cmd.Flags().StringVarP(&clusterName, "cluster-name", "n", "my-cluster", "Cluster name (default: my-cluster)")
	cmd.Flags().StringVar(&stackSource, "stack-source", "https://github.com/bamaas/klabctl", "Stack git repository URL (default: https://github.com/bamaas/klabctl.git)")
	cmd.Flags().StringVar(&stackVersion, "stack-version", "main", "Stack version/branch (default: main)")

	return cmd
}

func getDefaults(stackSource string, stackVersion string, clusterName string) error {
	// Ensure stack is available
	stackCacheDir := filepath.Join(".klabctl", "cache", "stack", stackVersion)
	if err := EnsureStackAvailable(stackSource, stackVersion, stackCacheDir); err != nil {
		return fmt.Errorf("failed to ensure stack is available: %w", err)
	}

	// Generate the site.yaml with defaults
	siteYaml, err := generateSiteYaml("", clusterName, stackSource, stackVersion)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "# Default configuration values for stack %s@%s\n", stackSource, stackVersion)
	fmt.Println(siteYaml)

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
func generateSiteYaml(outputPath, clusterName, stackSource, stackVersion string) (string, error) {
	// Load infra defaults
	infraDefaults, err := loadInfraDefaults(stackVersion)
	if err != nil {
		return "", fmt.Errorf("failed to load infra defaults: %w", err)
	}

	// Override cluster name
	if cluster, ok := infraDefaults["cluster"].(map[string]interface{}); ok {
		cluster["name"] = clusterName
	}

	// Discover all apps
	discoveredApps, err := discoverAppsWithDefaults(stackVersion)
	if err != nil {
		return "", fmt.Errorf("failed to discover apps: %w", err)
	}

	// Build app catalog - all apps enabled by default
	catalog := make(map[string]interface{})
	for _, appName := range discoveredApps {
		appDefaults, err := loadAppDefaults(stackVersion, appName)
		if err != nil {
			return "", fmt.Errorf("failed to load defaults for %s: %w", appName, err)
		}

		appConfig := map[string]interface{}{
			"enabled": true,
		}

		if len(appDefaults) > 0 {
			appConfig["values"] = appDefaults
		}

		catalog[appName] = appConfig
	}

	// Build site structure
	spec := map[string]interface{}{
		"stack": map[string]string{
			"source":  stackSource,
			"version": stackVersion,
		},
		"infra": infraDefaults,
		"apps": map[string]interface{}{
			"catalog": catalog,
		},
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
		return "", fmt.Errorf("failed to marshal site.yaml: %w", err)
	}

	// If outputPath is empty return the data as a string
	if outputPath == "" {
		return string(data), nil
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write site.yaml: %w", err)
	}

	return "", nil
}