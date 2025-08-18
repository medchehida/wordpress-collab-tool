package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

// Site represents a deployed WordPress site.
type Site struct {
	ProjectName   string   `json:"projectName"`
	WPPort        int      `json:"wpPort"`
	DBName        string   `json:"dbName"`
	DBPassword    string   `json:"dbPassword"`
	SiteURL       string   `json:"siteURL"`
	Plugins       []string `json:"plugins"`
	Status        string   `json:"status"`
	AdminUsername string   `json:"adminUsername"`
	AdminPassword string   `json:"adminPassword"`
}

var (
	sitesFilePath = "sites.json"
	sitesMutex    sync.Mutex
)

// readSites reads the list of sites from the JSON file.
func readSites() ([]Site, error) {
	sitesMutex.Lock()
	defer sitesMutex.Unlock()

	data, err := ioutil.ReadFile(sitesFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Site{}, nil // Return empty slice if file does not exist
		}
		return nil, fmt.Errorf("failed to read sites file: %w", err)
	}

	var sites []Site
	if err := json.Unmarshal(data, &sites); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sites data: %w", err)
	}

	return sites, nil
}

// writeSites writes the list of sites to the JSON file.
func writeSites(sites []Site) error {
	sitesMutex.Lock()
	defer sitesMutex.Unlock()

	data, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sites data: %w", err)
	}

	if err := ioutil.WriteFile(sitesFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sites file: %w", err)
	}

	return nil
}
