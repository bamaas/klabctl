package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/spf13/cobra"
)

func newProvisionInfraCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision infrastructure using Terraform",
		Long:  "Runs terraform init and apply to provision VMs",
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := config.LoadSiteFromFile(sitePath)
			if err != nil {
				return fmt.Errorf("load site: %w", err)
			}

			if site.Spec.Infra.Provider == "" {
				return fmt.Errorf("no infrastructure provider configured in site.yaml")
			}

			name := site.Metadata.Name
			if name == "" {
				return fmt.Errorf("metadata.name is required")
			}

			terraformDir := filepath.Join("clusters", name, "infra", "generated")

			if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
				return fmt.Errorf("terraform directory not found; run 'klabctl render' first")
			}

			if _, err := exec.LookPath("terraform"); err != nil {
				return fmt.Errorf("terraform not found in PATH")
			}

			fmt.Printf("Provisioning infrastructure for site: %s\n\n", name)

			// terraform init
			fmt.Println("Running terraform init...")
			cmdInit := exec.Command("terraform", "-chdir="+terraformDir, "init")
			cmdInit.Stdout = os.Stdout
			cmdInit.Stderr = os.Stderr
			cmdInit.Env = os.Environ()
			if err := cmdInit.Run(); err != nil {
				return fmt.Errorf("terraform init failed: %w", err)
			}

			// terraform apply
			fmt.Println("\nRunning terraform apply...")
			cmdApply := exec.Command("terraform", "-chdir="+terraformDir, "apply",
				"-var-file=terraform.tfvars.json", "-auto-approve")
			cmdApply.Stdout = os.Stdout
			cmdApply.Stderr = os.Stderr
			cmdApply.Env = os.Environ()
			if err := cmdApply.Run(); err != nil {
				return fmt.Errorf("terraform apply failed: %w", err)
			}

			fmt.Println("\n✓ Infrastructure provisioned successfully")

			return nil
		},
	}
	return cmd
}
