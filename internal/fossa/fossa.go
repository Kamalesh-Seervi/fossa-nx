package fossa

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kamalesh-seervi/fossa-nx/internal/mapping"
	"github.com/kamalesh-seervi/fossa-nx/internal/nx"
)

// Global mutex for filesystem operations to prevent race conditions
var (
	fsLock           sync.Mutex
	monorepoRootOnce sync.Once
	monorepoRootDir  string
	monorepoRootErr  error
	filteredEnvOnce  sync.Once
	filteredEnvVars  []string
)

// getMonorepoRoot returns the root directory of the monorepo (cached)
func getMonorepoRoot() (string, error) {
	monorepoRootOnce.Do(func() {
		var err error
		monorepoRootDir, err = os.Getwd()
		monorepoRootErr = err
	})
	return monorepoRootDir, monorepoRootErr
}

// getFilteredEnv returns environment variables without SSL_CERT_DIR (cached)
func getFilteredEnv() []string {
	filteredEnvOnce.Do(func() {
		env := os.Environ()
		filteredEnvVars = make([]string, 0, len(env))

		for _, envVar := range env {
			if !strings.HasPrefix(envVar, "SSL_CERT_DIR=") {
				filteredEnvVars = append(filteredEnvVars, envVar)
			}
		}
	})
	return filteredEnvVars
}

// RunAnalysis runs FOSSA analysis for a project with optimized performance
func RunAnalysis(projectName string) error {
	// Verify project is mapped in config
	if !mapping.IsProjectMapped(projectName) {
		return fmt.Errorf("project %s is not mapped in configuration", projectName)
	}

	// Get project root (cached)
	projectRoot, err := nx.GetProjectRoot(projectName)
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	// Get absolute path to project root
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get monorepo root (cached)
	monorepoRoot, err := getMonorepoRoot()
	if err != nil {
		return fmt.Errorf("failed to get monorepo root: %w", err)
	}

	// State tracking variables
	var (
		packageJsonPath         = filepath.Join(absProjectRoot, "package.json")
		originalPackageJson     []byte
		packageJsonExists       = false
		nodeModulesPath         = filepath.Join(absProjectRoot, "node_modules")
		monorepoNodeModulesPath = filepath.Join(monorepoRoot, "node_modules")
		nodeModulesCreated      = false
	)

	// Backup package.json if it exists
	fsLock.Lock()
	if _, err := os.Stat(packageJsonPath); err == nil {
		packageJsonExists = true
		originalPackageJson, err = os.ReadFile(packageJsonPath)
		if err != nil {
			fsLock.Unlock()
			return fmt.Errorf("failed to read package.json: %w", err)
		}
	}
	fsLock.Unlock()

	// Create or update package.json with dependencies (with lock)
	fsLock.Lock()
	_, err = nx.CreateTemporaryPackageJson(projectName, absProjectRoot)
	fsLock.Unlock()

	if err != nil {
		return fmt.Errorf("failed to create temporary package.json: %w", err)
	}

	// Check if node_modules symlink is needed
	fsLock.Lock()
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		if err := os.Symlink(monorepoNodeModulesPath, nodeModulesPath); err != nil {
			fsLock.Unlock()
			return fmt.Errorf("failed to create node_modules symlink: %w", err)
		}
		nodeModulesCreated = true
	}
	fsLock.Unlock()

	// Get team value and project ID for FOSSA from config (cached)
	teamValue := mapping.GetTeamValue(projectName)
	fossaProject := mapping.GetFossaProjectID(projectName)
	fossaEndpoint := mapping.GetFossaEndpoint()

	// Get current git commit information
	gitCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	gitCommitOutput, err := gitCommitCmd.Output()
	gitCommitHash := ""
	if err == nil {
		gitCommitHash = strings.TrimSpace(string(gitCommitOutput))
	}

	gitBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	gitBranchOutput, err := gitBranchCmd.Output()
	gitBranchName := ""
	if err == nil {
		gitBranchName = strings.TrimSpace(string(gitBranchOutput))
	}

	// Ensure cleanup of temporary resources when done
	defer func() {
		fsLock.Lock()
		// Restore original package.json
		if packageJsonExists {
			os.WriteFile(packageJsonPath, originalPackageJson, 0644)
		} else {
			os.Remove(packageJsonPath)
		}

		// Remove node_modules symlink if we created it
		if nodeModulesCreated {
			os.Remove(nodeModulesPath)
		}
		fsLock.Unlock()
	}()

	// Get filtered environment variables (cached)
	filteredEnv := getFilteredEnv()

	// Run FOSSA analyze command with optimized command construction
	analyzeArgs := []string{
		"analyze",
		"-e", fossaEndpoint,
		"-T", teamValue,
		"-p", fossaProject,
	}

	// Only add branch and revision if available
	if gitBranchName != "" {
		analyzeArgs = append(analyzeArgs, "-b", gitBranchName)
	}

	if gitCommitHash != "" {
		analyzeArgs = append(analyzeArgs, "-r", gitCommitHash)
	}

	analyzeArgs = append(analyzeArgs, "--policy", "Website/Hosted Service Use")

	analyzeCmd := exec.Command("fossa", analyzeArgs...)
	analyzeCmd.Dir = absProjectRoot
	analyzeCmd.Env = filteredEnv
	analyzeCmd.Stdout = os.Stdout
	analyzeCmd.Stderr = os.Stderr

	if err := analyzeCmd.Run(); err != nil {
		return fmt.Errorf("FOSSA analyze command failed: %w", err)
	}

	// Run FOSSA test command with minimal arguments
	testArgs := []string{
		"test",
		"-e", fossaEndpoint,
		"-p", fossaProject,
	}

	if gitCommitHash != "" {
		testArgs = append(testArgs, "-r", gitCommitHash)
	}

	testCmd := exec.Command("fossa", testArgs...)
	testCmd.Dir = absProjectRoot
	testCmd.Env = filteredEnv
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("FOSSA test command failed: %w", err)
	}

	return nil
}
