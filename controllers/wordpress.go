package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"wordpress-collab-tool/config" // Import config
	"wordpress-collab-tool/models"
	"wordpress-collab-tool/services"
	"wordpress-collab-tool/utils"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

var jwtKey = []byte("your_secret_key") // TODO: Load from config/environment variable

// Login handles user login and JWT token generation.
func Login(c *gin.Context) {
	var user models.User
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
	claims := &models.Claims{
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

// Logout handles user logout (client-side token removal).
func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful!"})
}

// AuthMiddleware authenticates requests using JWT.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ") // Remove "Bearer " prefix

		claims := &models.Claims{}

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

// CreateWordPressSite handles the request to create a new WordPress site.
func CreateWordPressSite(c *gin.Context) {
	// Ensure the form is parsed
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil { // 10 MB max memory
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form."})
		return
	}

	projectName := c.Request.FormValue("projectName")
	selectedPlugins := c.Request.Form["selectedPlugins"]
	adminUsername := c.Request.FormValue("adminUsername")
	adminPassword := c.Request.FormValue("adminPassword")

	if projectName == "" || adminUsername == "" || adminPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name, admin username, and admin password are required."})
		return
	}

	// TODO: Generate a random port (int) and ensure uniqueness
	wpPort := 8100 + len(services.ReadSitesOrEmpty()) // Simple placeholder for now

	dbName := fmt.Sprintf("%s_db", projectName)
	dbPassword := services.GenerateRandomPassword(16)

	newSite := models.Site{
		ProjectName:   projectName,
		WPPort:        wpPort,
		DBName:        dbName,
		DBPassword:    dbPassword,
		SiteURL:       fmt.Sprintf("http://%s:%d", config.LoadConfig().SSHHost, wpPort), // Use SSHHost from config
		Plugins:       selectedPlugins,
		Status:        "creating",
		AdminUsername: adminUsername,
		AdminPassword: adminPassword,
	}

	sites, err := services.ReadSites()
	if err != nil {
		utils.LogError("Failed to read sites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save site information."})
		return
	}
	sites = append(sites, newSite)

	if err := services.WriteSites(sites); err != nil {
		utils.LogError("Failed to write sites: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save site information."})
		return
	}

	services.LogActivity(fmt.Sprintf("Site '%s' creation initiated.", projectName))

	go func() {
		err := services.DeployWordPressSite(newSite, newSite.Plugins, newSite.AdminUsername, newSite.AdminPassword)
		if err != nil {
			utils.LogError("Failed to deploy WordPress site '%s': %v", newSite.ProjectName, err)
			services.UpdateSiteStatus(newSite.ProjectName, "failed")
			services.LogActivity(fmt.Sprintf("Site '%s' creation failed: %v", newSite.ProjectName, err))
			// TODO: Implement cleanup if deployment fails
			return
		}
		services.UpdateSiteStatus(newSite.ProjectName, "active")
		services.LogActivity(fmt.Sprintf("Site '%s' created successfully! URL: %s", newSite.ProjectName, newSite.SiteURL))
	}()

	c.JSON(http.StatusOK, gin.H{"message": "WordPress deployment initiated successfully!", "url": newSite.SiteURL})
}

// GetWordPressSites retrieves a list of all WordPress sites.
func GetWordPressSites(c *gin.Context) {
	sites, err := services.ReadSites()
	if err != nil {
		services.LogActivity("Failed to retrieve sites: Error reading sites file.")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	var wg sync.WaitGroup
	for i := range sites {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status := services.GetSiteStatus(sites[i]) // Use service function
			sites[i].Status = status
		}(i)
	}
	wg.Wait()

	if err := services.WriteSites(sites); err != nil {
		utils.LogError("Error writing sites after status update: %v", err)
		// Continue with the request even if writing fails
	}

	c.JSON(http.StatusOK, sites)
}

