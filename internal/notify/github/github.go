// Package github provides functionality for creating GitHub issues for vulnerabilities
package github

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/google/go-github/v71/github"
	"github.com/kamalesh-seervi/fossa-nx/internal/git"
	"github.com/kamalesh-seervi/fossa-nx/internal/models"
	"golang.org/x/oauth2"
)

// CreateIssues creates GitHub issues for vulnerabilities found in scan results
func CreateIssues(results []models.Result, config models.GitHubConfig, verbose bool) error {
	if !config.Enabled {
		return nil
	}

	// Check if issue creation is disabled
	if !config.CreateIssues {
		if verbose {
			log.Println("GitHub issue creation is disabled, skipping vulnerability issue creation")
		}
		return nil
	}

	// Count total vulnerabilities
	var totalIssues int
	for _, result := range results {
		totalIssues += len(result.Issues)
	}

	if totalIssues == 0 {
		if verbose {
			log.Println("No vulnerabilities found, skipping GitHub issue creation")
		}
		return nil
	}

	// Create GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	// Initialize GitHub client
	client := github.NewClient(tc)

	// Configure Enterprise GitHub URL if provided
	if config.ApiUrl != "" {
		if verbose {
			log.Printf("Using custom GitHub API URL: %s", config.ApiUrl)
		}

		// Create API endpoint
		baseURL, err := url.Parse(config.ApiUrl + "/")
		if err != nil {
			return fmt.Errorf("invalid GitHub API URL: %v", err)
		}

		// Set custom API URL for GitHub client
		client.BaseURL = baseURL

		// For GitHub Enterprise, upload URL is usually the same as API URL
		client.UploadURL = baseURL
	}

	if verbose {
		log.Printf("Creating GitHub issues for %d vulnerabilities", totalIssues)
	}

	// Track created issues to avoid duplicates
	issuesCreated := 0

	// Process each result
	for _, result := range results {
		if result.Error != nil || len(result.Issues) == 0 {
			continue
		}

		// Create issues for each vulnerability
		for _, vuln := range result.Issues {
			// Create issue title
			issueTitle := fmt.Sprintf("[FOSSA] %s: %s vulnerability in %s",
				vuln.Severity, vuln.Name, result.Project)

			// Create issue with detailed information
			var cveInfo string
			if vuln.CVE != "" {
				cveInfo = fmt.Sprintf("**CVE:** %s", vuln.CVE)
			}

			issueBody := fmt.Sprintf(`
## FOSSA Vulnerability Report

**Project:** %s  
**Vulnerability:** %s  
**Severity:** %s

### Description
%s

%s

### Vulnerability Details
`, result.Project, vuln.Name, vuln.Severity, vuln.Description, cveInfo)

			// Add fixed version info if available
			if vuln.FixedIn != "" {
				issueBody += fmt.Sprintf("\n**Fixed in:** %s", vuln.FixedIn)
			}

			// Add discovery date
			if !vuln.FirstSeen.IsZero() {
				issueBody += fmt.Sprintf("\n**First discovered:** %s", vuln.FirstSeen.Format("Jan 2, 2006"))
			}

			// Add links
			issueBody += fmt.Sprintf(`

### Links
- [View in FOSSA](%s)
- [Vulnerability details](%s)

---
*This issue was automatically created by fossa-nx*
`, result.FossaLink, vuln.Link)

			// Create labels based on severity
			labels := []string{"security", "fossa", "vulnerability"}
			switch strings.ToLower(vuln.Severity) {
			case "high", "critical":
				labels = append(labels, "severity:high")
			case "medium", "moderate":
				labels = append(labels, "severity:medium")
			case "low":
				labels = append(labels, "severity:low")
			}

			issue := &github.IssueRequest{
				Title:  &issueTitle,
				Body:   &issueBody,
				Labels: &labels,
			}

			_, _, err := client.Issues.Create(ctx, config.Organization, config.Repository, issue)
			if err != nil {
				log.Printf("Error creating GitHub issue: %v", err)
				continue
			}

			issuesCreated++
			if verbose {
				log.Printf("Created GitHub issue: %s", issueTitle)
			}
		}
	}

	if verbose {
		log.Printf("Created %d GitHub issues", issuesCreated)
	}

	return nil
}

