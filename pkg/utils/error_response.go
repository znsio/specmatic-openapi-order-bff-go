package utils

import (
	"time"

	"github.com/gin-gonic/gin"
)

func ErrorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error":     message,
		"status":    statusCode,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
