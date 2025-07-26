package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalesh-seervi/fossa-nx/internal/fossa"
	"github.com/kamalesh-seervi/fossa-nx/internal/mapping"
	"github.com/kamalesh-seervi/fossa-nx/internal/models"
	"github.com/kamalesh-seervi/fossa-nx/internal/notify/email"
	"github.com/kamalesh-seervi/fossa-nx/internal/notify/github"
	"github.com/kamalesh-seervi/fossa-nx/internal/nx"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Stats for tracking execution metrics
type Stats struct {
	totalProjects   int32
	successful      int32
	failed          int32
	vulnerabilities int32
	totalDuration   int64 // nanoseconds
	maxDuration     int64 // nanoseconds
	minDuration     int64 // nanoseconds (initialized to a large value)
	mutex           sync.Mutex
}

func (s *Stats) initialize(projectCount int) {
	atomic.StoreInt32(&s.totalProjects, int32(projectCount))
	atomic.StoreInt32(&s.successful, 0)
	atomic.StoreInt32(&s.failed, 0)
	atomic.StoreInt64(&s.totalDuration, 0)
	atomic.StoreInt64(&s.maxDuration, 0)
	s.mutex.Lock()
	s.minDuration = int64(time.Hour) // Initialize to a large value
	s.mutex.Unlock()
}

func (s *Stats) recordResult(success bool, duration time.Duration, vulnCount int) {
	durationNanos := duration.Nanoseconds()

	if success {
		atomic.AddInt32(&s.successful, 1)
	} else {
		atomic.AddInt32(&s.failed, 1)
	}

	if vulnCount > 0 {
		atomic.AddInt32(&s.vulnerabilities, int32(vulnCount))
	}

	atomic.AddInt64(&s.totalDuration, durationNanos)

	// Update max duration (atomic compare-and-swap)
	for {
		old := atomic.LoadInt64(&s.maxDuration)
		if durationNanos <= old {
			break
		}
		if atomic.CompareAndSwapInt64(&s.maxDuration, old, durationNanos) {
			break
		}
	}

	// Update min duration (with mutex for simplicity)
	s.mutex.Lock()
	if durationNanos < s.minDuration {
		s.minDuration = durationNanos
	}
	s.mutex.Unlock()
}

func (s *Stats) print() {
	successful := atomic.LoadInt32(&s.successful)
	failed := atomic.LoadInt32(&s.failed)
	total := atomic.LoadInt32(&s.totalProjects)
	vulnCount := atomic.LoadInt32(&s.vulnerabilities)
	totalDuration := time.Duration(atomic.LoadInt64(&s.totalDuration))

	s.mutex.Lock()
	minDuration := time.Duration(s.minDuration)
	s.mutex.Unlock()

	maxDuration := time.Duration(atomic.LoadInt64(&s.maxDuration))

	avgDuration := time.Duration(0)
	if successful+failed > 0 {
		avgDuration = totalDuration / time.Duration(successful+failed)
	}

	log.Printf("FOSSA Analysis Stats:")
	log.Printf("  Total Projects: %d", total)
	log.Printf("  Successful: %d", successful)
	log.Printf("  Failed: %d", failed)
	log.Printf("  Vulnerabilities Found: %d", vulnCount)

	// Display duration in minutes if > 60 seconds, otherwise show in seconds
	if avgDuration.Seconds() > 60.0 {
		log.Printf("  Average Duration: %.2f minutes", avgDuration.Minutes())
	} else {
		log.Printf("  Average Duration: %.2f seconds", avgDuration.Seconds())
	}

	if successful+failed > 0 {
		if minDuration.Seconds() > 60.0 {
			log.Printf("  Min Duration: %.2f minutes", minDuration.Minutes())
		} else {
			log.Printf("  Min Duration: %.2f seconds", minDuration.Seconds())
		}

		if maxDuration.Seconds() > 60.0 {
			log.Printf("  Max Duration: %.2f minutes", maxDuration.Minutes())
		} else {
			log.Printf("  Max Duration: %.2f seconds", maxDuration.Seconds())
		}
	}
}

