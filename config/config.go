package config

import (
	"log"
	"os"
)

type Config struct {
	SSHUser     string
	SSHHost     string
	SSHPassword string
}

func LoadConfig() *Config {
	sshUser := os.Getenv("SSH_USER")
	sshHost := os.Getenv("SSH_HOST")
	sshPassword := os.Getenv("SSH_PASSWORD")

	if sshUser == "" || sshHost == "" || sshPassword == "" {
		log.Fatal("SSH_USER, SSH_HOST, and SSH_PASSWORD environment variables must be set")
	}

	return &Config{
		SSHUser:     sshUser,
		SSHHost:     sshHost,
		SSHPassword: sshPassword,
	}
}
