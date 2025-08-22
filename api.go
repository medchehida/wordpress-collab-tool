package main

import (
	"encoding/json"
	"fmt"

	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/dgrijalva/jwt-go"
)

var jwtKey = []byte("my_secret_key") // Replace with a strong, random key from environment variable in production

// Config holds the variables for the docker-compose template.
type Config struct {
	ProjectName string
	WPPort      int
	DBPort      int
	DBName      string
	DBPassword  string
}

// SSHConfig holds the SSH connection details for the VPS.
type SSHConfig struct {
	User     string
	Host     string
	Password string
}

func createWordPressSite(c *gin.Context) {
	projectName := c.PostForm("projectName")
	selectedPlugins := c.PostFormArray("selectedPlugins")
	adminUsername := c.PostForm("adminUsername")
	adminPassword := c.PostForm("adminPassword")

	if projectName == "" || adminUsername == "" || adminPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name, admin username, and admin password are required."})
		return
	}

	// Dynamically generate a random port to avoid conflicts
	wpPort := rand.Intn(1000) + 8100 // Generates a port between 8100 and 9099
	dbName := fmt.Sprintf("%s_db", projectName)
	dbPassword := generateRandomPassword(16) // Generate a secure random password

	// Create a dynamic configuration for the template
	cfg := Config{
		ProjectName: projectName,
		WPPort:      wpPort,
		DBName:      dbName,
		DBPassword:  dbPassword,
	}

	// Save initial site information with "creating" status
	sites, err := readSites()
	if err != nil {
		log.Printf("Failed to read sites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save site information."})
		return
	}

	newSite := Site{
		ProjectName:   projectName,
		WPPort:        wpPort,
		DBName:        dbName,
		DBPassword:    dbPassword,
		SiteURL:       fmt.Sprintf("http://%s:%d", os.Getenv("SSH_HOST"), wpPort),
		Plugins:       selectedPlugins,
		Status:        "creating",
		AdminUsername: adminUsername,
		AdminPassword: adminPassword,
	}
	sites = append(sites, newSite)

	if err := writeSites(sites); err != nil {
		log.Printf("Failed to write sites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save site information."})
		return
	}

	logActivity(fmt.Sprintf("Site '%s' creation initiated.", projectName))

	go func() {
		sshUser := os.Getenv("SSH_USER")
		sshHost := os.Getenv("SSH_HOST")
		sshPassword := os.Getenv("SSH_PASSWORD")

		if sshUser == "" || sshHost == "" || sshPassword == "" {
			log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
		}

		sshConfig := &SSHConfig{
			User:     sshUser,
			Host:     sshHost,
			Password: sshPassword,
		}

		client, err := getSSHClient(sshConfig)
		if err != nil {
			log.Printf("Failed to connect to VPS: %v", err)
			cleanupSite(client, projectName, wpPort)
			return
		}
		defer client.Close()

		// Generate the docker-compose.yml file on the local machine
		tmpl, err := template.ParseFiles("templates/template.yml")
		if err != nil {
			log.Printf("Failed to parse template: %v", err)
			cleanupSite(client, projectName, wpPort)
			return
		}

		// Create the remote directory with sudo and change ownership
		remotePath := fmt.Sprintf("/var/www/%s", projectName)

		stdout, stderr, err := runSSHCommand(client, fmt.Sprintf("sudo install -d -o %s -g %s %s", sshConfig.User, sshConfig.User, remotePath))
		if err != nil {
			log.Printf("Failed to create and set ownership of remote directory: %v, stdout: %s, stderr: %s", err, stdout, stderr)
			cleanupSite(client, projectName, wpPort)
			return
		}

		// SFTP client to transfer the file
		sftpClient, err := sftp.NewClient(client)
		if err != nil {
			log.Printf("Failed to create SFTP client: %v", err)
			cleanupSite(client, projectName, wpPort)
			return
		}
		defer sftpClient.Close()

		// Create the remote docker-compose.yml file
		remoteFile, err := sftpClient.Create(filepath.Join(remotePath, "docker-compose.yml"))
		if err != nil {
			log.Printf("Failed to create remote file: %v", err)
			cleanupSite(client, projectName, wpPort)
			return
		}
		defer remoteFile.Close()

		// Execute the template and write it to the remote file
		if err := tmpl.Execute(remoteFile, cfg); err != nil {
			log.Printf("Failed to execute template: %v", err)
			cleanupSite(client, projectName, wpPort)
			return
		}

		// Run docker-compose up -d
		log.Printf("Running docker-compose up -d in %s", remotePath)
		stdout, stderr, err = runSSHCommand(client, fmt.Sprintf("cd %s && docker-compose up -d", remotePath))
		if err != nil {
			log.Printf("Failed to run docker-compose up: %v, stdout: %s, stderr: %s", err, stdout, stderr)
			cleanupSite(client, projectName, wpPort)
			return
		}
		log.Printf("docker-compose up -d command executed. stdout: %s, stderr: %s", stdout, stderr)
		time.Sleep(5 * time.Second)

		// Wait for the WordPress container to be ready
		log.Printf("Waiting for WordPress container to be ready for site '%s'...", projectName)
		ready := false
		for i := 0; i < 24; i++ { // 2 minutes timeout
			containerName := fmt.Sprintf("%s_wordpress", projectName)
			cmd := fmt.Sprintf("docker inspect --format='{{.State.Health.Status}}' %s", containerName)
			stdout, stderr, err := runSSHCommand(client, cmd)
			log.Printf("Checking WordPress container health: %s. Raw stdout: '%s', Raw stderr: '%s', Error: %v", cmd, stdout, stderr)
			if err == nil && strings.TrimSpace(stdout) == "healthy" {
				ready = true
				break
			}
			log.Printf("Waiting for container to be healthy... attempt %d/24, status: %s, stderr: %s, err: %v", i+1, strings.TrimSpace(stdout), stderr, err)
			time.Sleep(5 * time.Second)
		}

		if !ready {
			log.Printf("WordPress container for site '%s' did not become ready in time.", projectName)
			cleanupSite(client, projectName, wpPort)
			return
		}
		log.Printf("WordPress container for site '%s' is ready.", projectName)

		// Wait for the CLI container to be ready
		log.Printf("Waiting for CLI container to be ready for site '%s'...", projectName)
		ready = false
		for i := 0; i < 24; i++ { // 2 minutes timeout
			containerName := fmt.Sprintf("%s_cli", projectName)
			cmd := fmt.Sprintf("docker inspect --format='{{.State.Health.Status}}' %s", containerName)
			log.Printf("Checking CLI container health: %s. Raw stdout: '%s', Raw stderr: '%s', Error: %v", cmd, stdout, stderr)
			stdout, stderr, err := runSSHCommand(client, cmd)
			if err == nil && strings.TrimSpace(stdout) == "healthy" {
				ready = true
				break
			}
			log.Printf("Waiting for CLI container to be healthy... attempt %d/24, status: %s, stderr: %s, err: %v", i+1, strings.TrimSpace(stdout), stderr, err)
			time.Sleep(5 * time.Second)
		}

		if !ready {
			log.Printf("CLI container for site '%s' did not become ready in time.", projectName)
			cleanupSite(client, projectName, wpPort)
			return
		}
		log.Printf("CLI container for site '%s' is ready.", projectName)
		time.Sleep(5 * time.Second)

		// Install WordPress
		wpInstallCmd := fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp core install --url=%s --title='%s' --admin_user='%s' --admin_password='%s' --admin_email='admin@%s.com' --skip-email --debug", remotePath, projectName, newSite.SiteURL, projectName, adminUsername, adminPassword, projectName)
		stdout, stderr, err = runSSHCommand(client, wpInstallCmd)
		if err != nil {
			log.Printf("Failed to install WordPress for site '%s'. Command: '%s', Error: %v, stdout: %s, stderr: %s", projectName, wpInstallCmd, err, stdout, stderr)
			cleanupSite(client, projectName, wpPort)
			return
		}

		// Install selected plugins
		for _, plugin := range selectedPlugins {
			pluginInstallCmd := fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp plugin install %s --activate", remotePath, projectName, plugin)
			stdout, stderr, err = runSSHCommand(client, pluginInstallCmd)
			if err != nil {
				log.Printf("Failed to install plugin %s: %v, stdout: %s, stderr: %s", plugin, err, stdout, stderr)
				logActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s'.", plugin, projectName))
			}
		}

		updateSiteStatus(projectName, "active")
		logActivity(fmt.Sprintf("Site '%s' created successfully! URL: %s", projectName, newSite.SiteURL))
	}()

	c.JSON(http.StatusOK, gin.H{"message": "WordPress deployment initiated successfully!", "url": newSite.SiteURL})
}