func main() {
	var (
		base            string
		head            string
		verboseLogging  bool
		maxConcurrent   int
		timeout         int
		configPath      string
		cpuProfile      string
		memProfile      string
		allProjects     bool
		includeUnmapped bool
		projectName     string // Add specific project option

		// Email configuration
		emailEnabled bool
		smtpServer   string
		smtpPort     int
		smtpUser     string
		smtpPassword string
		fromEmail    string
		toEmails     string

		// GitHub configuration
		githubEnabled bool
		githubToken   string
		githubOrg     string
		githubRepo    string
		githubApiUrl  string
	)

	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Printf("fossa-nx version %s (%s) built on %s\n", version, commit, date)
		os.Exit(0)
	}

	// Initialize stats tracking
	stats := &Stats{}

	rootCmd := &cobra.Command{
		Use:   "fossa-nx",
		Short: "High-performance FOSSA license scanning for NX monorepos",
		Long: `A CLI tool to help developers run FOSSA license analysis efficiently on NX monorepo projects.
        
By default, only affected projects that are mapped in the configuration are analyzed.
Use the --all flag to analyze all mapped projects.
Use the --include-unmapped flag to include projects not defined in configuration.
Use the --project flag to analyze a specific project by name.

Examples:
  fossa-nx --base=develop --head=feature-branch  # Analyze affected mapped projects
  fossa-nx --all                                # Analyze all mapped projects
  fossa-nx --all --include-unmapped             # Analyze all projects, even unmapped ones
  fossa-nx --project=my-app                     # Analyze a specific project
`,
		Run: func(cmd *cobra.Command, args []string) {
			if verboseLogging {
				log.Println("Running FOSSA analysis on projects...")
				if projectName != "" {
					if strings.Contains(projectName, ",") {
						log.Printf("Analyzing specific projects: %s\n", projectName)
					} else {
						log.Printf("Analyzing specific project: %s\n", projectName)
					}
				} else if !allProjects && base != "" && head != "" {
					log.Printf("Using base: %s and head: %s\n", base, head)
				} else if allProjects {
					log.Println("Analyzing ALL mapped projects (not just affected ones)")
				}
				if includeUnmapped {
					log.Println("Including projects not defined in configuration")
				}
			}

			// Parse email recipients
			recipientList := []string{}
			if toEmails != "" {
				recipientList = email.ParseEmailList(toEmails)
				if verboseLogging {
					log.Printf("Will send notifications to %d recipients", len(recipientList))
				}
			}

			// Setup notification services
			emailConfig := models.EmailConfig{
				SmtpServer:   smtpServer,
				SmtpPort:     smtpPort,
				SmtpUser:     smtpUser,
				SmtpPassword: smtpPassword,
				FromEmail:    fromEmail,
				ToEmails:     recipientList,
				Enabled:      emailEnabled && len(recipientList) > 0,
			}

			githubConfig := models.GitHubConfig{
				Token:        githubToken,
				Organization: githubOrg,
				Repository:   githubRepo,
				ApiUrl:       githubApiUrl,
				Enabled:      githubEnabled && githubToken != "",
			}

			var projects []string
			startTime := time.Now()

			// Check for mutually exclusive flags
			if projectName != "" && (allProjects || (base != "" || head != "")) {
				log.Fatalf("Error: --project flag cannot be used with --all, --base, or --head")
			}

			// Handle project-specific mode
			if projectName != "" {
				// Check if it's a comma-separated list
				if strings.Contains(projectName, ",") {
					validProjects, invalidProjects, err := nx.GetProjectsFromList(projectName)
					if err != nil {
						log.Fatalf("Error getting projects: %v", err)
					}

					if len(invalidProjects) > 0 {
						log.Printf("Warning: The following projects were not found and will be skipped: %v", invalidProjects)
					}

					if len(validProjects) == 0 {
						log.Fatalf("No valid projects found in the list: %s", projectName)
					}

					// Filter unmapped projects if needed
					if !includeUnmapped {
						var mappedProjects []string
						var unmappedProjects []string

						for _, project := range validProjects {
							if mapping.IsProjectMapped(project) {
								mappedProjects = append(mappedProjects, project)
							} else {
								unmappedProjects = append(unmappedProjects, project)
							}
						}

						if len(unmappedProjects) > 0 && verboseLogging {
							log.Printf("Skipping unmapped projects: %v. Use --include-unmapped to include them.", unmappedProjects)
						}

						if len(mappedProjects) == 0 {
							log.Fatalf("No mapped projects found in the list. Use --include-unmapped to include unmapped projects.")
						}

						projects = mappedProjects
					} else {
						projects = validProjects
					}

					if verboseLogging {
						log.Printf("Found %d project(s) for analysis: %v", len(projects), projects)
					}
				} else {
					// Original single project flow
					allAvailableProjects, err := nx.GetProjects("", "", true)
					if err != nil {
						log.Fatalf("Error getting projects: %v", err)
					}

					// Check if the specified project exists
					projectExists := false
					for _, p := range allAvailableProjects {
						if p == projectName {
							projectExists = true
							break
						}
					}

					if !projectExists {
						log.Fatalf("Project '%s' not found. Available projects: %v",
							projectName, allAvailableProjects)
					}

					// Check if project is mapped (unless includeUnmapped is specified)
					if !includeUnmapped && !mapping.IsProjectMapped(projectName) {
						log.Fatalf("Project '%s' is not mapped in configuration. Use --include-unmapped to include it.", projectName)
					}

					projects = []string{projectName}
					if verboseLogging {
						log.Printf("Found project '%s' for analysis", projectName)
					}
				}
			} else {
				// Get all candidate projects (either all or affected)
				candidateProjects, err := nx.GetProjects(base, head, allProjects)
				if err != nil {
					log.Fatalf("Error getting projects: %v", err)
				}

				// Filter projects based on mapping configuration
				var skippedProjects []string

				// Only filter if we're not including unmapped projects
				if !includeUnmapped {
					for _, project := range candidateProjects {
						if mapping.IsProjectMapped(project) {
							projects = append(projects, project)
						} else {
							skippedProjects = append(skippedProjects, project)
						}
					}
				} else {
					projects = candidateProjects
				}

				if len(projects) == 0 {
					log.Println("No projects found to analyze.")
					if len(skippedProjects) > 0 {
						log.Printf("Skipped %d unmapped projects. Use --include-unmapped to include them.", len(skippedProjects))
					}
					return
				}

				if verboseLogging {
					log.Printf("Found %d projects to analyze in %.2f seconds\n",
						len(projects), time.Since(startTime).Seconds())

					if len(skippedProjects) > 0 {
						log.Printf("Skipped %d unmapped projects: %v\n",
							len(skippedProjects), skippedProjects)
					}
				}
			}

			// Set default concurrent workers if not specified
			if maxConcurrent <= 0 {
				maxConcurrent = runtime.NumCPU()
				log.Printf("Concurrency set to number of CPUs: %d\n", maxConcurrent)
			}

			// Initialize stats
			stats.initialize(len(projects))

			// Create timeout context
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
			defer cancel()

			// Process projects with optimized worker pool
			startTime = time.Now()
			results := processProjectsOptimized(ctx, projects, maxConcurrent, verboseLogging, stats)
			duration := time.Since(startTime)

			// Print summary
			log.Printf("FOSSA analysis complete in %.2f seconds", duration.Seconds())
			stats.print()

			// Send notifications if enabled
			if emailConfig.Enabled {
				if err := email.SendHTMLReport(results, emailConfig, verboseLogging); err != nil {
					log.Printf("Error sending email report: %v", err)
				}
			}

			if githubConfig.Enabled {
				if err := github.CreateIssues(results, githubConfig, verboseLogging); err != nil {
					log.Printf("Error creating GitHub issues: %v", err)
				}
				// Create commit status check
				if err := github.CreateCommitStatus(results, githubConfig, verboseLogging); err != nil {
					log.Printf("Error creating GitHub commit status: %v", err)
				}
			}

			// Exit with error if any projects failed
			if stats.failed > 0 {
				os.Exit(1)
			}
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Set config path if provided
			if configPath != "" {
				os.Setenv("FOSSA_CONFIG_PATH", configPath)
			}

			// CPU profiling if requested
			if cpuProfile != "" {
				f, err := os.Create(cpuProfile)
				if err != nil {
					log.Fatalf("Could not create CPU profile: %v", err)
				}
				if err := pprof.StartCPUProfile(f); err != nil {
					log.Fatalf("Could not start CPU profile: %v", err)
				}
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Stop CPU profiling if active
			if cpuProfile != "" {
				pprof.StopCPUProfile()
			}

			// Memory profiling if requested
			if memProfile != "" {
				f, err := os.Create(memProfile)
				if err != nil {
					log.Fatalf("Could not create memory profile: %v", err)
				}
				runtime.GC() // Get up-to-date statistics
				if err := pprof.WriteHeapProfile(f); err != nil {
					log.Fatalf("Could not write memory profile: %v", err)
				}
				f.Close()
			}
		},
	}

	// Root command flags
	rootCmd.Flags().StringVar(&base, "base", "", "Base commit for comparison")
	rootCmd.Flags().StringVar(&head, "head", "", "Head commit for comparison")
	rootCmd.Flags().BoolVarP(&verboseLogging, "verbose", "v", false, "Enable verbose logging")
	rootCmd.Flags().IntVarP(&maxConcurrent, "concurrent", "j", 0, "Maximum number of concurrent FOSSA scans (default: number of CPUs)")
	rootCmd.Flags().IntVarP(&timeout, "timeout", "t", 30, "Timeout in minutes for the entire operation")
	rootCmd.Flags().BoolVarP(&allProjects, "all", "a", false, "Analyze all projects, not just affected ones")
	rootCmd.Flags().BoolVar(&includeUnmapped, "include-unmapped", false, "Include projects not defined in configuration")
	rootCmd.Flags().StringVarP(&projectName, "project", "p", "", "Analyze a specific project by name")

	// Email notification flags
	rootCmd.Flags().BoolVar(&emailEnabled, "email", false, "Enable email notifications")
	rootCmd.Flags().StringVar(&smtpServer, "smtp-server", "", "SMTP server for email notifications")
	rootCmd.Flags().IntVar(&smtpPort, "smtp-port", 587, "SMTP port for email notifications")
	rootCmd.Flags().StringVar(&smtpUser, "smtp-user", "", "SMTP username")
	rootCmd.Flags().StringVar(&smtpPassword, "smtp-password", "", "SMTP password")
	rootCmd.Flags().StringVar(&fromEmail, "from-email", "", "Sender email address")
	rootCmd.Flags().StringVar(&toEmails, "to-email", "", "Recipient email addresses (comma-separated)")

	// GitHub integration flags
	rootCmd.Flags().BoolVar(&githubEnabled, "github", false, "Enable GitHub issue creation")
	rootCmd.Flags().StringVar(&githubToken, "github-token", "", "GitHub API token")
	rootCmd.Flags().StringVar(&githubOrg, "github-org", "", "GitHub organization")
	rootCmd.Flags().StringVar(&githubRepo, "github-repo", "", "GitHub repository")
	rootCmd.Flags().StringVar(&githubApiUrl, "github-api-url", "", "GitHub API URL for Enterprise instances")

	// Persistent flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "Write CPU profile to file")
	rootCmd.PersistentFlags().StringVar(&memProfile, "memprofile", "", "Write memory profile to file")
	rootCmd.PersistentFlags().BoolP("version", "V", false, "Show version information")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Updated to return results for notifications
func processProjectsOptimized(ctx context.Context, projects []string, workers int, verbose bool, stats *Stats) []models.Result {
	projectCh := make(chan string, workers*2)
	resultCh := make(chan models.Result, workers*2)
	results := make([]models.Result, 0, len(projects))

	// Use a WaitGroup to track worker completion
	var wg sync.WaitGroup

	// Spawn worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go optimizedWorker(ctx, projectCh, resultCh, &wg, verbose, i)
	}

	// Start a goroutine to close resultCh when all workers are done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Send projects to workers
	go func() {
		defer close(projectCh)
		for _, project := range projects {
			select {
			case projectCh <- project:
				// Project sent successfully
			case <-ctx.Done():
				// Context canceled, stop sending projects
				return
			}
		}
	}()

	// Collect and process results as they come in
	for result := range resultCh {
		success := result.Error == nil
		stats.recordResult(success, result.Duration, len(result.Issues))
		results = append(results, result)

		if verbose || !success {
			if success {
				log.Printf("✓ %s (%.2fs)", result.Project, result.Duration.Seconds())
				if len(result.Issues) > 0 {
					log.Printf("  Found %d vulnerabilities", len(result.Issues))
				}
			} else {
				log.Printf("✗ %s: %v (%.2fs)", result.Project, result.Error, result.Duration.Seconds())
			}
		}
	}

	return results
}

