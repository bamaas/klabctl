package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
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

	fmt.Println(siteYaml)

	return nil
}
