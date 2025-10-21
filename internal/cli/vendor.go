package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newVendorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendor",
		Short: "Vendor applications and infrastructure base to cluster-specific location",
		Long:  "Vendors the applications and infrastructure base from the source repository to the cluster's base directory and modifies helm-chart.yaml files to support value overlays",
		RunE: func(cmd *cobra.Command, args []string) error {
			site, err := config.LoadSiteFromFile(sitePath)
			if err != nil {
				return err
			}
			return vendor(site)
		},
	}
	return cmd
}

func vendor(site *config.Site) error {
	// Validate required fields
	if site.Spec.Apps.Base.Source == "" {
		return fmt.Errorf("apps.base.source is required")
	}
	if site.Spec.Apps.Base.Ref == "" {
		return fmt.Errorf("apps.base.ref is required")
	}
	if site.Spec.Apps.Base.Path == "" {
		return fmt.Errorf("apps.base.path is required")
	}
	if site.Spec.Infra.Base.Source == "" {
		return fmt.Errorf("infra.base.source is required")
	}
	if site.Spec.Infra.Base.Ref == "" {
		return fmt.Errorf("infra.base.ref is required")
	}
	if site.Spec.Infra.Base.Path == "" {
		return fmt.Errorf("infra.base.path is required")
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}

	// Validate site name
	if site.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}

	// Vendor apps - each app gets its own base directory
	if err := vendorApps(site); err != nil {
		return err
	}

	// Vendor infra to clusters/{site}/infra/base/
	if err := vendorInfra(site); err != nil {
		return err
	}

	return nil
}

func vendorApps(site *config.Site) error {
	tempDir := filepath.Join(os.TempDir(), "klabctl-vendor-apps-temp")
	defer os.RemoveAll(tempDir)

	// Clone repository
	fmt.Printf("Cloning apps repository: %s (ref: %s)\n", site.Spec.Apps.Base.Source, site.Spec.Apps.Base.Ref)
	cmdClone := exec.Command("git", "clone", "--depth", "1", "--branch", site.Spec.Apps.Base.Ref, site.Spec.Apps.Base.Source, tempDir)
	cmdClone.Stdout = os.Stdout
	cmdClone.Stderr = os.Stderr
	if err := cmdClone.Run(); err != nil {
		return fmt.Errorf("git clone apps failed: %w", err)
	}

	// Get the base apps path
	baseAppsPath := filepath.Join(tempDir, site.Spec.Apps.Base.Path)
	if _, err := os.Stat(baseAppsPath); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist in apps repository", site.Spec.Apps.Base.Path)
	}

	// Vendor each enabled app to its own base directory
	vendoredCount := 0
	for appName, app := range site.Spec.Apps.Catalog {
		if !app.Enabled {
			continue
		}

		// Source: the app in the cloned base repository
		appSourcePath := filepath.Join(baseAppsPath, appName)
		if _, err := os.Stat(appSourcePath); os.IsNotExist(err) {
			fmt.Printf("⚠ Warning: app '%s' not found in base repository, skipping\n", appName)
			continue
		}

		// Destination: clusters/{site}/apps/{app}/base/
		appDestPath := filepath.Join("clusters", site.Metadata.Name, "apps", appName, "base")

		// Remove existing base directory
		if err := os.RemoveAll(appDestPath); err != nil {
			return fmt.Errorf("failed to remove existing base for %s: %w", appName, err)
		}

		// Copy app base
		fmt.Printf("Vendoring %s to %s\n", appName, appDestPath)
		if err := copyDir(appSourcePath, appDestPath); err != nil {
			return fmt.Errorf("failed to copy %s: %w", appName, err)
		}

		// Modify helm-chart.yaml to add additionalValuesFiles
		helmChartPath := filepath.Join(appDestPath, "helm-chart.yaml")
		if _, err := os.Stat(helmChartPath); err == nil {
			// Path to custom values: ../custom/values.yaml
			customValuesPath := "../custom/values.yaml"
			if err := addAdditionalValuesFile(helmChartPath, customValuesPath); err != nil {
				return fmt.Errorf("failed to modify helm-chart.yaml for %s: %w", appName, err)
			}
		}

		vendoredCount++
	}

	fmt.Printf("✓ Successfully vendored %d apps from %s@%s:%s\n", vendoredCount, site.Spec.Apps.Base.Source, site.Spec.Apps.Base.Ref, site.Spec.Apps.Base.Path)
	return nil
}

