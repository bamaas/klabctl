package cli

import (
	"fmt"

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
