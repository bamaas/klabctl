package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/spf13/cobra"
)

func newVendorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendor",
		Short: "Vendor applications and infrastructure base",
		Long:  "Vendors the applications and infrastructure base from the source repository to the base directory",
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
	// Validate required fields for apps
	if site.Spec.Apps.Base.Source == "" {
		return fmt.Errorf("apps.base.source is required")
	}
	if site.Spec.Apps.Base.Ref == "" {
		return fmt.Errorf("apps.base.ref is required")
	}
	if site.Spec.Apps.Base.Path == "" {
		return fmt.Errorf("apps.base.path is required")
	}

	// Validate required fields for infra
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

	// Create a temporary directory for the clone
	tempDir := filepath.Join(os.TempDir(), "klabctl-vendor-temp")
	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("failed to clean temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone apps repository
	appsSource := site.Spec.Apps.Base.Source
	appsRef := site.Spec.Apps.Base.Ref
	appsPath := site.Spec.Apps.Base.Path

	fmt.Printf("Cloning apps repository: %s (ref: %s)\n", appsSource, appsRef)

	cmdClone := exec.Command("git", "clone", "--depth", "1", "--branch", appsRef, appsSource, tempDir)
	cmdClone.Stdout = os.Stdout
	cmdClone.Stderr = os.Stderr
	if err := cmdClone.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Move apps/base to base/apps
	appsSourcePath := filepath.Join(tempDir, appsPath)
	appsDestPath := filepath.Join("base", "apps")

	if _, err := os.Stat(appsSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist in apps repository", appsPath)
	}

	// Remove existing base/apps directory if it exists
	if _, err := os.Stat(appsDestPath); err == nil {
		fmt.Printf("Removing existing directory: %s\n", appsDestPath)
		if err := os.RemoveAll(appsDestPath); err != nil {
			return fmt.Errorf("failed to remove existing apps directory: %w", err)
		}
	}

	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(appsDestPath), 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	fmt.Printf("Moving %s to %s\n", appsPath, appsDestPath)
	if err := copyDir(appsSourcePath, appsDestPath); err != nil {
		return fmt.Errorf("failed to copy apps directory: %w", err)
	}

	fmt.Printf("✓ Successfully vendored apps from %s@%s:%s\n", appsSource, appsRef, appsPath)

	// Handle infra (required)
	infraSource := site.Spec.Infra.Base.Source
	infraRef := site.Spec.Infra.Base.Ref
	infraPath := site.Spec.Infra.Base.Path

	// Clean temp directory for infra clone
	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("failed to clean temp directory: %w", err)
	}

	fmt.Printf("Cloning infra repository: %s (ref: %s)\n", infraSource, infraRef)

	cmdCloneInfra := exec.Command("git", "clone", "--depth", "1", "--branch", infraRef, infraSource, tempDir)
	cmdCloneInfra.Stdout = os.Stdout
	cmdCloneInfra.Stderr = os.Stderr
	if err := cmdCloneInfra.Run(); err != nil {
		return fmt.Errorf("git clone infra failed: %w", err)
	}

	// Move provision/core to base/infra
	infraSourcePath := filepath.Join(tempDir, infraPath)
	infraDestPath := filepath.Join("base", "infra")

	if _, err := os.Stat(infraSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("path '%s' does not exist in infra repository", infraPath)
	}

	// Remove existing base/infra directory if it exists
	if _, err := os.Stat(infraDestPath); err == nil {
		fmt.Printf("Removing existing directory: %s\n", infraDestPath)
		if err := os.RemoveAll(infraDestPath); err != nil {
			return fmt.Errorf("failed to remove existing infra directory: %w", err)
		}
	}

	fmt.Printf("Moving %s to %s\n", infraPath, infraDestPath)
	if err := copyDir(infraSourcePath, infraDestPath); err != nil {
		return fmt.Errorf("failed to copy infra directory: %w", err)
	}

	fmt.Printf("✓ Successfully vendored infra from %s@%s:%s\n", infraSource, infraRef, infraPath)

	return nil
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
