package profiles

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Profile represents a routing configuration
type Profile struct {
	Name         string        `yaml:"name"`
	Description  string        `yaml:"description"`
	VirtualSink  string        `yaml:"virtual_sink"`
	Applications []Application `yaml:"applications"`
}

// Application within a profile
type Application struct {
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

// LoadProfile loads a profile from a YAML file
func LoadProfile(path string) (*Profile, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile: %w", err)
	}

	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}
	return &p, nil
}

// ListProfiles returns a list of profile filenames from a directory
func ListProfiles(profilesDir string) ([]string, error) {
	// Check if directory exists
	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("profiles directory does not exist: %s", profilesDir)
	}

	files, err := ioutil.ReadDir(profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profileFiles []string
	for _, file := range files {
		if !file.IsDir() {
			if strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") {
				profileFiles = append(profileFiles, file.Name())
			}
		}
	}

	return profileFiles, nil
}

// ProfileExists checks if a profile file exists
func ProfileExists(profilesDir, profileName string) (bool, error) {
	profilePath := filepath.Join(profilesDir, profileName)
	_, err := os.Stat(profilePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
