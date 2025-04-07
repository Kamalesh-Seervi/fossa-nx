package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalesh-seervi/fossa-cli/internal/fossa"
	"github.com/kamalesh-seervi/fossa-cli/internal/nx"
	"github.com/spf13/cobra"
)

type Result struct {
	Project  string
	Error    error
	Duration time.Duration
}

// Stats for tracking execution metrics
type Stats struct {
	totalProjects int32
	successful    int32
	failed        int32
	totalDuration int64 // nanoseconds
	maxDuration   int64 // nanoseconds
	minDuration   int64 // nanoseconds (initialized to a large value)
	mutex         sync.Mutex
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

func (s *Stats) recordResult(success bool, duration time.Duration) {
	durationNanos := duration.Nanoseconds()

	if success {
		atomic.AddInt32(&s.successful, 1)
	} else {
		atomic.AddInt32(&s.failed, 1)
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
	log.Printf("  Average Duration: %.2f seconds", avgDuration.Seconds())

	if successful+failed > 0 {
		log.Printf("  Min Duration: %.2f seconds", minDuration.Seconds())
		log.Printf("  Max Duration: %.2f seconds", maxDuration.Seconds())
	}
}

func main() {
	var (
		base           string
		head           string
		verboseLogging bool
		maxConcurrent  int
		timeout        int
		configPath     string
		cpuProfile     string
		memProfile     string
		allProjects    bool // New flag for analyzing all projects
	)

	// Initialize stats tracking
	stats := &Stats{}

	rootCmd := &cobra.Command{
		Use:   "fossa-nx",
		Short: "fossa-nx is a CLI tool for Ford's development workflow",
		Long:  `A CLI tool to help Ford developers with various tasks including FOSSA analysis.`,
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

	fossaCmd := &cobra.Command{
		Use:   "fossa",
		Short: "Run FOSSA analysis on NX projects",
		Long: `Run FOSSA analysis on NX projects.
		
By default, only affected projects are analyzed. Use the --all flag to analyze all projects.

Examples:
	fossa-nx fossa --base=develop --head=feature-branch  # Analyze affected projects
  fossa-nx fossa --all                                # Analyze all projects
`,
		Run: func(cmd *cobra.Command, args []string) {
			if verboseLogging {
				log.Println("Running FOSSA analysis on projects...")
				if !allProjects && base != "" && head != "" {
					log.Printf("Using base: %s and head: %s\n", base, head)
				}
				if allProjects {
					log.Println("Analyzing ALL projects (not just affected ones)")
				}
			}

			// Get projects based on all flag
			startTime := time.Now()
			projects, err := nx.GetProjects(base, head, allProjects)
			if err != nil {
				log.Fatalf("Error getting projects: %v", err)
			}
			projectFetchTime := time.Since(startTime)

			if len(projects) == 0 {
				log.Println("No projects found.")
				return
			}

			if verboseLogging {
				log.Printf("Found %d projects in %.2f seconds\n",
					len(projects), projectFetchTime.Seconds())
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
			processProjectsOptimized(ctx, projects, maxConcurrent, verboseLogging, stats)
			duration := time.Since(startTime)

			// Print summary
			log.Printf("FOSSA analysis complete in %.2f seconds", duration.Seconds())
			stats.print()

			// Exit with error if any projects failed
			if stats.failed > 0 {
				os.Exit(1)
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&cpuProfile, "cpuprofile", "", "Write CPU profile to file")
	rootCmd.PersistentFlags().StringVar(&memProfile, "memprofile", "", "Write memory profile to file")

	fossaCmd.Flags().StringVar(&base, "base", "", "Base commit for comparison")
	fossaCmd.Flags().StringVar(&head, "head", "", "Head commit for comparison")
	fossaCmd.Flags().BoolVarP(&verboseLogging, "verbose", "v", false, "Enable verbose logging")
	fossaCmd.Flags().IntVarP(&maxConcurrent, "concurrent", "j", 0, "Maximum number of concurrent FOSSA scans (default: number of CPUs)")
	fossaCmd.Flags().IntVarP(&timeout, "timeout", "t", 30, "Timeout in minutes for the entire operation")
	fossaCmd.Flags().BoolVarP(&allProjects, "all", "a", false, "Analyze all projects, not just affected ones")

	rootCmd.AddCommand(fossaCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func processProjectsOptimized(ctx context.Context, projects []string, workers int, verbose bool, stats *Stats) {
	// Use buffered channels appropriately sized
	projectCh := make(chan string, workers*2)
	resultCh := make(chan Result, workers*2)

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
		for _, project := range projects {
			select {
			case projectCh <- project:
				// Project sent successfully
			case <-ctx.Done():
				// Context canceled, stop sending projects
				break
			}
		}
		close(projectCh)
	}()

	// Collect and process results as they come in
	for result := range resultCh {
		success := result.Error == nil
		stats.recordResult(success, result.Duration)

		if verbose || !success {
			if success {
				log.Printf("✓ %s (%.2fs)", result.Project, result.Duration.Seconds())
			} else {
				log.Printf("✗ %s: %v (%.2fs)", result.Project, result.Error, result.Duration.Seconds())
			}
		}
	}
}

func optimizedWorker(ctx context.Context, projectCh <-chan string, resultCh chan<- Result, wg *sync.WaitGroup, verbose bool, workerId int) {
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
			duration := time.Since(startTime)

			resultCh <- Result{
				Project:  project,
				Error:    err,
				Duration: duration,
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
