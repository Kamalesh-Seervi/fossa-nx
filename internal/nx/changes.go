package nx

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Cache for project graph and changed files
var (
	projectGraphCache map[string]ProjectNode
	projectGraphOnce  sync.Once
	changedFilesCache []string
	changedFilesOnce  sync.Once
)

// ProjectNode represents a project in the NX workspace
type ProjectNode struct {
	Name string `json:"name"`
	Data struct {
		Root string `json:"root"`
	} `json:"data"`
}

// ProjectGraph represents the NX project graph structure
type ProjectGraph struct {
	Nodes map[string]ProjectNode `json:"nodes"`
}

// GetChangedFiles returns files that changed between base and head commits
func GetChangedFiles(base, head string) ([]string, error) {
	// In CI environments, we typically run with the same base/head for the entire process
	// so simple sync.Once caching is sufficient
	var err error
	changedFilesOnce.Do(func() {
		// Build git diff command
		var cmd *exec.Cmd
		if base != "" && head != "" {
			cmd = exec.Command("git", "diff", "--name-only", fmt.Sprintf("%s..%s", base, head))
		} else if base != "" {
			cmd = exec.Command("git", "diff", "--name-only", base)
		} else {
			// Default to uncommitted changes
			cmd = exec.Command("git", "diff", "--name-only", "HEAD")
		}

		output, cmdErr := cmd.Output()
		if cmdErr != nil {
			err = fmt.Errorf("failed to get changed files: %w", cmdErr)
			return
		}

		outputStr := strings.TrimSpace(string(output))

		// If no changes, return empty slice immediately
		if outputStr == "" {
			changedFilesCache = []string{}
			return
		}

		changedFiles := []string{}
		lines := strings.Split(outputStr, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				// Normalize path separators for cross-platform compatibility
				changedFiles = append(changedFiles, strings.ReplaceAll(line, "\\", "/"))
			}
		}

		changedFilesCache = changedFiles
	})

	if err != nil {
		return nil, err
	}

	// Return a copy to prevent external modifications
	result := make([]string, len(changedFilesCache))
	copy(result, changedFilesCache)
	return result, nil
}

// GetProjectGraph returns the NX project graph (cached)
func GetProjectGraph() (map[string]ProjectNode, error) {
	var err error
	projectGraphOnce.Do(func() {
		// Create a temporary file for cross-platform compatibility
		tempFile, tempErr := os.CreateTemp("", "nx_graph_*.json")
		if tempErr != nil {
			err = fmt.Errorf("failed to create temporary file: %w", tempErr)
			return
		}
		defer func() {
			tempFile.Close()
			os.Remove(tempFile.Name()) // Ensure the file is cleaned up
		}()

		// Try nx command first (newer versions)
		cmd := exec.Command("yarn", "nx", "graph", fmt.Sprintf("--file=%s", tempFile.Name()), "--format=json")
		cmdErr := cmd.Run()

		if cmdErr != nil {
			// Fallback to older nx command
			cmd = exec.Command("yarn", "nx", "dep-graph", fmt.Sprintf("--file=%s", tempFile.Name()), "--format=json")
			cmdErr = cmd.Run()

			if cmdErr != nil {
				err = fmt.Errorf("failed to get project graph: %w", cmdErr)
				return
			}
		}

		// Read the generated file
		output, readErr := os.ReadFile(tempFile.Name())
		if readErr != nil {
			err = fmt.Errorf("failed to read project graph file: %w", readErr)
			return
		}

		var graph ProjectGraph
		if jsonErr := json.Unmarshal(output, &graph); jsonErr != nil {
			err = fmt.Errorf("failed to parse project graph: %w", jsonErr)
			return
		}

		projectGraphCache = graph.Nodes
	})

	return projectGraphCache, err
}

// GetChangedProjectsUsingGraph returns projects that have file changes between commits
func GetChangedProjectsUsingGraph(base, head string) ([]string, error) {
	// Get changed files
	changedFiles, err := GetChangedFiles(base, head)
	if err != nil {
		return nil, err
	}

	if len(changedFiles) == 0 {
		return []string{}, nil
	}

	// Get project graph
	projectGraph, err := GetProjectGraph()
	if err != nil {
		return nil, err
	}

	// Map changed files to projects
	changedProjectsSet := make(map[string]bool)

	for _, node := range projectGraph {
		projectRoot := strings.ReplaceAll(node.Data.Root, "\\", "/")

		for _, file := range changedFiles {
			// Check if file is in this project's directory
			if strings.HasPrefix(file, projectRoot+"/") || file == projectRoot {
				changedProjectsSet[node.Name] = true
				break
			}
		}
	}

	// Convert set to slice
	changedProjects := make([]string, 0, len(changedProjectsSet))
	for project := range changedProjectsSet {
		changedProjects = append(changedProjects, project)
	}

	return changedProjects, nil
}

// ShouldSkipProject returns true if the project has no changes and should be skipped
func ShouldSkipProject(projectName, base, head string, forceAll bool) (bool, error) {
	// If forcing all projects, don't skip
	if forceAll {
		return false, nil
	}

	// If no base/head specified, don't skip (analyze all)
	if base == "" && head == "" {
		return false, nil
	}

	// Get changed projects
	changedProjects, err := GetChangedProjectsUsingGraph(base, head)
	if err != nil {
		// If we can't determine changes, err on the side of running the scan
		return false, nil
	}

	// Check if this project is in the changed list
	for _, changed := range changedProjects {
		if changed == projectName {
			return false, nil // Don't skip, project has changes
		}
	}

	// Project not in changed list, can skip
	return true, nil
}
