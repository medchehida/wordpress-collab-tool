package utils

import (
	"log"
)

// Logger provides a basic logging utility.
// In a real application, this would be more sophisticated,
// potentially using a logging framework like Zap or Logrus.
func LogInfo(message string, args ...interface{}) {
	log.Printf("INFO: "+message, args...)
}

func LogError(message string, args ...interface{}) {
	log.Printf("ERROR: "+message, args...)
}