func optimizedWorker(ctx context.Context, projectCh <-chan string, resultCh chan<- models.Result, wg *sync.WaitGroup, verbose bool, workerId int) {
	defer wg.Done()

	for {
		select {
		case project, ok := <-projectCh:
			if !ok {
				// Channel closed, worker can exit
				return
			}

			if verbose {
				log.Printf("[Worker %d] Starting FOSSA analysis for %s...", workerId, project)
			}

			startTime := time.Now()
			err := fossa.RunAnalysis(project)

			// Get vulnerability data and FOSSA project link
			issues := []models.VulnerabilityIssue{}
			fossaLink := ""
			depCount := 0

			// If analysis was successful, check for vulnerabilities
			if err == nil {
				issues, fossaLink, depCount = getVulnerabilities(project)
			}

			duration := time.Since(startTime)
			endTime := time.Now() // Record when the scan completed

			resultCh <- models.Result{
				Project:         project,
				Error:           err,
				Duration:        duration,
				EndTime:         endTime,
				Issues:          issues,
				FossaLink:       fossaLink,
				DependencyCount: depCount,
			}

		case <-ctx.Done():
			// Context canceled, worker should exit
			if verbose {
				log.Printf("[Worker %d] Shutting down (context canceled)", workerId)
			}
			return
		}
	}
}

