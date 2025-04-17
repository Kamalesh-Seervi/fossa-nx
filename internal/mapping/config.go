package mapping

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kamalesh-seervi/fossa-nx/internal/models"
	"gopkg.in/yaml.v3"
)

var (
	// globalConfig holds the loaded configuration
	globalConfig *models.Config
	configOnce   sync.Once
	configError  error
)

// LoadConfig loads the configuration from the config file - using sync.Once for thread-safety
func LoadConfig() (*models.Config, error) {
	configOnce.Do(func() {
		configError = loadConfigImpl()
	})
	return globalConfig, configError
}

// loadConfigImpl is the actual implementation of config loading
func loadConfigImpl() error {
	// Get config path from environment if set
	configPath := os.Getenv("FOSSA_CONFIG_PATH")

	// If config path is set and file exists, use it
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return loadConfigFromFile(configPath)
		} else {
			return fmt.Errorf("specified config file not found: %s", configPath)
		}
	}

	// Check for config files in common locations (in order of preference)
	configFiles := []string{
		"fossa-config.yaml",
		"fossa-config.yml",
		".fossa.yaml",
		".fossa.yml",
	}

	// Try current directory first (most common case)
	for _, filename := range configFiles {
		if _, err := os.Stat(filename); err == nil {
			return loadConfigFromFile(filename)
		}
	}

	// Try home directory next
	home, err := os.UserHomeDir()
	if err == nil {
		for _, filename := range configFiles {
			homeConfig := filepath.Join(home, filename)
			if _, err := os.Stat(homeConfig); err == nil {
				return loadConfigFromFile(homeConfig)
			}
		}
	}

	return fmt.Errorf("no configuration file found. Please create fossa-config.yaml in your project directory or home directory")
}

// loadConfigFromFile loads and parses a specific config file
func loadConfigFromFile(configPath string) error {
	// Read and parse the config file
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file %s: %w", configPath, err)
	}

	config := &models.Config{}
	if err = yaml.Unmarshal(configData, config); err != nil {
		return fmt.Errorf("error parsing config file %s: %w", configPath, err)
	}

	// Validate required fields
	if config.Fossa.Projects == nil || len(config.Fossa.Projects) == 0 {
		return fmt.Errorf("missing or empty projects section in config file %s", configPath)
	}

	if config.Fossa.Teams == nil || len(config.Fossa.Teams) == 0 {
		return fmt.Errorf("missing or empty teams section in config file %s", configPath)
	}

	if config.Fossa.Endpoint == "" {
		return fmt.Errorf("missing endpoint in config file %s", configPath)
	}

	globalConfig = config
	fmt.Printf("Loaded configuration from %s\n", configPath)

	return nil
}

// IsProjectMapped returns whether a project is mapped in the configuration
func IsProjectMapped(projectName string) bool {
	config, err := LoadConfig()
	if err != nil {
		return false
	}

	_, exists := config.Fossa.Projects[projectName]
	return exists
}

// GetFossaProjectID returns the FOSSA project ID for a given project name
func GetFossaProjectID(projectName string) string {
	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if id, ok := config.Fossa.Projects[projectName]; ok {
		return id
	}

	return config.Fossa.DefaultProject
}

// GetTeamValue returns the team value for a given project name
func GetTeamValue(projectName string) string {
	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Find matching team from config
	for _, team := range config.Fossa.Teams {
		for _, prefix := range team.Prefixes {
			if strings.HasPrefix(projectName, prefix) {
				return team.TeamValue
			}
		}
	}

	return config.Fossa.DefaultTeam
}

// GetFossaEndpoint returns the configured FOSSA endpoint
func GetFossaEndpoint() string {
	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	return config.Fossa.Endpoint
}
