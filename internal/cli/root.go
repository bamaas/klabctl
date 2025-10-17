package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var sitePath string

var rootCmd = &cobra.Command{
	Use:   "klabctl",
	Short: "Klabctl as a Product CLI",
	Long:  "klabctl: takes a site.yaml and produces cluster GitOps artifacts and can provision infra.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&sitePath, "site", "s", "site.yaml", "Path to site.yaml")
	rootCmd.AddCommand(newRenderCmd())
	rootCmd.AddCommand(newProvisionInfraCmd())
	rootCmd.AddCommand(newVendorCmd())
	// rootCmd.AddCommand(newInitCmd())
}
