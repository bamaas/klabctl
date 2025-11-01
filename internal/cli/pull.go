package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bamaas/klabctl/internal/config"
	"github.com/spf13/cobra"
)

var (
	pullForce         bool
	hiddenKlabctlDir  = filepath.Join(".klabctl")
	stackCacheDirRoot = filepath.Join(hiddenKlabctlDir, "cache", "stack")
)

func newPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull and validate stack cache",
		Long:  "Ensures the stack is cached and valid, pulling or repairing as needed",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load site.yaml to get stack info
			site, err := config.LoadSiteFromFile(sitePath)
			if err != nil {
				return fmt.Errorf("failed to load site.yaml: %w", err)
			}

			if site.Spec.Stack.Source == "" || site.Spec.Stack.Ref == "" {
				return fmt.Errorf("stack.source and stack.ref are required in site.yaml")
			}

			return EnsureStackAvailable(site.Spec.Stack.Source, site.Spec.Stack.Ref, pullForce)
		},
	}

	cmd.Flags().BoolVar(&pullForce, "force", false, "Force re-pull stack even if cached")

	return cmd
}

func createHiddenKlabctlDir() error {
	// Create hidden klabctl directory
	if _, err := os.Stat(hiddenKlabctlDir); os.IsNotExist(err) {
		if err := os.MkdirAll(hiddenKlabctlDir, 0755); err != nil {
			return err
		}
	}

	// Create .gitignore file
	outputPath := filepath.Join(hiddenKlabctlDir, ".gitignore")
	// Don't overwrite existing .gitignore
	if _, err := os.Stat(outputPath); err == nil {
		return nil
	}

	gitignoreContent := `# Created by klabctl - Ignore all files in the cache directory.
*
`

	if err := os.WriteFile(outputPath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	return nil
}

// getStackCacheDir returns the path to the cached stack directory
func getStackCacheDir(site *config.Site) string {
	return filepath.Join(stackCacheDirRoot, site.Spec.Stack.Ref)
}

// getStackTemplatesDir returns the path to the stack templates directory in cache
func getStackTemplatesDir(site *config.Site) string {
	return filepath.Join(getStackCacheDir(site), "stack", "templates")
}

// getStackAppsDir returns the path to the stack apps directory in cache
func getStackAppsDir(site *config.Site) string {
	return filepath.Join(getStackCacheDir(site), "stack", "apps")
}

// isGitRepo checks if a directory is a git repository
func isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}
	return false
}

// getCachedVersion returns the current git version (branch/tag/commit) of the cache
func getCachedVersion(stackDir string) (string, error) {
	if !isGitRepo(stackDir) {
		return "", fmt.Errorf("not a git repository")
	}

	// Get current branch or tag
	cmd := exec.Command("git", "-C", stackDir, "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))

	// If detached HEAD, get commit SHA
	if branch == "HEAD" {
		cmd = exec.Command("git", "-C", stackDir, "rev-parse", "--short", "HEAD")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get git commit: %w", err)
		}
		return strings.TrimSpace(string(output)), nil
	}

	return branch, nil
}

// isValidCache validates the cache integrity using git
func isValidCache(stackDir string) bool {
	if !isGitRepo(stackDir) {
		return false
	}

	// Check git status - are files modified/missing?
	cmd := exec.Command("git", "-C", stackDir, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If output is not empty, working tree has modifications
	if len(strings.TrimSpace(string(output))) > 0 {
		return false
	}

	// Check required stack directories exist
	requiredPaths := []string{
		filepath.Join(stackDir, "stack"),
		filepath.Join(stackDir, "stack/apps"),
		filepath.Join(stackDir, "stack/templates"),
	}

	for _, path := range requiredPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// repairCache attempts to repair a corrupted cache using git
func repairCache(stackDir string) error {
	if !isGitRepo(stackDir) {
		return fmt.Errorf("not a git repository")
	}

	fmt.Fprintln(os.Stderr, "🔧 Repairing cache...")

	// Reset to clean state
	cmd := exec.Command("git", "-C", stackDir, "reset", "--hard", "HEAD")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}

	// Clean untracked files
	cmd = exec.Command("git", "-C", stackDir, "clean", "-fd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clean failed: %w", err)
	}

	fmt.Fprintln(os.Stderr, "✓ Cache repaired")
	return nil
}

// updateGitRepo updates an existing git repository to a specific version
func updateGitRepo(dir, version string) error {
	// Fetch latest
	fmt.Fprintln(os.Stderr, "Fetching updates...")
	cmd := exec.Command("git", "-C", dir, "fetch", "origin")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	// Checkout requested version
	cmd = exec.Command("git", "-C", dir, "checkout", version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}

	// Pull if it's a branch (ignore errors if it's a tag/commit)
	cmd = exec.Command("git", "-C", dir, "pull", "--ff-only")
	cmd.Run()

	return nil
}

