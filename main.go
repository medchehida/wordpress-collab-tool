package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	// Add CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Configure your VPS details with your actual credentials
	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	router.POST("/api/login", login)
	router.POST("/api/logout", logout)

	auth := router.Group("/api")
	auth.Use(authMiddleware())
	{
		auth.POST("/sites", createWordPressSite)
		auth.GET("/sites", getWordPressSites)
		auth.GET("/sites/:projectName", getWordPressSite)
		auth.DELETE("/sites/:projectName", deleteWordPressSite)
		auth.POST("/sites/:projectName/restart", restartWordPressSite)
		auth.GET("/sites/:projectName/plugins", getSitePlugins)
		auth.POST("/sites/:projectName/plugins/:pluginName", installPlugin)
		auth.DELETE(" /sites/:projectName/plugins/:pluginName", deletePlugin)
		auth.GET("/vps/stats", getVPSStats)
		auth.GET("/activities", getActivities)
	}

	router.Run(":8081")
}