package server

import (
	"github.com/gin-gonic/gin"
	"wordpress-collab-tool/controllers" // Assuming controllers package will be at this path
)

func SetupRoutes(router *gin.Engine) {
	// Public routes
	router.POST("/api/login", controllers.Login)
	router.POST("/api/logout", controllers.Logout)

	// Authenticated routes
	auth := router.Group("/api")
	auth.Use(controllers.AuthMiddleware()) // Assuming AuthMiddleware is in controllers
	{
		auth.POST("/sites", controllers.CreateWordPressSite)
		auth.GET("/sites", controllers.GetWordPressSites)
		auth.GET("/sites/:projectName", controllers.GetWordPressSite)
		auth.DELETE("/sites/:projectName", controllers.DeleteWordPressSite)
		auth.POST("/sites/:projectName/restart", controllers.RestartWordPressSite)
		auth.GET("/sites/:projectName/plugins", controllers.GetSitePlugins)
		auth.POST("/sites/:projectName/plugins/:pluginName", controllers.InstallPlugin)
		auth.DELETE(" /sites/:projectName/plugins/:pluginName", controllers.DeletePlugin)
		auth.GET("/vps/stats", controllers.GetVPSStats)
		auth.GET("/activities", controllers.GetActivities)
	}
}
