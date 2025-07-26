package nx

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	// Cache project roots to avoid repeated lookups
	projectRootCache = make(map[string]string)
	projectRootMutex sync.RWMutex

	// Cache all projects list
	allProjectsCache  []string
	allProjectsMutex  sync.RWMutex
	allProjectsLoaded bool
)

// GetProjects returns a list of projects based on the provided options
func GetProjects(base, head string, getAllProjects bool) ([]string, error) {
	if getAllProjects {
		// Try to use cached all projects list
		allProjectsMutex.RLock()
		if allProjectsLoaded {
			projects := allProjectsCache
			allProjectsMutex.RUnlock()
			return projects, nil
		}
		allProjectsMutex.RUnlock()

		// Not in cache, need to fetch all projects
		allProjectsMutex.Lock()
		defer allProjectsMutex.Unlock()

		// Double-check after lock acquisition
		if allProjectsLoaded {
			return allProjectsCache, nil
		}

		// Get all projects
		cmd := exec.Command("yarn", "nx", "show", "projects")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to run nx command: %w\nOutput: %s", err, output)
		}

		// Filter empty lines
		rawProjects := strings.Split(strings.TrimSpace(string(output)), "\n")
		projects := make([]string, 0, len(rawProjects))

		for _, p := range rawProjects {
			if p != "" {
				projects = append(projects, p)
			}
		}

		// Cache the result
		allProjectsCache = projects
		allProjectsLoaded = true

		return projects, nil
	} else {
		// Get affected projects
		args := []string{"show", "projects", "--affected", "-t", "build"}

		if base != "" && head != "" {
			args = append(args, fmt.Sprintf("--base=%s", base), fmt.Sprintf("--head=%s", head))
		}

		cmd := exec.Command("yarn", append([]string{"nx"}, args...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to run nx command: %w\nOutput: %s", err, output)
		}

		// Filter empty lines
		rawProjects := strings.Split(strings.TrimSpace(string(output)), "\n")
		projects := make([]string, 0, len(rawProjects))

		for _, p := range rawProjects {
			if p != "" {
				projects = append(projects, p)
			}
		}

		return projects, nil
	}
}

// GetAffectedProjects returns a list of affected projects using nx
// For backward compatibility - now calls the more general GetProjects function
func GetAffectedProjects(base, head string) ([]string, error) {
	return GetProjects(base, head, false)
}

// GetProjectRoot returns the root directory of a project with caching
func GetProjectRoot(projectName string) (string, error) {
	// Check cache first
	projectRootMutex.RLock()
	if root, ok := projectRootCache[projectName]; ok {
		projectRootMutex.RUnlock()
		return root, nil
	}
	projectRootMutex.RUnlock()

	// Not in cache, need to determine project root
	root, err := determineProjectRoot(projectName)
	if err != nil {
		return "", err
	}

	// Store in cache
	projectRootMutex.Lock()
	projectRootCache[projectName] = root
	projectRootMutex.Unlock()

	return root, nil
}

// determineProjectRoot finds the project root using NX or fallback methods
func determineProjectRoot(projectName string) (string, error) {
	// Use NX CLI to directly get project info (most reliable)
	cmd := exec.Command("yarn", "nx", "show", "project", projectName, "--json")
	output, err := cmd.CombinedOutput()
	if err == nil {
		var projectInfo map[string]interface{}
		if err := json.Unmarshal(output, &projectInfo); err == nil {
			if root, ok := projectInfo["root"].(string); ok && root != "" {
				return root, nil
			}
		}
	}

	// Fallback: try common patterns for nx project directories
	sanitizedName := strings.ReplaceAll(projectName, "@", "")
	sanitizedName = strings.ReplaceAll(sanitizedName, "/", "-")

	commonPatterns := []string{
		filepath.Join("apps", sanitizedName),
		filepath.Join("libs", sanitizedName),
		filepath.Join("packages", sanitizedName),
	}

	for _, pattern := range commonPatterns {
		if _, err := os.Stat(pattern); err == nil {
			return pattern, nil
		}
	}

	return "", fmt.Errorf("could not determine project root for %s", projectName)
}

// CreateTemporaryPackageJson creates a temporary package.json with dependencies
func CreateTemporaryPackageJson(projectName, projectRoot string) (string, error) {
	packageJsonPath := filepath.Join(projectRoot, "package.json")

	// Create a basic package.json structure
	packageJSON := map[string]interface{}{
		"name":         projectName,
		"version":      "1.0.0",
		"dependencies": map[string]string{},
	}

	// Get project dependencies using nx show project
	cmd := exec.Command("yarn", "nx", "show", "project", projectName, "--with-deps", "--json")
	depOutput, err := cmd.CombinedOutput()

	if err != nil {
		// If getting dependencies fails, create a minimal package.json
		jsonData, err := json.MarshalIndent(packageJSON, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to serialize package.json: %w", err)
		}

		if err := os.WriteFile(packageJsonPath, jsonData, 0644); err != nil {
			return "", fmt.Errorf("failed to write package.json: %w", err)
		}

		return packageJsonPath, nil
	}

	// Parse dependency information
	var depInfo map[string]interface{}
	if err := json.Unmarshal(depOutput, &depInfo); err == nil {
		// Add dependencies to package.json
		dependencies := packageJSON["dependencies"].(map[string]string)

		// Add all projects that this project depends on
		if deps, ok := depInfo["dependencies"].([]interface{}); ok {
			for _, dep := range deps {
				if depName, ok := dep.(string); ok {
					dependencies[depName] = "^1.0.0"
				}
			}
		}
	}

	// Check for existing package.json to merge with
	if existingData, err := os.ReadFile(packageJsonPath); err == nil {
		var existingPackage map[string]interface{}
		if err := json.Unmarshal(existingData, &existingPackage); err == nil {
			// Merge properties (except dependencies which we handle specially)
			for k, v := range existingPackage {
				if k != "dependencies" {
					packageJSON[k] = v
				}
			}

			// Merge dependencies
			if existingDeps, ok := existingPackage["dependencies"].(map[string]interface{}); ok {
				for k, v := range existingDeps {
					if vStr, ok := v.(string); ok {
						packageJSON["dependencies"].(map[string]string)[k] = vStr
					}
				}
			}
		}
	}

	// Write the final package.json
	jsonData, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize package.json: %w", err)
	}

	if err := os.WriteFile(packageJsonPath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("failed to write package.json: %w", err)
	}

	return packageJsonPath, nil
}

// GetProjectsFromList takes a comma-separated string of project names and returns a list of valid projects
func GetProjectsFromList(projectsList string) ([]string, []string, error) {
	// Get all available projects first
	allProjects, err := GetProjects("", "", true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get all projects: %w", err)
	}

	// Create a map for O(1) lookup of valid projects
	projectMap := make(map[string]bool)
	for _, p := range allProjects {
		projectMap[p] = true
	}

	// Parse comma-separated projects list
	requestedProjects := strings.Split(projectsList, ",")
	validProjects := []string{}
	invalidProjects := []string{}

	for _, project := range requestedProjects {
		trimmedProject := strings.TrimSpace(project)
		if trimmedProject == "" {
			continue // Skip empty project names
		}

		if projectMap[trimmedProject] {
			// Project exists
			validProjects = append(validProjects, trimmedProject)
		} else {
			// Project doesn't exist
			invalidProjects = append(invalidProjects, trimmedProject)
		}
	}

	return validProjects, invalidProjects, nil
}
