package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net/http" // Added for GetSiteStatus
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"wordpress-collab-tool/config"
	"wordpress-collab-tool/models"
	"wordpress-collab-tool/utils"
)

const (
	sitesFilePath    = "sites.json"
	activitiesFilePath = "activities.json"
)

// GenerateRandomPassword generates a random string of specified length.
func GenerateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// ReadSites reads the list of sites from sites.json.
func ReadSites() ([]models.Site, error) {
	var sites []models.Site
	if _, err := os.Stat(sitesFilePath); os.IsNotExist(err) {
		return sites, nil // Return empty list if file doesn't exist
	}

	data, err := ioutil.ReadFile(sitesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sites file: %w", err)
	}

	if len(data) == 0 {
		return sites, nil // Return empty list if file is empty
	}

	err = json.Unmarshal(data, &sites)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal sites data: %w", err)
	}
	return sites, nil
}

// ReadSitesOrEmpty reads the list of sites from sites.json, returning an empty slice on error.
func ReadSitesOrEmpty() []models.Site {
	sites, err := ReadSites()
	if err != nil {
		utils.LogError("Error reading sites file, returning empty slice: %v", err)
		return []models.Site{}
	}
	return sites
}

// WriteSites writes the list of sites to sites.json.
func WriteSites(sites []models.Site) error {
	data, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sites data: %w", err)
	}

	err = ioutil.WriteFile(sitesFilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write sites file: %w", err)
	}
	return nil
}

// LogActivity logs an activity to activities.json.
func LogActivity(action string) {
	var activities []models.Activity
	if _, err := os.Stat(activitiesFilePath); !os.IsNotExist(err) {
		data, err := ioutil.ReadFile(activitiesFilePath)
		if err == nil && len(data) > 0 {
			json.Unmarshal(data, &activities)
		}
	}

	newActivity := models.Activity{
		Action:    action,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	activities = append(activities, newActivity)

	data, err := json.MarshalIndent(activities, "", "  ")
	if err != nil {
		utils.LogError("Failed to marshal activities data: %v", err)
		return
	}

	err = ioutil.WriteFile(activitiesFilePath, data, 0644)
	if err != nil {
		utils.LogError("Failed to write activities file: %v", err)
	}
}

// ReadActivities reads the list of activities from activities.json.
func ReadActivities() ([]models.Activity, error) {
	var activities []models.Activity
	if _, err := os.Stat(activitiesFilePath); os.IsNotExist(err) {
		return activities, nil // Return empty list if file doesn't exist
	}

	data, err := ioutil.ReadFile(activitiesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read activities file: %w", err)
	}

	if len(data) == 0 {
		return activities, nil // Return empty list if file is empty
	}

	err = json.Unmarshal(data, &activities)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal activities data: %w", err)
	}
	return activities, nil
}

// UpdateSiteStatus updates the status of a site.
func UpdateSiteStatus(projectName, status string) {
	sites, err := ReadSites()
	if err != nil {
		utils.LogError("Failed to read sites for status update: %v", err)
		return
	}

	for i := range sites {
		if sites[i].ProjectName == projectName {
			sites[i].Status = status
			sites[i].LastChecked = time.Now().Format(time.RFC3339)
			break
		}
	}

	if err := WriteSites(sites); err != nil {
		utils.LogError("Failed to write sites after status update: %v", err)
	}
}

// CleanupSite cleans up resources if site creation fails.
func CleanupSite(client *ssh.Client, projectName string, wpPort int) {
	utils.LogInfo("Cleaning up resources for failed site: %s", projectName)
	if client != nil {
		remotePath := fmt.Sprintf("/var/www/%s", projectName)
		// Stop and remove docker containers
		RunSSHCommand(client, fmt.Sprintf("cd %s && docker compose down", remotePath))
		// Remove remote directory
		RunSSHCommand(client, fmt.Sprintf("sudo rm -rf %s", remotePath))
	}
	UpdateSiteStatus(projectName, "failed")
	LogActivity(fmt.Sprintf("Site '%s' creation failed and resources cleaned up.", projectName))
}

