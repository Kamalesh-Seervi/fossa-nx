// Package models contains shared data structures used across the FOSSA-NX application
package models

import (
	"time"
)

// VulnerabilityIssue represents a security vulnerability found by FOSSA
type VulnerabilityIssue struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Link        string    `json:"link"`
	CVE         string    `json:"cve,omitempty"`
	FirstSeen   time.Time `json:"firstSeen,omitempty"`
	FixedIn     string    `json:"fixedIn,omitempty"`
}

// Result represents the result of a FOSSA scan for a project
type Result struct {
	Project         string
	Error           error
	Duration        time.Duration
	EndTime         time.Time // When the scan completed
	Issues          []VulnerabilityIssue
	FossaLink       string
	DependencyCount int
}

// EmailConfig holds email notification configuration
type EmailConfig struct {
	SmtpServer   string
	SmtpPort     int
	SmtpUser     string
	SmtpPassword string
	FromEmail    string
	ToEmails     []string
	Enabled      bool
}

// GitHubConfig holds GitHub issue creation configuration
type GitHubConfig struct {
	Token        string
	Organization string
	Repository   string
	ApiUrl       string // GitHub API URL for Enterprise instances
	Enabled      bool
}

// FossaConfig holds all FOSSA-related configuration
type FossaConfig struct {
	Projects       map[string]string `yaml:"projects"`
	DefaultProject string            `yaml:"defaultProject"`
	Teams          []TeamMapping     `yaml:"teams"`
	DefaultTeam    string            `yaml:"defaultTeam"`
	Endpoint       string            `yaml:"endpoint"`
}

// TeamMapping represents a team mapping from project prefix to team value
type TeamMapping struct {
	Prefixes      []string `yaml:"prefixes"`
	TeamValue     string   `yaml:"teamValue"`
	CheckmarxPath string   `yaml:"checkmarxPath"`
}

// Config holds the entire application configuration
type Config struct {
	Fossa FossaConfig `yaml:"fossa"`
}

// Stats tracks execution metrics for FOSSA scan operations
type Stats struct {
	TotalProjects   int32
	Successful      int32
	Failed          int32
	Vulnerabilities int32
	TotalDuration   int64 // nanoseconds
	MaxDuration     int64 // nanoseconds
	MinDuration     int64 // nanoseconds
}
