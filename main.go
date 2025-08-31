package main

import (
	"log"
	"wordpress-collab-tool/config"
	"wordpress-collab-tool/server"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	_ = cfg // Use cfg to avoid "declared and not used" error for now. Will be used later.

	// Setup Gin router
	router := server.SetupRouter()

	// Setup routes
	server.SetupRoutes(router)

	// Run the server
	log.Fatal(router.Run(":8081"))
}
