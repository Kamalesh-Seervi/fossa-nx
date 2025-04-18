// Package github provides functionality for creating GitHub issues for vulnerabilities
package github

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/google/go-github/v71/github"
	"github.com/kamalesh-seervi/fossa-nx/internal/models"
	"golang.org/x/oauth2"
)

// CreateIssues creates GitHub issues for vulnerabilities found in scan results
func CreateIssues(results []models.Result, config models.GitHubConfig, verbose bool) error {
	if !config.Enabled {
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
			// Check if issue already exists by searching for title
			issueTitle := fmt.Sprintf("[FOSSA] %s: %s vulnerability in %s",
				vuln.Severity, vuln.Name, result.Project)

			// Skip duplicate issues by searching existing ones
			query := fmt.Sprintf("repo:%s/%s is:issue is:open in:title %s",
				config.Organization, config.Repository, vuln.Name)

			searchResult, _, err := client.Search.Issues(ctx, query, nil)
			if err != nil {
				log.Printf("Error searching for existing issues: %v", err)
				continue
			}

			// Skip if issue already exists
			if searchResult.GetTotal() > 0 {
				if verbose {
					log.Printf("Skipping duplicate issue: %s", issueTitle)
				}
				continue
			}

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

			_, _, err = client.Issues.Create(ctx, config.Organization, config.Repository, issue)
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