// DeployWordPressSite handles the full deployment process of a WordPress site.
func DeployWordPressSite(site models.Site, selectedPlugins []string, adminUsername, adminPassword string) error {
	cfg := config.LoadConfig()

	sshClient, err := GetSSHClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to VPS: %w", err)
	}
	defer sshClient.Close()

	// Generate the docker-compose.yml file on the local machine
	tmpl, err := template.ParseFiles("templates/template.yml")
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var tpl bytes.Buffer
	templateConfig := models.Config{
		ProjectName: site.ProjectName,
		WPPort:      site.WPPort,
		DBName:      site.DBName,
		DBPassword:  site.DBPassword,
	}
	if err := tmpl.Execute(&tpl, templateConfig); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	remotePath := fmt.Sprintf("/var/www/%s", site.ProjectName)

	// Create the remote directory with sudo and change ownership
	utils.LogInfo("Creating remote directory: %s", remotePath)
	stdout, stderr, err := RunSSHCommand(sshClient, fmt.Sprintf("sudo install -d -o %s -g %s %s", cfg.SSHUser, cfg.SSHUser, remotePath))
	if err != nil {
		return fmt.Errorf("failed to create and set ownership of remote directory: %w, stdout: %s, stderr: %s", err, stdout, stderr)
	}
	utils.LogInfo("Remote directory created successfully.")

	sftpClient, err := GetSFTPClient(sshClient)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Upload the docker-compose.yml file
	utils.LogInfo("Uploading docker-compose.yml to %s", remotePath)
	err = UploadFile(sftpClient, filepath.Join(remotePath, "docker-compose.yml"), &tpl)
	if err != nil {
		return fmt.Errorf("failed to upload docker-compose.yml: %w", err)
	}
	utils.LogInfo("docker-compose.yml uploaded successfully.")

	// Check if the file exists and has the correct permissions
	utils.LogInfo("Checking for docker-compose.yml in %s", remotePath)
	stdout, stderr, err = RunSSHCommand(sshClient, fmt.Sprintf("ls -l %s", remotePath))
	if err != nil {
		utils.LogError("Failed to list files in remote directory: %v, stdout: %s, stderr: %s", err, stdout, stderr)
	} else {
		utils.LogInfo("Files in remote directory: %s", stdout)
	}

	// Run docker compose up -d
	utils.LogInfo("Executing command: cd %s && docker compose up -d", remotePath)
	stdout, stderr, err = RunSSHCommand(sshClient, fmt.Sprintf("cd %s && docker compose up -d", remotePath))
	if err != nil {
		return fmt.Errorf("failed to run docker compose up: %w, stdout: %s, stderr: %s", err, stdout, stderr)
	}
	utils.LogInfo("docker compose up -d command executed. stdout: %s, stderr: %s", stdout, stderr)
	time.Sleep(5 * time.Second) // Give docker-compose a moment to start

	// Wait for the WordPress container to be ready
	utils.LogInfo("Waiting for WordPress container to be ready for site '%s'...", site.ProjectName)
	if err := waitForContainerHealthy(sshClient, site.ProjectName, "_wordpress"); err != nil {
		return fmt.Errorf("WordPress container did not become ready: %w", err)
	}
	utils.LogInfo("WordPress container for site '%s' is ready.", site.ProjectName)

	// Wait for the CLI container to be ready
	utils.LogInfo("Waiting for CLI container to be ready for site '%s'...", site.ProjectName)
	if err := waitForContainerHealthy(sshClient, site.ProjectName, "_cli"); err != nil {
		return fmt.Errorf("CLI container did not become ready: %w", err)
	}
	utils.LogInfo("CLI container for site '%s' is ready.", site.ProjectName)
	time.Sleep(5 * time.Second) // Give CLI container a moment to be fully ready

	// Install WordPress
	wpInstallCmd := fmt.Sprintf("cd %s && docker compose exec -T --user www-data %s_cli wp core install --url=%s --title='%s' --admin_user='%s' --admin_password='%s' --admin_email='admin@%s.com' --skip-email --debug", remotePath, site.ProjectName, site.SiteURL, site.ProjectName, adminUsername, adminPassword, site.ProjectName)
	utils.LogInfo("Executing WordPress install command: %s", wpInstallCmd)
	stdout, stderr, err = RunSSHCommand(sshClient, wpInstallCmd)
	if err != nil {
		return fmt.Errorf("failed to install WordPress for site '%s'. Command: '%s', Error: %w, stdout: %s, stderr: %s", site.ProjectName, wpInstallCmd, err, stdout, stderr)
	}
	utils.LogInfo("WordPress installed successfully for site '%s'.", site.ProjectName)

	// Install selected plugins
	if len(selectedPlugins) > 0 {
		utils.LogInfo("Waiting for 10 seconds before starting plugin installation...")
		time.Sleep(10 * time.Second)
		utils.LogInfo("Starting plugin installation for site '%s'.", site.ProjectName)

		var installErrors []string
		for _, plugin := range selectedPlugins {
			pluginInstallCmd := fmt.Sprintf("cd %s && docker compose exec -T %s_cli wp plugin install %s --activate", remotePath, site.ProjectName, plugin)
			stdout, stderr, err := RunSSHCommand(sshClient, pluginInstallCmd)
			if err != nil {
				errMessage := fmt.Sprintf("failed to install plugin '%s' for site '%s'. Error: %v, stdout: %s, stderr: %s", plugin, site.ProjectName, err, stdout, stderr)
				utils.LogError(errMessage)
				LogActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s'.", plugin, site.ProjectName))
				installErrors = append(installErrors, errMessage)
			} else {
				utils.LogInfo("Plugin '%s' installed successfully on site '%s'.", plugin, site.ProjectName)
				LogActivity(fmt.Sprintf("Plugin '%s' installed successfully on site '%s'.", plugin, site.ProjectName))
			}
		}

		if len(installErrors) > 0 {
			return fmt.Errorf("encountered errors during plugin installation:\n%s", strings.Join(installErrors, "\n"))
		}

		utils.LogInfo("All plugins installed successfully for site '%s'.", site.ProjectName)
	}

	return nil
}

// waitForContainerHealthy waits for a specific container to report a "healthy" status.
func waitForContainerHealthy(client *ssh.Client, projectName, suffix string) error {
	containerName := fmt.Sprintf("%s%s", projectName, suffix)
	for i := 0; i < 24; i++ { // 2 minutes timeout (24 * 5 seconds)
		cmd := fmt.Sprintf("docker inspect --format='{{.State.Health.Status}}' %s", containerName)
		stdout, stderr, err := RunSSHCommand(client, cmd)
		if err == nil && strings.TrimSpace(stdout) == "healthy" {
			return nil
		}
		utils.LogInfo("Waiting for container %s to be healthy... attempt %d/24, status: %s, stderr: %s, err: %v", containerName, i+1, strings.TrimSpace(stdout), stderr, err)
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("container %s did not become healthy in time", containerName)
}

// GetSiteStatus checks the HTTP status of a WordPress site.
func GetSiteStatus(site models.Site) string {
	resp, err := http.Get(site.SiteURL)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "active"
	}

	return "error"
}