func vendorInfra(site *config.Site) error {
	tempDir := filepath.Join(os.TempDir(), "klabctl-vendor-infra-temp")
	defer os.RemoveAll(tempDir)

	// Clone repository
	fmt.Printf("Cloning infra repository: %s (ref: %s)\n", site.Spec.Infra.Base.Source, site.Spec.Infra.Base.Ref)
	cmdClone := exec.Command("git", "clone", "--depth", "1", "--branch", site.Spec.Infra.Base.Ref, site.Spec.Infra.Base.Source, tempDir)
	cmdClone.Stdout = os.Stdout
	cmdClone.Stderr = os.Stderr
	if err := cmdClone.Run(); err != nil {
		return fmt.Errorf("git clone infra failed: %w", err)
	}

	// Copy to clusters/{site}/infra/base/
	sourcePath := filepath.Join(tempDir, site.Spec.Infra.Base.Path)
	destPath := filepath.Join("clusters", site.Metadata.Name, "infra", "base")

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist in infra repository", site.Spec.Infra.Base.Path)
	}

	// Remove existing directory
	if err := os.RemoveAll(destPath); err != nil {
		return fmt.Errorf("failed to remove existing infra directory: %w", err)
	}

	// Copy
	fmt.Printf("Copying %s to %s\n", site.Spec.Infra.Base.Path, destPath)
	if err := copyDir(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy infra directory: %w", err)
	}

	fmt.Printf("✓ Successfully vendored infra from %s@%s:%s\n", site.Spec.Infra.Base.Source, site.Spec.Infra.Base.Ref, site.Spec.Infra.Base.Path)
	return nil
}

// addAdditionalValuesFile adds or appends to additionalValuesFiles in a helm-chart.yaml
func addAdditionalValuesFile(helmChartPath, customValuesPath string) error {
	// Read and parse the YAML file
	data, err := os.ReadFile(helmChartPath)
	if err != nil {
		return err
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return err
	}

	// The root node is a Document node, we need the actual map content
	if len(node.Content) == 0 {
		return fmt.Errorf("empty YAML document")
	}

	rootMap := node.Content[0]
	if rootMap.Kind != yaml.MappingNode {
		return fmt.Errorf("expected YAML mapping node")
	}

	// Find additionalValuesFiles key
	var additionalValuesIndex = -1
	for i := 0; i < len(rootMap.Content); i += 2 {
		keyNode := rootMap.Content[i]
		if keyNode.Value == "additionalValuesFiles" {
			additionalValuesIndex = i
			break
		}
	}

	// Check if the custom values path already exists
	if additionalValuesIndex != -1 {
		valuesNode := rootMap.Content[additionalValuesIndex+1]
		if valuesNode.Kind == yaml.SequenceNode {
			for _, item := range valuesNode.Content {
				if item.Value == customValuesPath {
					// Already exists, skip
					return nil
				}
			}
			// Append to existing array
			newItem := &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: customValuesPath,
			}
			valuesNode.Content = append(valuesNode.Content, newItem)
		}
	} else {
		// Create new additionalValuesFiles at the end
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "additionalValuesFiles",
		}
		valueNode := &yaml.Node{
			Kind: yaml.SequenceNode,
			Content: []*yaml.Node{
				{
					Kind:  yaml.ScalarNode,
					Value: customValuesPath,
				},
			},
		}
		rootMap.Content = append(rootMap.Content, keyNode, valueNode)
	}

	// Marshal back to YAML
	output, err := yaml.Marshal(&node)
	if err != nil {
		return err
	}

	// Write back to file
	return os.WriteFile(helmChartPath, output, 0644)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source file info to preserve permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := srcFile.WriteTo(dstFile); err != nil {
		return err
	}

	return nil
}
