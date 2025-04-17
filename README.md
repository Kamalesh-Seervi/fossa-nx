# FOSSA-NX CLI

A high-performance Go CLI tool for running FOSSA license scans on `NX monorepo projects.`

## Table of Contents

- [Overview](#overview)
- [Requirements](#requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Basic Usage](#basic-usage)
  - [Project-Specific Analysis](#project-specific-analysis)
  - [Advanced Options](#advanced-options)
  - [Notification Options](#notification-options)
  - [Complete Usage Reference](#complete-usage-reference)
- [Technical Architecture](#technical-architecture)
- [Performance Optimization](#performance-optimization)
- [Troubleshooting](#troubleshooting)

## Overview

FOSSA-NX CLI is a developer-friendly tool designed to simplify running FOSSA license analysis on projects in an NX monorepo. Key features include:

- **Parallel Scanning**: Runs multiple FOSSA analyses simultaneously for dramatically faster results
- **Affected Project Detection**: Scans only projects affected by your changes
- **Full Repository Analysis**: Option to scan all projects in the monorepo
- **Single Project Analysis**: Target a specific project for faster, focused analysis
- **Configurable**: Use YAML configuration files for project and team mappings
- **Performance Optimized**: Advanced caching and concurrency management
- **Notification Support**: Email reports and GitHub issue creation for vulnerabilities

## Requirements

- **Go 1.17+** (for building from source)
- **FOSSA CLI** installed and authenticated (`fossa` command available in PATH)
- **NX** monorepo structure
- **Git** repository

## Installation

### Using Homebrew (This is Under Development NotDone)

```bash
brew tap kamalesh-seervi/tap
brew install fossa-nx
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/Kamalesh-Seervi/fossa-nx.git
cd fossa-nx

# Build the binary
go build -o fossa-nx cmd/fossa-nx/main.go

# Move to a directory in your PATH (optional)
sudo mv fossa-nx /usr/local/bin/
```

## Configuration

fossa-nx uses YAML configuration files to map projects to FOSSA configurations.

### Configuration File Location

Create `fossa-config.yaml` in one of the following locations:
- Current working directory (recommended)
- Your home directory
- A custom location specified with `--config` flag

### Configuration File Format

```yaml
# FOSSA Project Configuration
fossa:
  # Project ID mappings
  projects:
    "@test/app1": "app1-ID"
    "@test/app2": "app2-ID"
    # Add all your projects here
  
  # Default project ID if not found in mappings
  defaultProject: "test"
  
  # Team mappings
  teams:
    - prefixes: ["@test1/", "@shared/data"]
      teamValue: "org1"
      checkmarxPath: "data"
    
    - prefixes: ["@test2/"]
      teamValue: "org2" 
      checkmarxPath: "data2"
    
    # Add all your team mappings here
  
  # Default team ID if not found in mappings  
  defaultTeam: "org"
  
  # FOSSA endpoint URL
  endpoint: "https://fossa.com"
```

## Usage

### Basic Usage

```bash
# Analyze projects affected by changes between commits
fossa-nx fossa --base=develop --head=feature-branch

# Analyze all projects in the repository
fossa-nx fossa --all

# Use a specific config file
fossa-nx fossa --config=/path/to/config.yaml --all

# Increase verbosity
fossa-nx fossa --verbose --base=main --head=HEAD
```

### Project-Specific Analysis

```bash
# Analyze a specific project
fossa-nx fossa --project=@test/app1

# Analyze multiple specific projects with a comma-separated list
fossa-nx fossa --project=my-project1,my-project2,my-project3

# Include unmapped projects in specific project analysis
fossa-nx fossa --project=my-project1,my-project2 --include-unmapped
```

### Advanced Options

```bash
# Change the number of concurrent scans (default: number of CPU cores)
fossa-nx fossa --all --concurrent=16

# Set a custom timeout (default: 30 minutes)
fossa-nx fossa --all --timeout=60

# Performance profiling
fossa-nx fossa --all --cpuprofile=cpu.prof --memprofile=mem.prof
```

### Notification Options

```bash
# Send email report
fossa-nx fossa --all --email=report@example.com

# Create GitHub issue for vulnerabilities
fossa-nx fossa --all --github-issue
```

### Complete Usage Reference

```
Usage:
  fossa-nx fossa [flags]

Flags:
  -a, --all                   Analyze all projects, not just affected ones
      --base string           Base commit for comparison
  -j, --concurrent int        Maximum number of concurrent FOSSA scans (default: number of CPUs)
      --email                 Enable email notifications
      --from-email string     Sender email address
      --github                Enable GitHub issue creation
      --github-api-url string GitHub API URL for Enterprise instances
      --github-org string     GitHub organization
      --github-repo string    GitHub repository
      --github-token string   GitHub API token
      --head string           Head commit for comparison
      --include-unmapped      Include projects not defined in configuration
  -p, --project string        Analyze a specific project by name
      --smtp-password string  SMTP password
      --smtp-port int         SMTP port for email notifications (default 587)
      --smtp-server string    SMTP server for email notifications
      --smtp-user string      SMTP username
  -t, --timeout int           Timeout in minutes for the entire operation (default 30)
      --to-email string       Recipient email addresses (comma-separated)
  -v, --verbose               Enable verbose logging

Global Flags:
  -c, --config string        Path to config file
      --cpuprofile string    Write CPU profile to file
      --memprofile string    Write memory profile to file
  -V, --version              Show version information
```

## Examples

### Basic Workflow Examples

```bash
# Check version
fossa-nx --version

# Show help
fossa-nx --help
fossa-nx fossa --help

# Analyze projects affected by your changes
fossa-nx fossa --base=main --head=feature-branch

# Analyze a specific project
fossa-nx fossa --project=my-project

# Analyze all projects
fossa-nx fossa --all

# Include projects not mapped in configuration
fossa-nx fossa --all --include-unmapped
```

### Notification Examples

```bash
# Send email notifications
fossa-nx fossa --all --email \
  --smtp-server=smtp.example.com --smtp-port=587 \
  --smtp-user=username --smtp-password=password \
  --from-email=sender@example.com --to-email=recipient@example.com,team@example.com

# Create GitHub issues for vulnerabilities
fossa-nx fossa --all --github \
  --github-token=YOUR_TOKEN --github-org=your-org --github-repo=your-repo

# For GitHub Enterprise
fossa-nx fossa --all --github \
  --github-token=YOUR_TOKEN --github-org=your-org --github-repo=your-repo \
  --github-api-url=https://github.yourcompany.com/api/v3

# Using both email and GitHub notifications together
fossa-nx fossa --all --email --github \
  --smtp-server=smtp.example.com --smtp-port=587 \
  --smtp-user=username --smtp-password=password \
  --from-email=sender@example.com --to-email=recipient@example.com \
  --github-token=YOUR_TOKEN --github-org=your-org --github-repo=your-repo
```

### Performance Tuning Examples

```bash
# Adjust concurrency
fossa-nx fossa --all --concurrent=8

# Set longer timeout (for large repositories)
fossa-nx fossa --all --timeout=60  # 60 minutes

# Generate performance profiles
fossa-nx fossa --all --cpuprofile=cpu.prof --memprofile=mem.prof
```

### CI/CD Integration Examples

```bash
# Jenkins Pipeline
stage('FOSSA Analysis') {
  steps {
    sh 'fossa-nx fossa --base=$GIT_PREVIOUS_COMMIT --head=$GIT_COMMIT --verbose'
  }
}

# GitHub Actions workflow
- name: Run FOSSA Analysis
  run: |
    fossa-nx fossa --base=${{ github.event.before }} --head=${{ github.sha }} --verbose

# GitLab CI
fossa-scan:
  script:
    - fossa-nx fossa --base=$CI_COMMIT_BEFORE_SHA --head=$CI_COMMIT_SHA --verbose
```

## Technical Architecture

FOSSA-NX CLI uses Go's advanced concurrency primitives for high performance:

### Concurrency Model

- **Worker Pool Pattern**: Uses a pool of worker goroutines to process projects in parallel
- **Buffered Channels**: Efficiently distributes work and collects results
- **Context-Based Cancellation**: Ensures proper cleanup on timeouts
- **Atomic Operations**: Thread-safe counters for statistics
- **Mutexes**: Limited use for filesystem operations and non-atomic state

## FOSSA-NX CLI Architecture

```
+----------------------------------------------------------------------------------------------------------+
|                                             FOSSA-NX CLI Architecture                                    |
+----------------------------------------------------------------------------------------------------------+

+-------------+     +-----------------+     +-------------------+     +-----------------+
| User Command|---->| Parse Arguments |---->| Load Configuration|---->| Get NX Projects |
+-------------+     +-----------------+     +-------------------+     +-----------------+
                                                                              |
                                                                              v
                                +-------------------------------+
                                |                               |
                                |     Concurrency Control       |
                                |                               |
                                +--------------+----------------+
                                               |
                                               |
                                               v
+----------------------------------------------------------------------------+
|                            Worker Pool Architecture                         |
+----------------------------------------------------------------------------+

  +----------------+      +--------------+     +-------------+
  | Project List   |      | Channel      |     | Worker Pool |
  | [P1,P2,P3,...] |----->| Buffer       |---->| Goroutines  |
  +----------------+      +--------------+     +-------------+
                                |                    |
                                v                    v
         +--------------------------+      +-------------------+
         | projectCh (buffered)     |      | WaitGroup tracks |
         | Size = workers*2         |      | worker completion|
         +--------------------------+      +-------------------+
                   |                               |
                   v                               v
  +-------------------------------+      +-----------------------+
  | Sender Goroutine              |      | Multiple Worker       |
  | - Feeds projects to channel   |      | Goroutines (1 per CPU)|
  | - Handles context cancellation|      | - Process projects    |
  +-------------------------------+      | - Send results        |
                                         +-----------------------+
                                                   |
                                                   v
                                         +-----------------------+
                                         | resultCh (buffered)   |
                                         | Size = workers*2      |
                                         +-----------------------+
                                                   |
                                                   v
                           +-------------------------------------------+
                           | Results Processor in Main Goroutine       |
                           | - Processes results as they arrive        |
                           | - Updates statistics                      |
                           | - Logs success/failure                    |
                           +-------------------------------------------+

Concurrency Flow:
==================

  Main Thread                 Sender Goroutine             Worker Goroutines (n)         
  ============                ================             ====================         
       |                            |                              |
       |---(Initialize)------------>|                              |
       |                            |                              |
       |---(Start workers)---------------------------------------->|
       |                            |                              |
       |                            |---(Send project)------------>|
       |                            |                              |
       |                            |<-----(Request next)----------|
       |                            |                              |
       |                            |---(Send project)------------>|
       |                            |                              |
       |<--(Collect results)-----------------------------------------|
       |                            |                              |
       |---(Context canceled?)----->|                              |
       |                            |                              |
       |                            |---(Close channel)----------->|
       |                            |                              |
       |<--(All workers done)---------------------------------------|
       |                            |                              |
       |---(Print summary)          |                              |
       |                            |                              |

Synchronization Mechanisms:
===========================

  +-------------------+    +-------------------+    +-------------------+
  | sync.WaitGroup    |    | Buffered Channels |    | Context           |
  | - Track completion|    | - Flow control    |    | - Cancellation    |
  | - Join goroutines |    | - Work queue      |    | - Timeout handling|
  +-------------------+    +-------------------+    +-------------------+
                 |                  |                      |
                 v                  v                      v
  +-------------------+    +-------------------+    +-------------------+
  | Atomic Operations |    | Mutex Protection  |    | Select Statements |
  | - Thread-safe     |    | - Protect shared  |    | - Non-blocking I/O|
  |   counters        |    |   resources       |    | - Multi-channel   |
  +-------------------+    +-------------------+    +-------------------+
```

### Key Components

- **nx**: Handles project discovery and dependency resolution
- **fossa**: Performs the actual FOSSA analysis with proper Git context
- **mapping**: Manages project/team mappings from configuration
- **git**: Encapsulates Git operations with caching

## Performance Optimization

This tool is highly optimized for performance:

- **Caching**: Project roots, Git info, and configuration are cached
- **Parallel Processing**: Multiple analyses run simultaneously
- **Efficient Resource Usage**: Controlled concurrency prevents system overload
- **Minimal Filesystem Operations**: Avoids unnecessary I/O
- **Smart Dependency Resolution**: Creates accurate package.json files for analysis

## Troubleshooting

### Common Issues

**Error: No configuration file found**
- Create `fossa-config.yaml` in your project directory or specify with `--config`

**Error: Could not determine project root**
- Verify project name matches NX structure
- Check your NX workspace configuration

**Error: FOSSA analyze command failed**
- Ensure FOSSA CLI is installed and authenticated
- Check team values in configuration file

**Error: SSL certificate issues**
- Tool automatically unsets SSL_CERT_DIR environment variable

### Performance Issues

If scans are taking too long:
- Increase concurrency with `--concurrent` flag
- Ensure system has sufficient resources
- Use `--verbose` to identify bottlenecks

### Debugging

For advanced debugging:
- Use `--verbose` flag for detailed logging
- Use `--cpuprofile` and `--memprofile` flags
- Examine Go profiling data with `go tool pprof`

# For CPU profile visualization
```go tool pprof -http=:8080 cpu.prof```

# For memory profile visualization
```go tool pprof -http=:8080 mem.prof```
This opens a web interface on port 8080 where you can explore:
