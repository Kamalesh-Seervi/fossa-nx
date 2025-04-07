package git

import (
	"os/exec"
	"strings"
	"sync"
)

var (
	// Cache git information to avoid repeated calls
	currentCommit      string
	currentBranch      string
	gitInfoMutex       sync.Mutex
	gitInfoInitialized bool
)

// GetCommitHash returns the current git commit hash
func GetCommitHash() (string, error) {
	initGitInfoOnce()
	return currentCommit, nil
}

// GetBranchName returns the current git branch name
func GetBranchName() (string, error) {
	initGitInfoOnce()
	return currentBranch, nil
}

// Initialize git info only once
func initGitInfoOnce() {
	gitInfoMutex.Lock()
	defer gitInfoMutex.Unlock()

	if gitInfoInitialized {
		return
	}

	// Get commit hash
	gitCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	if output, err := gitCommitCmd.Output(); err == nil {
		currentCommit = strings.TrimSpace(string(output))
	}

	// Get branch name
	gitBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if output, err := gitBranchCmd.Output(); err == nil {
		currentBranch = strings.TrimSpace(string(output))
	}

	gitInfoInitialized = true
}
