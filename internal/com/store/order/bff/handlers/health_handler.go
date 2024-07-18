package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck
func HealthCheck(c *gin.Context) {
	c.String(http.StatusOK, "Ok")
}