func runSSHCommand(client *ssh.Client, cmd string) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stdout, stderr strings.Builder
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("SSH command failed: %w", err)
	}

	return stdout.String(), stderr.String(), nil
}

func updateSiteStatus(projectName, status string) {
	sites, err := readSites()
	if err != nil {
		log.Printf("Error reading sites to update status: %v", err)
		return
	}

	for i, site := range sites {
		if site.ProjectName == projectName {
			sites[i].Status = status
			break
		}
	}

	if err := writeSites(sites); err != nil {
		log.Printf("Error writing sites after status update: %v", err)
	}
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func cleanupSite(client *ssh.Client, projectName string, port int) {
	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	log.Printf("Cleaning up site '%s'...", projectName)

	// Check if the directory exists
	_, _, err := runSSHCommand(client, fmt.Sprintf("stat %s", remotePath))
	if err == nil {
		// Directory exists, so run docker-compose down
		cleanupCmd := fmt.Sprintf("cd %s && docker-compose down --volumes && cd /var/www && sudo rm -rf %s", remotePath, projectName)
		stdout, stderr, err := runSSHCommand(client, cleanupCmd)
		if err != nil {
			log.Printf("Failed to cleanup site directory for '%s': %v, stdout: %s, stderr: %s", projectName, err, stdout, stderr)
			logActivity(fmt.Sprintf("Failed to cleanup site directory for '%s'. Manual intervention may be required.", projectName))
			// Continue with deletion even if cleanup fails
		}
	} else {
		// Directory does not exist, check for orphaned container by name and status
		log.Printf("Directory for site '%s' not found. Checking for orphaned containers.", projectName)
		checkCmd := fmt.Sprintf("docker ps -a --filter 'name=%s_wordpress' --format '{{.Status}}'", projectName)
		stdout, _, err := runSSHCommand(client, checkCmd)
		if err != nil {
			log.Printf("Failed to check for orphaned containers for site '%s': %v", projectName, err)
		} else {
			status := strings.TrimSpace(stdout)
			log.Printf("Orphaned container status for '%s': %s", projectName, status)
			// Only cleanup if the container is truly exited (not running, restarting, or paused)
			if strings.HasPrefix(status, "Exited") {
				stopCmd := fmt.Sprintf("docker rm %s_wordpress", projectName)
				stdout, stderr, err := runSSHCommand(client, stopCmd)
				if err != nil {
					log.Printf("Failed to remove orphaned container for site '%s': %v, stdout: %s, stderr: %s", projectName, err, stdout, stderr)
				}
			}
		}
	}
}

func getWordPressSites(c *gin.Context) {
	sites, err := readSites()
	if err != nil {
		logActivity("Failed to retrieve sites: Error reading sites file.")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}
	c.JSON(http.StatusOK, sites)
}

func getWordPressSite(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := readSites()
	if err != nil {
		logActivity(fmt.Sprintf("Failed to retrieve site '%s': Error reading sites file.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	for _, site := range sites {
		if site.ProjectName == projectName {
			c.JSON(http.StatusOK, site)
			return
		}
	}

	logActivity(fmt.Sprintf("Failed to retrieve site '%s': Site not found.", projectName))
	c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
}

func deleteWordPressSite(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := readSites()
	if err != nil {
		logActivity(fmt.Sprintf("Failed to delete site '%s': Error reading sites file.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	var siteToDelete Site
	found := false
	updatedSites := []Site{}
	for _, site := range sites {
		if site.ProjectName == projectName {
			siteToDelete = site
			found = true
		} else {
			updatedSites = append(updatedSites, site)
		}
	}

	if !found {
		c.JSON(http.StatusOK, gin.H{"message": "Site already deleted."})
		return
	}

	// Connect to the VPS via SSH
	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	sshConfig := &SSHConfig{
		User:     sshUser,
		Host:     sshHost,
		Password: sshPassword,
	}

	client, err := getSSHClient(sshConfig)
	if err != nil {
		log.Printf("Failed to connect to VPS: %v", err)
		logActivity(fmt.Sprintf("Failed to delete site '%s': Failed to connect to VPS.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	cleanupSite(client, projectName, siteToDelete.WPPort)

	if err := writeSites(updatedSites); err != nil {
		log.Printf("Failed to write sites: %v", err)
		logActivity(fmt.Sprintf("Failed to delete site '%s': Failed to update site information.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete site information."})
		return
	}

	logActivity(fmt.Sprintf("Site '%s' deleted successfully!", projectName))
	c.JSON(http.StatusOK, gin.H{"message": "Site deleted successfully!"})
}

func restartWordPressSite(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := readSites()
	if err != nil {
		logActivity(fmt.Sprintf("Failed to restart site '%s': Error reading sites file.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	found := false
	for _, site := range sites {
		if site.ProjectName == projectName {
			found = true
			break
		}
	}

	if !found {
		logActivity(fmt.Sprintf("Failed to restart site '%s': Site not found.", projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	// Connect to the VPS via SSH
	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	sshConfig := &SSHConfig{
		User:     sshUser,
		Host:     sshHost,
		Password: sshPassword,
	}

	client, err := getSSHClient(sshConfig)
	if err != nil {
		log.Printf("Failed to connect to VPS: %v", err)
		logActivity(fmt.Sprintf("Failed to restart site '%s': Failed to connect to VPS.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	// Restart docker containers
	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	session, err := client.NewSession()
	if err != nil {
		log.Printf("Failed to create SSH session: %v", err)
		logActivity(fmt.Sprintf("Failed to restart site '%s': Failed to create SSH session.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SSH session."})
		return
	}
	defer session.Close()

	cmd := fmt.Sprintf("cd %s && docker-compose restart", remotePath)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Printf("SSH command failed: %v, output: %s", err, string(output))
		logActivity(fmt.Sprintf("Failed to restart site '%s': Remote command failed.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restart remote site."})
		return
	}

	logActivity(fmt.Sprintf("Site '%s' restarted successfully!", projectName))
	c.JSON(http.StatusOK, gin.H{"message": "Site restarted successfully!"})
}

func getSitePlugins(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := readSites()
	if err != nil {
		logActivity(fmt.Sprintf("Failed to get plugins for site '%s': Error reading sites file.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	found := false
	for _, site := range sites {
		if site.ProjectName == projectName {
			found = true
			break
		}
	}

	if !found {
		logActivity(fmt.Sprintf("Failed to get plugins for site '%s': Site not found.", projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	sshConfig := &SSHConfig{
		User:     sshUser,
		Host:     sshHost,
		Password: sshPassword,
	}

	client, err := getSSHClient(sshConfig)
	if err != nil {
		log.Printf("Failed to connect to VPS: %v", err)
		logActivity(fmt.Sprintf("Failed to get plugins for site '%s': Failed to connect to VPS.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	session, err := client.NewSession()
	if err != nil {
		log.Printf("Failed to create SSH session: %v", err)
		logActivity(fmt.Sprintf("Failed to get plugins for site '%s': Failed to create SSH session.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SSH session."})
		return
	}
	defer session.Close()

	cmd := fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp plugin list --field=name --format=json", remotePath, projectName)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Printf("SSH command failed: %v, output: %s", err, string(output))
		logActivity(fmt.Sprintf("Failed to list plugins for site '%s': Remote command failed.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list plugins."})
		return
	}

	var plugins []string
	if err := json.Unmarshal(output, &plugins); err != nil {
		log.Printf("Failed to unmarshal plugins: %v", err)
		logActivity(fmt.Sprintf("Failed to list plugins for site '%s': Failed to parse plugin list.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse plugin list."})
		return
	}

	logActivity(fmt.Sprintf("Plugins listed for site '%s'.", projectName))
	c.JSON(http.StatusOK, plugins)
}

func installPlugin(c *gin.Context) {
	projectName := c.Param("projectName")
	pluginName := c.Param("pluginName")

	if projectName == "" || pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name and plugin name are required."})
		return
	}

	sites, err := readSites()
	if err != nil {
		logActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Error reading sites file.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	found := false
	for _, site := range sites {
		if site.ProjectName == projectName {
			found = true
			break
		}
	}

	if !found {
		logActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Site not found.", pluginName, projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	sshConfig := &SSHConfig{
		User:     sshUser,
		Host:     sshHost,
		Password: sshPassword,
	}

	client, err := getSSHClient(sshConfig)
	if err != nil {
		log.Printf("Failed to connect to VPS: %v", err)
		logActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Failed to connect to VPS.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	session, err := client.NewSession()
	if err != nil {
		log.Printf("Failed to create SSH session: %v", err)
		logActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Failed to create SSH session.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SSH session."})
		return
	}
	defer session.Close()

	cmd := fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp plugin install %s --activate", remotePath, projectName, pluginName)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Printf("SSH command failed: %v, output: %s", err, string(output))
		logActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Remote command failed.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to install plugin."})
		return
	}

	logActivity(fmt.Sprintf("Plugin '%s' installed successfully on site '%s'.", pluginName, projectName))
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Plugin %s installed successfully!", pluginName)})
}

func deletePlugin(c *gin.Context) {
	projectName := c.Param("projectName")
	pluginName := c.Param("pluginName")

	if projectName == "" || pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name and plugin name are required."})
		return
	}

	sites, err := readSites()
	if err != nil {
		logActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Error reading sites file.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	found := false
	for _, site := range sites {
		if site.ProjectName == projectName {
			found = true
			break
		}
	}

	if !found {
		logActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Site not found.", pluginName, projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	sshConfig := &SSHConfig{
		User:     sshUser,
		Host:     sshHost,
		Password: sshPassword,
	}

	client, err := getSSHClient(sshConfig)
	if err != nil {
		log.Printf("Failed to connect to VPS: %v", err)
		logActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Failed to connect to VPS.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	session, err := client.NewSession()
	if err != nil {
		log.Printf("Failed to create SSH session: %v", err)
		logActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Failed to create SSH session.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SSH session."})
		return
	}
	defer session.Close()

	cmd := fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp plugin delete %s", remotePath, projectName, pluginName)
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		log.Printf("SSH command failed: %v, output: %s", err, string(output))
		logActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Remote command failed.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to uninstall plugin."})
		return
	}

	logActivity(fmt.Sprintf("Plugin '%s' uninstalled successfully from site '%s'.", pluginName, projectName))
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Plugin %s uninstalled successfully!", pluginName)})
}

func getActivities(c *gin.Context) {
	activities, err := readActivities()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve activities."})
		return
	}
	c.JSON(http.StatusOK, activities)
}

func getVPSStats(c *gin.Context) {
	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	sshConfig := &SSHConfig{
		User:     sshUser,
		Host:     sshHost,
		Password: sshPassword,
	}

	client, err := getSSHClient(sshConfig)
	if err != nil {
		log.Printf("Failed to connect to VPS: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	// Get CPU usage
	session, err := client.NewSession()
	if err != nil {
		log.Printf("Failed to create SSH session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get CPU stats."})
		return
	}
	defer session.Close()

	cpuCmd := "top -bn1 | grep \"%Cpu(s)\" | awk '{print $2 + $4}'"
	cpuOutput, err := session.CombinedOutput(cpuCmd)
	if err != nil {
		log.Printf("SSH command failed for CPU: %v, output: %s", err, string(cpuOutput))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get CPU stats."})
		return
	}
	cpuUsage := strings.TrimSpace(string(cpuOutput))

	// Get RAM usage
	session, err = client.NewSession()
	if err != nil {
		log.Printf("Failed to create SSH session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get RAM stats."})
		return
	}
	defer session.Close()

	ramCmd := "free -m | grep Mem | awk '{print $3/$2 * 100.0}'"
	ramOutput, err := session.CombinedOutput(ramCmd)
	if err != nil {
		log.Printf("SSH command failed for RAM: %v, output: %s", err, string(ramOutput))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get RAM stats."})
		return
	}
	ramUsage := strings.TrimSpace(string(ramOutput))

	c.JSON(http.StatusOK, gin.H{
		"cpu_usage": cpuUsage,
		"ram_usage": ramUsage,
	})
}

// User represents a user for authentication.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func login(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// For now, a simple hardcoded check. Replace with proper authentication.
	if user.Username != "admin" || user.Password != "password" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials."})
		return
	}

	expirationTime := time.Now().Add(8 * time.Hour)
	claims := &Claims{
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Login successful!", "token": tokenString})
}

func logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful!"})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString = tokenString[len("Bearer "):] // Remove "Bearer " prefix

		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func getSSHClient(config *SSHConfig) (*ssh.Client, error) {
	knownHostsPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("could not create hostkeycallback: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: config.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.Password),
		},
		HostKeyCallback: hostKeyCallback,
	}
	return ssh.Dial("tcp", config.Host+":22", sshConfig)
}
