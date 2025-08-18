package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

// Activity represents a log entry for an action.
type Activity struct {
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
}

var (
	activitiesFilePath = "activities.json"
	activitiesMutex    sync.Mutex
)

// readActivities reads the list of activities from the JSON file.
func readActivities() ([]Activity, error) {
	activitiesMutex.Lock()
	defer activitiesMutex.Unlock()

	data, err := ioutil.ReadFile(activitiesFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Activity{}, nil // Return empty slice if file does not exist
		}
		return nil, fmt.Errorf("failed to read activities file: %w", err)
	}

	var activities []Activity
	if err := json.Unmarshal(data, &activities); err != nil {
		return nil, fmt.Errorf("failed to unmarshal activities data: %w", err)
	}

	return activities, nil
}

// writeActivities writes the list of activities to the JSON file.
func writeActivities(activities []Activity) error {
	activitiesMutex.Lock()
	defer activitiesMutex.Unlock()

	data, err := json.MarshalIndent(activities, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal activities data: %w", err)
	}

	if err := ioutil.WriteFile(activitiesFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write activities file: %w", err)
	}

	return nil
}

// logActivity adds a new activity entry.
func logActivity(action string) {
	activities, err := readActivities()
	if err != nil {
		log.Printf("Error reading activities to log: %v", err)
		return
	}

	newActivity := Activity{
		Action:    action,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}

	activities = append([]Activity{newActivity}, activities...) // Add to the beginning

	// Keep only the last 100 activities
	if len(activities) > 100 {
		activities = activities[:100]
	}

	if err := writeActivities(activities); err != nil {
		log.Printf("Error writing activities after logging: %v", err)
	}
}
