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
	var stackRef string
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
			return getDefaults(stackSource, stackRef, clusterName)
		},
	}

	cmd.Flags().StringVarP(&clusterName, "cluster-name", "n", "my-cluster", "Cluster name (default: my-cluster)")
	cmd.Flags().StringVar(&stackSource, "stack-source", "https://github.com/bamaas/klabctl", "Stack git repository URL (default: https://github.com/bamaas/klabctl.git)")
	cmd.Flags().StringVar(&stackRef, "stack-ref", "main", "Stack reference (version/branch/commit) (default: main)")

	return cmd
}

func getDefaults(stackSource string, stackVersion string, clusterName string) error {
	// Ensure stack is available
	if err := EnsureStackAvailable(stackSource, stackVersion, false); err != nil {
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
	valuesPath := filepath.Join(stackCacheDirRoot, stackVersion, "stack", "infra", "templates", "values.yaml")

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

// loadYamlFile
func loadYamlFile(path string) (map[string]interface{}, error) {

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]interface{}), nil
	}

	// Unmarshal the data into a map[string]interface{}
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return yamlData, nil
}

// discoverAppsWithDefaults discovers all apps that have templates/values.yaml in the stack
func discoverAppsWithDefaults(stackRef string) ([]string, error) {
	appsDir := filepath.Join(stackCacheDirRoot, stackRef, "stack", "apps")

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
func generateSiteYaml(outputPath, clusterName, stackSource, stackRef string) (string, error) {
	// Load infra defaults
	infraDefaults, err := loadInfraDefaults(stackRef)
	if err != nil {
		return "", fmt.Errorf("failed to load infra defaults: %w", err)
	}

	// Override cluster name
	if cluster, ok := infraDefaults["cluster"].(map[string]interface{}); ok {
		cluster["name"] = clusterName
	}

	// Discover all apps
	discoveredApps, err := discoverAppsWithDefaults(stackRef)
	if err != nil {
		return "", fmt.Errorf("failed to discover apps: %w", err)
	}

	// Load meta.yaml for each app
	catalog := make(map[string]interface{})
	for _, appName := range discoveredApps {
		metaYamlPath := filepath.Join(stackCacheDirRoot, stackRef, "stack", "apps", appName, "meta.yaml")
		meta, err := loadYamlFile(metaYamlPath)
		if err != nil {
			return "", fmt.Errorf("failed to load meta for %s: %w", appName, err)
		}
		catalog[appName] = meta
	}

	// Load values.yaml for each app
	for _, appName := range discoveredApps {
		valuesYamlPath := filepath.Join(stackCacheDirRoot, stackRef, "stack", "apps", appName, "values.yaml")
		appDefaultValues, err := loadYamlFile(valuesYamlPath)
		if err != nil {
			return "", fmt.Errorf("failed to load defaults for %s: %w", appName, err)
		}

		// Ensure appConfig exists and is a map[string]interface{}
		appConfig, ok := catalog[appName].(map[string]interface{})
		if !ok {
			appConfig = make(map[string]interface{})
		}

		// If there are default values, set them
		if len(appDefaultValues) > 0 {
			appConfig["values"] = appDefaultValues
		}

		// Set the appConfig back in the catalog
		catalog[appName] = appConfig
	}

	// Build site structure
	spec := map[string]interface{}{
		"stack": map[string]string{
			"source": stackSource,
			"ref":    stackRef,
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
