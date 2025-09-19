package models

import "github.com/dgrijalva/jwt-go"

// Site represents a WordPress site.
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
	LastChecked   string   `json:"lastChecked"`
}

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

// User represents a user for authentication.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Claims represents the JWT claims.
type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

// Activity represents a log entry for an action.
type Activity struct {
	Message     string `json:"message"`
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"` // e.g., "info", "error"
	ProjectName string `json:"projectName,omitempty"`
}