// pullStack clones the stack repository to the cache directory
func pullStack(source, version, destDir string) error {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}

	// Remove existing directory if it exists
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to remove existing cache: %w", err)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Clone repository
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", version, source, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Keep .git directory for cache validation and updates

	return nil
}

// EnsureStackAvailable ensures the stack is cached and valid, pulling/repairing as needed
// This is the main function that implements the "always validate" strategy
func EnsureStackAvailable(source, ref string, force bool) error {

	if err := createHiddenKlabctlDir(); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", hiddenKlabctlDir, err)
	}

	stackCacheDir := filepath.Join(stackCacheDirRoot, ref)

	// Handle force flag - remove cache if force is requested
	if force {
		fmt.Fprintln(os.Stderr, "Force re-pulling stack...")
		if err := os.RemoveAll(stackCacheDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove cache: %v\n", err)
		}
		// After force removal, proceed with normal flow (force is now done)
	}

	// Check if directory exists
	if _, err := os.Stat(stackCacheDir); os.IsNotExist(err) {
		// Cache doesn't exist - clone it
		fmt.Fprintf(os.Stderr, "📦 Pulling stack %s@%s...\n", source, ref)
		if err := pullStack(source, ref, stackCacheDir); err != nil {
			return fmt.Errorf("failed to pull stack: %w", err)
		}
		fmt.Fprintln(os.Stderr, "✓ Stack pulled successfully")
		return nil
	}

	// Cache exists - validate it
	if !isGitRepo(stackCacheDir) {
		// Not a git repo (corrupted) - remove and re-clone
		fmt.Fprintln(os.Stderr, "⚠ Cache is not a git repository, re-pulling...")
		if err := os.RemoveAll(stackCacheDir); err != nil {
			return fmt.Errorf("failed to remove invalid cache: %w", err)
		}
		return EnsureStackAvailable(source, ref, false)
	}

	// Check current version
	currentRef, err := getCachedVersion(stackCacheDir)
	if err != nil {
		// Can't determine ref (corrupted) - re-clone
		fmt.Fprintf(os.Stderr, "⚠ Cannot determine cache ref: %v\n", err)
		fmt.Fprintln(os.Stderr, "⚠ Re-pulling stack...")
		if err := os.RemoveAll(stackCacheDir); err != nil {
			return fmt.Errorf("failed to remove invalid cache: %w", err)
		}
		return EnsureStackAvailable(source, ref, false)
	}

	// Already on correct version?
	if currentRef == ref {
		// Validate integrity
		if !isValidCache(stackCacheDir) {
			fmt.Fprintln(os.Stderr, "⚠ Cache is corrupted or has modifications")
			// Try to repair
			if err := repairCache(stackCacheDir); err != nil {
				// Repair failed - re-clone
				fmt.Fprintf(os.Stderr, "⚠ Repair failed: %v\n", err)
				fmt.Fprintln(os.Stderr, "⚠ Re-pulling stack...")
				if err := os.RemoveAll(stackCacheDir); err != nil {
					return fmt.Errorf("failed to remove invalid cache: %w", err)
				}
				return EnsureStackAvailable(source, ref, false)
			}
		}

		fmt.Fprintf(os.Stderr, "✓ Using cached stack %s\n", ref)
		return nil
	}

	// Different version - switch to requested version
	fmt.Fprintf(os.Stderr, "Switching cache from %s to %s...\n", currentRef, ref)
	if err := updateGitRepo(stackCacheDir, ref); err != nil {
		// Update failed - re-clone
		fmt.Fprintf(os.Stderr, "⚠ Version switch failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "⚠ Re-pulling stack...")
		if err := os.RemoveAll(stackCacheDir); err != nil {
			return fmt.Errorf("failed to remove invalid cache: %w", err)
		}
		return EnsureStackAvailable(source, ref, false)
	}

	// Validate after switching
	if !isValidCache(stackCacheDir) {
		fmt.Fprintln(os.Stderr, "⚠ Cache invalid after version switch")
		if err := repairCache(stackCacheDir); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ Repair failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "⚠ Re-pulling stack...")
			if err := os.RemoveAll(stackCacheDir); err != nil {
				return fmt.Errorf("failed to remove invalid cache: %w", err)
			}
			return EnsureStackAvailable(source, ref, false)
		}
	}

	fmt.Fprintln(os.Stderr, "✓ Cache switched and validated")

	return nil
}
