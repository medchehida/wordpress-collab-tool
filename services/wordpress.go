package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http" // Added for GetSiteStatus
	"os"
	"path/filepath"
	"strings"
	"text/template"
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
		return []models.Site{}, nil // Return empty slice if file doesn't exist
	}

	data, err := ioutil.ReadFile(sitesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sites file: %w", err)
	}

	if len(data) == 0 {
		return []models.Site{}, nil // Return empty slice if file is empty
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
				RunSSHCommand(client, fmt.Sprintf("cd %s && docker compose -f %s/docker-compose.yml down", remotePath, remotePath))
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
	stdout, stderr, err = RunSSHCommand(sshClient, fmt.Sprintf("cd %s && docker compose -f %s/docker-compose.yml up -d", remotePath, remotePath))
	if err != nil {
		return fmt.Errorf("failed to run docker compose up: %w, stdout: %s, stderr: %s", err, stdout, stderr)
	}
	utils.LogInfo("docker compose up -d command executed. stdout: %s, stderr: %s", stdout, stderr)
	time.Sleep(5 * time.Second) // Give docker-compose a moment to start

	// Wait for the WordPress container to be ready
	utils.LogInfo("Waiting for WordPress container to be ready for site '%s'...", site.ProjectName)
	if err := waitForContainerHealthy(sshClient, site.ProjectName, "_wordpress", remotePath); err != nil {
		return fmt.Errorf("WordPress container did not become ready: %w", err)
	}
	utils.LogInfo("WordPress container for site '%s' is ready.", site.ProjectName)

	// Wait for the CLI container to be ready
	utils.LogInfo("Waiting for CLI container to be ready for site '%s'...", site.ProjectName)
	if err := waitForContainerHealthy(sshClient, site.ProjectName, "_cli", remotePath); err != nil {
		return fmt.Errorf("CLI container did not become ready: %w", err)
	}
	utils.LogInfo("CLI container for site '%s' is ready.", site.ProjectName)
	time.Sleep(5 * time.Second) // Give CLI container a moment to be fully ready

	// Install WordPress
	wpInstallCmd := fmt.Sprintf("cd %s && docker compose -f %s/docker-compose.yml exec -T --user www-data %s_cli wp core install --url=%s --title='%s' --admin_user='%s' --admin_password='%s' --admin_email='admin@%s.com' --skip-email --debug", remotePath, remotePath, site.ProjectName, site.SiteURL, site.ProjectName, adminUsername, adminPassword, site.ProjectName)
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
			pluginInstallCmd := fmt.Sprintf("cd %s && docker compose -f %s/docker-compose.yml exec -T %s_cli wp plugin install %s --activate", remotePath, remotePath, site.ProjectName, plugin)
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
func waitForContainerHealthy(client *ssh.Client, projectName, suffix, remotePath string) error {
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

func CreateBackup(projectName string) error {
	cfg := config.LoadConfig()

	sshClient, err := GetSSHClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to VPS: %w", err)
	}
	defer sshClient.Close()

	// Find the site details to get DB credentials
	sites, err := ReadSites()
	if err != nil {
		return fmt.Errorf("failed to read sites: %w", err)
	}

	var site models.Site
	found := false
	for _, s := range sites {
		if s.ProjectName == projectName {
			site = s
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("site '%s' not found", projectName)
	}

	LogActivity(fmt.Sprintf("Backup initiated for site '%s'.", projectName))

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	backupDir := fmt.Sprintf("/var/www/backups/%s", projectName)
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	dbBackupFile := fmt.Sprintf("%s_db_backup_%s.sql", projectName, timestamp)
	filesBackupFile := fmt.Sprintf("%s_files_backup_%s.tar.gz", projectName, timestamp)
	finalBackupFile := fmt.Sprintf("backup-%s.tar.gz", timestamp)

	// 1. Ensure backup directory exists
	utils.LogInfo("Ensuring backup directory exists: %s", backupDir)
	_, _, err = RunSSHCommand(sshClient, fmt.Sprintf("sudo install -d -o %s -g %s %s", cfg.SSHUser, cfg.SSHUser, backupDir))
	if err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 2. Dump the database
	utils.LogInfo("Dumping database for site '%s'...", projectName)
	dbDumpCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml exec -T -e MYSQL_PWD='%s' %s_db mariadb-dump -u root %s > %s/%s", remotePath, site.DBPassword, projectName, site.DBName, remotePath, dbBackupFile)
	_, _, err = RunSSHCommand(sshClient, dbDumpCmd)
	if err != nil {
		LogActivity(fmt.Sprintf("Failed to dump database for site '%s'.", projectName))
		return fmt.Errorf("failed to dump database: %w", err)
	}

	// 3. Archive the wp-content directory
	utils.LogInfo("Archiving wp-content for site '%s'...", projectName)
	filesArchiveCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml exec -T %s_wordpress tar -czf - -C /var/www/html wp-content > %s", remotePath, projectName, filesBackupFile)
	_, _, err = RunSSHCommand(sshClient, filesArchiveCmd)
	if err != nil {
		LogActivity(fmt.Sprintf("Failed to archive files for site '%s'.", projectName))
		return fmt.Errorf("failed to archive files: %w", err)
	}

	// 4. Bundle database and files into a single archive
	utils.LogInfo("Bundling backup for site '%s'வுகளை...", projectName)
	bundleCmd := fmt.Sprintf("cd %s && tar -czf %s %s %s", remotePath, finalBackupFile, dbBackupFile, filesBackupFile)
	_, _, err = RunSSHCommand(sshClient, bundleCmd)
	if err != nil {
		LogActivity(fmt.Sprintf("Failed to bundle backup for site '%s'.", projectName))
		return fmt.Errorf("failed to bundle backup: %w", err)
	}

	// 5. Move final backup to the central backup directory
	utils.LogInfo("Moving final backup to %s", backupDir)
	moveCmd := fmt.Sprintf("mv %s/%s %s/", remotePath, finalBackupFile, backupDir)
	_, _, err = RunSSHCommand(sshClient, moveCmd)
	if err != nil {
		return fmt.Errorf("failed to move backup to final destination: %w", err)
	}

	// 6. Clean up temporary files
	utils.LogInfo("Cleaning up temporary files for site '%s'வுகளை...", projectName)
	cleanupCmd := fmt.Sprintf("rm %s/%s %s/%s", remotePath, dbBackupFile, remotePath, filesBackupFile)
	_, _, err = RunSSHCommand(sshClient, cleanupCmd)
	if err != nil {
		// This is not a fatal error, so just log it
		utils.LogError("Failed to clean up temporary backup files: %v", err)
	}

	LogActivity(fmt.Sprintf("Backup created successfully for site '%s'.", projectName))
	utils.LogInfo("Backup for site '%s' completed successfully.", projectName)

	return nil
}

// ListBackups lists the backups for a given site.
func ListBackups(projectName string) ([]string, error) {
	cfg := config.LoadConfig()

	sshClient, err := GetSSHClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to VPS: %w", err)
	}
	defer sshClient.Close()

	backupDir := fmt.Sprintf("/var/www/backups/%s", projectName)

	// List files in the backup directory
	// The `find ... -printf` command is used to get just the filenames, sorted by time, newest first.
	listCmd := fmt.Sprintf("find %s -maxdepth 1 -type f -printf '%%T@ %%f\n' | sort -nr | cut -d' ' -f2-", backupDir)
	stdout, stderr, err := RunSSHCommand(sshClient, listCmd)
	if err != nil {
		// If the directory doesn't exist, ls will error. We can treat this as an empty list.
		if strings.Contains(stderr, "No such file or directory") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list backups: %w, stderr: %s", err, stderr)
	}

	if stdout == "" {
		return []string{}, nil
	}

	backups := strings.Split(strings.TrimSpace(stdout), "\n")
	return backups, nil
}



// RestoreBackup restores a WordPress site from a backup.
func RestoreBackup(projectName, backupFile string) error {
	cfg := config.LoadConfig()

	sshClient, err := GetSSHClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to VPS: %w", err)
	}
	defer sshClient.Close()

	// Find the site details to get DB credentials
	sites, err := ReadSites()
	if err != nil {
		return fmt.Errorf("failed to read sites: %w", err)
	}

	var site models.Site
	found := false
	for _, s := range sites {
		if s.ProjectName == projectName {
			site = s
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("site '%s' not found", projectName)
	}

	LogActivity(fmt.Sprintf("Restore initiated for site '%s' from backup '%s'.", projectName, backupFile))

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	backupDir := fmt.Sprintf("/var/www/backups/%s", projectName)
	backupPath := filepath.Join(backupDir, backupFile)
	restoreTempDir := filepath.Join(remotePath, "restore_temp")

	// 1. Create a temporary directory for restore
	utils.LogInfo("Creating temporary directory for restore: %s", restoreTempDir)
	_, _, err = RunSSHCommand(sshClient, fmt.Sprintf("mkdir -p %s", restoreTempDir))
	if err != nil {
		return fmt.Errorf("failed to create temporary restore directory: %w", err)
	}
	// Defer cleanup of the temporary directory
	defer func() {
		utils.LogInfo("Cleaning up temporary restore directory: %s", restoreTempDir)
		RunSSHCommand(sshClient, fmt.Sprintf("rm -rf %s", restoreTempDir))
	}()

	// 2. Copy backup to temp directory and extract it
	utils.LogInfo("Extracting backup file: %s", backupPath)
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s", backupPath, restoreTempDir)
	_, _, err = RunSSHCommand(sshClient, extractCmd)
	if err != nil {
		return fmt.Errorf("failed to extract backup file: %w", err)
	}

	// Find the extracted files
	dbBackupFile, _, err := RunSSHCommand(sshClient, fmt.Sprintf("find %s -name '*_db_backup_*.sql' -print -quit", restoreTempDir))
	if err != nil || dbBackupFile == "" {
		return fmt.Errorf("could not find database backup file in extracted archive: %w", err)
	}
	dbBackupFile = strings.TrimSpace(dbBackupFile)

	filesBackupFile, _, err := RunSSHCommand(sshClient, fmt.Sprintf("find %s -name '*_files_backup_*.tar.gz' -print -quit", restoreTempDir))
	if err != nil || filesBackupFile == "" {
		return fmt.Errorf("could not find files backup file in extracted archive: %w", err)
	}
	filesBackupFile = strings.TrimSpace(filesBackupFile)

	// 3. Stop the site
	utils.LogInfo("Stopping site '%s' for restore...", projectName)
	stopCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml stop", remotePath)
	stdout, stderr, err := RunSSHCommand(sshClient, stopCmd)
	if err != nil {
		utils.LogError("Failed to stop site '%s': %v. Stdout: %s, Stderr: %s", projectName, err, stdout, stderr)
		return fmt.Errorf("failed to stop site: %w", err)
	}
	utils.LogInfo("Site '%s' stopped successfully.", projectName)

	// 4. Start db service
	utils.LogInfo("Starting db service for site '%s'...", projectName)
	startDbCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml start %s_db", remotePath, projectName)
	stdout, stderr, err = RunSSHCommand(sshClient, startDbCmd)
	if err != nil {
		utils.LogError("Failed to start db service for site '%s': %v. Stdout: %s, Stderr: %s", projectName, err, stdout, stderr)
		return fmt.Errorf("failed to start db service: %w", err)
	}
	utils.LogInfo("db service for site '%s' started successfully.", projectName)

	// Wait for the container to be healthy
	utils.LogInfo("Waiting for db container to be healthy for site '%s'...", projectName)
	if err := waitForContainerHealthy(sshClient, projectName, "_db", remotePath); err != nil {
		return fmt.Errorf("db container did not become ready: %w", err)
	}
	utils.LogInfo("db container for site '%s' is ready.", projectName)

	// 5. Restore the database
	utils.LogInfo("Restoring database for site '%s'...", projectName)
	dbRestoreCmd := fmt.Sprintf("cd %s && cat %s | docker compose -f docker-compose.yml exec -T -e MYSQL_PWD='%s' %s_db mariadb -u root %s", remotePath, dbBackupFile, site.DBPassword, projectName, site.DBName)
	stdout, stderr, err = RunSSHCommand(sshClient, dbRestoreCmd)
	if err != nil {
		utils.LogError("Failed to restore database for site '%s': %v. Stdout: %s, Stderr: %s", projectName, err, stdout, stderr)
		return fmt.Errorf("failed to restore database: %w, stderr: %s", err, stderr)
	}
	utils.LogInfo("Database for site '%s' restored successfully.", projectName)

	// 6. Start wordpress service
	utils.LogInfo("Starting wordpress service for site '%s'...", projectName)
	startWpCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml start %s_wordpress", remotePath, projectName)
	stdout, stderr, err = RunSSHCommand(sshClient, startWpCmd)
	if err != nil {
		utils.LogError("Failed to start wordpress service for site '%s': %v. Stdout: %s, Stderr: %s", projectName, err, stdout, stderr)
		return fmt.Errorf("failed to start wordpress service: %w", err)
	}
	utils.LogInfo("wordpress service for site '%s' started successfully.", projectName)

	// 7. Restore the wp-content directory
	utils.LogInfo("Restoring wp-content for site '%s'...", projectName)
	// Create a temporary directory inside the container
	tmpRestoreDir := "/tmp/restore_wp_content"
	mkdirCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml exec -T %s_wordpress mkdir -p %s", remotePath, projectName, tmpRestoreDir)
	_, _, err = RunSSHCommand(sshClient, mkdirCmd)
	if err != nil {
		return fmt.Errorf("failed to create temporary directory inside container: %w", err)
	}

	// Extract files to the temporary directory
	extractCmd = fmt.Sprintf("cd %s && cat %s | docker compose -f docker-compose.yml exec -T %s_wordpress tar -xzf - -C %s", remotePath, filesBackupFile, projectName, tmpRestoreDir)
	_, stderr, err = RunSSHCommand(sshClient, extractCmd)
	if err != nil {
		return fmt.Errorf("failed to extract files to temporary directory: %w, stderr: %s", err, stderr)
	}

	// Remove old wp-content
	rmCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml exec -T %s_wordpress rm -rf /var/www/html/wp-content", remotePath, projectName)
	_, stderr, err = RunSSHCommand(sshClient, rmCmd)
	if err != nil {
		return fmt.Errorf("failed to remove old wp-content: %w, stderr: %s", err, stderr)
	}

	// Move the restored wp-content to the correct location
	moveCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml exec -T %s_wordpress mv %s/var/www/html/wp-content /var/www/html/wp-content", remotePath, projectName, tmpRestoreDir)
	_, stderr, err = RunSSHCommand(sshClient, moveCmd)
	if err != nil {
		// This is for old backups. New backups will not have the full path.
		// If the move fails, it means it's a new backup, so we try to move the content of the directory.
		moveCmd = fmt.Sprintf("cd %s && docker compose -f docker-compose.yml exec -T %s_wordpress mv %s/wp-content /var/www/html/wp-content", remotePath, projectName, tmpRestoreDir)
		_, stderr, err = RunSSHCommand(sshClient, moveCmd)
		if err != nil {
			return fmt.Errorf("failed to move restored wp-content: %w, stderr: %s", err, stderr)
		}
	}

	// Cleanup the temporary directory
	rmTmpDirCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml exec -T %s_wordpress rm -rf %s", remotePath, projectName, tmpRestoreDir)
	_, _, err = RunSSHCommand(sshClient, rmTmpDirCmd)
	if err != nil {
		utils.LogError("Failed to cleanup temporary restore directory inside container: %v", err)
	}

	// 8. Start all services
	utils.LogInfo("Starting all services for site '%s' after restore...", projectName)
	startAllCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yml start", remotePath)
	stdout, stderr, err = RunSSHCommand(sshClient, startAllCmd)
	if err != nil {
		utils.LogError("Failed to start all services for site '%s': %v. Stdout: %s, Stderr: %s", projectName, err, stdout, stderr)
		return fmt.Errorf("failed to start all services after restore: %w", err)
	}
	utils.LogInfo("All services for site '%s' started successfully.", projectName)

	LogActivity(fmt.Sprintf("Site '%s' restored successfully from backup '%s'.", projectName, backupFile))
	utils.LogInfo("Site '%s' restored successfully.", projectName)

	return nil
}