// GetWordPressSite retrieves details for a single WordPress site.
func GetWordPressSite(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := services.ReadSites()
	if err != nil {
		services.LogActivity(fmt.Sprintf("Failed to retrieve site '%s': Error reading sites file.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	var foundSite *models.Site
	for i := range sites {
		if sites[i].ProjectName == projectName {
			status := services.GetSiteStatus(sites[i]) // Use service function
			sites[i].Status = status
			foundSite = &sites[i]
			break
		}
	}

	if foundSite == nil {
		services.LogActivity(fmt.Sprintf("Failed to retrieve site '%s': Site not found.", projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	if err := services.WriteSites(sites); err != nil {
		utils.LogError("Error writing sites after status update: %v", err)
		// Continue with the request even if writing fails
	}

	c.JSON(http.StatusOK, foundSite)
}

// DeleteWordPressSite deletes a WordPress site.
func DeleteWordPressSite(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := services.ReadSites()
	if err != nil {
		services.LogActivity(fmt.Sprintf("Failed to delete site '%s': Error reading sites file.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sites."})
		return
	}

	var siteToDelete models.Site
	found := false
	updatedSites := []models.Site{}
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

	cfg := config.LoadConfig()
	client, err := services.GetSSHClient(cfg)
	if err != nil {
		utils.LogError("Failed to connect to VPS: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to delete site '%s': Failed to connect to VPS.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	services.CleanupSite(client, projectName, siteToDelete.WPPort)

	if err := services.WriteSites(updatedSites); err != nil {
		utils.LogError("Failed to write sites: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to delete site '%s': Failed to update site information.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete site information."})
		return
	}

	services.LogActivity(fmt.Sprintf("Site '%s' deleted successfully!", projectName))
	c.JSON(http.StatusOK, gin.H{"message": "Site deleted successfully!"})
}

// RestartWordPressSite restarts a WordPress site.
func RestartWordPressSite(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := services.ReadSites()
	if err != nil {
		services.LogActivity(fmt.Sprintf("Failed to restart site '%s': Error reading sites file.", projectName))
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
		services.LogActivity(fmt.Sprintf("Failed to restart site '%s': Site not found.", projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	cfg := config.LoadConfig()
	client, err := services.GetSSHClient(cfg)
	if err != nil {
		utils.LogError("Failed to connect to VPS: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to restart site '%s': Failed to connect to VPS.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	_, _, err = services.RunSSHCommand(client, fmt.Sprintf("cd %s && docker-compose restart", remotePath))
	if err != nil {
		utils.LogError("SSH command failed: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to restart site '%s': Remote command failed.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restart remote site."})
		return
	}

	services.LogActivity(fmt.Sprintf("Site '%s' restarted successfully!", projectName))
	c.JSON(http.StatusOK, gin.H{"message": "Site restarted successfully!"})
}

// GetSitePlugins retrieves a list of plugins for a site.
func GetSitePlugins(c *gin.Context) {
	projectName := c.Param("projectName")
	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name is required."})
		return
	}

	sites, err := services.ReadSites()
	if err != nil {
		services.LogActivity(fmt.Sprintf("Failed to get plugins for site '%s': Error reading sites file.", projectName))
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
		services.LogActivity(fmt.Sprintf("Failed to get plugins for site '%s': Site not found.", projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	cfg := config.LoadConfig()
	client, err := services.GetSSHClient(cfg)
	if err != nil {
		utils.LogError("Failed to connect to VPS: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to get plugins for site '%s': Failed to connect to VPS.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	stdout, stderr, err := services.RunSSHCommand(client, fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp plugin list --field=name --format=json", remotePath, projectName))
	if err != nil {
		utils.LogError("SSH command failed: %v, output: %s", err, stdout+stderr)
		services.LogActivity(fmt.Sprintf("Failed to list plugins for site '%s': Remote command failed.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list plugins."})
		return
	}

	var plugins []string
	if err := json.Unmarshal([]byte(stdout), &plugins); err != nil {
		utils.LogError("Failed to unmarshal plugins: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to list plugins for site '%s': Failed to parse plugin list.", projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse plugin list."})
		return
	}

	services.LogActivity(fmt.Sprintf("Plugins listed for site '%s'.", projectName))
	c.JSON(http.StatusOK, plugins)
}

// InstallPlugin installs a new plugin on a site.
func InstallPlugin(c *gin.Context) {
	projectName := c.Param("projectName")
	pluginName := c.Param("pluginName")

	if projectName == "" || pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name and plugin name are required."})
		return
	}

	sites, err := services.ReadSites()
	if err != nil {
		services.LogActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Error reading sites file.", pluginName, projectName))
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
		services.LogActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Site not found.", pluginName, projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	cfg := config.LoadConfig()
	client, err := services.GetSSHClient(cfg)
	if err != nil {
		utils.LogError("Failed to connect to VPS: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Failed to connect to VPS.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	stdout, stderr, err := services.RunSSHCommand(client, fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp plugin install %s --activate", remotePath, projectName, pluginName))
	if err != nil {
		utils.LogError("SSH command failed: %v, output: %s", err, stdout+stderr)
		services.LogActivity(fmt.Sprintf("Failed to install plugin '%s' on site '%s': Remote command failed.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to install plugin."})
		return
	}

	services.LogActivity(fmt.Sprintf("Plugin '%s' installed successfully on site '%s'.", pluginName, projectName))
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Plugin %s installed successfully!", pluginName)})
}

// DeletePlugin removes a plugin from a site.
func DeletePlugin(c *gin.Context) {
	projectName := c.Param("projectName")
	pluginName := c.Param("pluginName")

	if projectName == "" || pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name and plugin name are required."})
		return
	}

	sites, err := services.ReadSites()
	if err != nil {
		services.LogActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Error reading sites file.", pluginName, projectName))
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
		services.LogActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Site not found.", pluginName, projectName))
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found."})
		return
	}

	cfg := config.LoadConfig()
	client, err := services.GetSSHClient(cfg)
	if err != nil {
		utils.LogError("Failed to connect to VPS: %v", err)
		services.LogActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Failed to connect to VPS.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	remotePath := fmt.Sprintf("/var/www/%s", projectName)
	stdout, stderr, err := services.RunSSHCommand(client, fmt.Sprintf("cd %s && docker-compose exec -T %s_cli wp plugin delete %s", remotePath, projectName, pluginName))
	if err != nil {
		utils.LogError("SSH command failed: %v, output: %s", err, stdout+stderr)
		services.LogActivity(fmt.Sprintf("Failed to delete plugin '%s' from site '%s': Remote command failed.", pluginName, projectName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to uninstall plugin."})
		return
	}

	services.LogActivity(fmt.Sprintf("Plugin '%s' uninstalled successfully from site '%s'.", pluginName, projectName))
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Plugin %s uninstalled successfully!", pluginName)})
}

// GetVPSStats retrieves VPS resource usage.
func GetVPSStats(c *gin.Context) {
	cfg := config.LoadConfig()
	client, err := services.GetSSHClient(cfg)
	if err != nil {
		utils.LogError("Failed to connect to VPS: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to VPS."})
		return
	}
	defer client.Close()

	// Get CPU usage
	cpuCmd := "top -bn1 | grep '%Cpu(s)' | awk '{print $2 + $4}'"
	stdoutCPU, stderrCPU, err := services.RunSSHCommand(client, cpuCmd)
	if err != nil {
		utils.LogError("SSH command failed for CPU: %v, output: %s", err, stdoutCPU+stderrCPU)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get CPU stats."})
		return
	}
	cpuUsage := strings.TrimSpace(stdoutCPU)

	// Get RAM usage
	ramCmd := "free -m | grep Mem | awk '{print $3/$2 * 100.0}'"
	stdoutRAM, stderrRAM, err := services.RunSSHCommand(client, ramCmd)
	if err != nil {
		utils.LogError("SSH command failed for RAM: %v, output: %s", err, stdoutRAM+stderrRAM)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get RAM stats."})
		return
	}
	ramUsage := strings.TrimSpace(stdoutRAM)

	services.LogActivity("VPS stats requested.")
	c.JSON(http.StatusOK, gin.H{
		"cpu_usage": cpuUsage,
		"ram_usage": ramUsage,
	})
}

// GetActivities retrieves a list of recent activities.
func GetActivities(c *gin.Context) {
	activities, err := services.ReadActivities() // Assuming a ReadActivities in services
	if err != nil {
		utils.LogError("Failed to retrieve activities: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve activities."})
		return
	}
	c.JSON(http.StatusOK, activities)
}