// Function to get vulnerabilities from FOSSA API
func getVulnerabilities(project string) ([]models.VulnerabilityIssue, string, int) {
	// This function would normally query the FOSSA API
	// For this example, I'm adding placeholder functionality
	issues := []models.VulnerabilityIssue{}

	// Get FOSSA project ID from mapping
	fossaProjectId := mapping.GetFossaProjectID(project)
	fossaEndpoint := mapping.GetFossaEndpoint()
	fossaLink := fmt.Sprintf("%s/projects/%s", fossaEndpoint, fossaProjectId)

	// Example API request to FOSSA
	apiUrl := fmt.Sprintf("%s/api/projects/%s/issues", fossaEndpoint, fossaProjectId)
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return issues, fossaLink, 0
	}

	apiKey := os.Getenv("FOSSA_API_KEY")
	req.Header.Add("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return issues, fossaLink, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return issues, fossaLink, 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return issues, fossaLink, 0
	}

	// Parse the response
	var issuesResponse struct {
		Issues []struct {
			Name        string    `json:"name"`
			Description string    `json:"description"`
			Severity    string    `json:"severity"`
			Link        string    `json:"link"`
			CVE         string    `json:"cve"`
			FirstSeen   time.Time `json:"firstSeen"`
			FixedIn     string    `json:"fixedIn"`
		} `json:"issues"`
		Dependencies int `json:"dependencies"`
	}

	if err := json.Unmarshal(body, &issuesResponse); err != nil {
		return issues, fossaLink, 0
	}

	// Convert to our issue format
	for _, issue := range issuesResponse.Issues {
		issues = append(issues, models.VulnerabilityIssue{
			Name:        issue.Name,
			Description: issue.Description,
			Severity:    issue.Severity,
			Link:        issue.Link,
			CVE:         issue.CVE,
			FirstSeen:   issue.FirstSeen,
			FixedIn:     issue.FixedIn,
		})
	}

	return issues, fossaLink, issuesResponse.Dependencies
}