// CreateCommitStatus creates a commit status check for FOSSA scan results
func CreateCommitStatus(results []models.Result, config models.GitHubConfig, verbose bool) error {
	if !config.Enabled {
		return nil
	}

	// Get current git commit hash
	commitHash, err := git.GetCommitHash()
	if err != nil || commitHash == "" {
		if verbose {
			log.Printf("Could not get git commit hash, skipping commit status update: %v", err)
		}
		return nil
	}

	// Create GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Configure Enterprise GitHub URL if provided
	if config.ApiUrl != "" {
		baseURL, err := url.Parse(config.ApiUrl + "/")
		if err != nil {
			return fmt.Errorf("invalid GitHub API URL: %v", err)
		}
		client.BaseURL = baseURL
		client.UploadURL = baseURL
	}

	// Determine overall status based on scan results
	var totalVulnerabilities int
	var failedScans int
	var projectNames []string
	var failedProjects []string

	for _, result := range results {
		projectNames = append(projectNames, result.Project)
		if result.Error != nil {
			failedScans++
			failedProjects = append(failedProjects, result.Project)
		} else {
			totalVulnerabilities += len(result.Issues)
		}
	}

	// Escape project names for use in status context
	escapedProjectName := strings.Join(projectNames, "-")
	escapedProjectName = strings.ReplaceAll(escapedProjectName, " ", "-")
	escapedProjectName = strings.ReplaceAll(escapedProjectName, "/", "-")

	// Truncate context to fit GitHub's 255 character limit
	context := fmt.Sprintf("ci/fossa-%s", escapedProjectName)
	if len(context) > 255 {
		// If too long, use a generic context or truncate
		if len(projectNames) == 1 {
			context = fmt.Sprintf("ci/fossa-%s", projectNames[0])
			if len(context) > 255 {
				context = "ci/fossa-scan"
			}
		} else {
			context = fmt.Sprintf("ci/fossa-%d-projects", len(projectNames))
		}
	}

	// Determine status state and description
	var state string
	var description string

	if failedScans > 0 {
		state = "failure"
		if len(failedProjects) == 1 {
			appName := failedProjects[0]
			if len(appName) > 80 { // Leave room for prefix text
				appName = appName[:80] + "..."
			}
			description = fmt.Sprintf("FOSSA scan failed for %s", appName)
		} else {
			// Show first few failed projects if space allows
			failedList := strings.Join(failedProjects, ", ")
			if len(failedList) > 100 {
				description = fmt.Sprintf("FOSSA scan failed for %d projects", len(failedProjects))
			} else {
				description = fmt.Sprintf("FOSSA scan failed: %s", failedList)
			}
		}
	} else if totalVulnerabilities > 0 {
		state = "failure"
		if len(projectNames) == 1 {
			appName := projectNames[0]
			if len(appName) > 70 { // Leave room for vulnerability count
				appName = appName[:70] + "..."
			}
			description = fmt.Sprintf("FOSSA: %d vuln(s) in %s", totalVulnerabilities, appName)
		} else {
			description = fmt.Sprintf("FOSSA found %d vulnerabilities across %d projects", totalVulnerabilities, len(projectNames))
		}
	} else {
		state = "success"
		if len(projectNames) == 1 {
			projectName := projectNames[0]
			if len(projectName) > 100 { // Leave room for prefix text
				projectName = projectName[:100] + "..."
			}
			description = fmt.Sprintf("FOSSA scan passed for %s", projectName)
		} else {
			description = fmt.Sprintf("FOSSA scan passed for %d projects", len(projectNames))
		}
	}

	// Ensure description doesn't exceed GitHub's 140 character limit
	if len(description) > 140 {
		description = description[:137] + "..."
	}

	// Create status request
	status := &github.RepoStatus{
		State:       github.Ptr(state),
		Description: github.Ptr(description),
		Context:     github.Ptr(context),
	}

	// Create the commit status
	_, _, err = client.Repositories.CreateStatus(ctx, config.Organization, config.Repository, commitHash, status)
	if err != nil {
		return fmt.Errorf("error creating commit status: %v", err)
	}

	if verbose {
		log.Printf("Created commit status: %s - %s (commit: %s)", state, description, commitHash[:8])
	}

	return nil
}
