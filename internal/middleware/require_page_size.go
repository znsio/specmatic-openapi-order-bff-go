// File: middleware/require_page_size.go

package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/znsio/specmatic-order-bff-go/pkg/utils"
)

func RequirePageSize() gin.HandlerFunc {
	return func(c *gin.Context) {
		pageSizeStr := c.GetHeader("pageSize")

		// page size is required
		if pageSizeStr == "" {
			utils.ErrorResponse(c, http.StatusBadRequest, "pageSize header is required")
			c.Abort()
			return
		}

		// page size should be a valid int
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil {
			utils.ErrorResponse(c, http.StatusBadRequest, "pageSize must be a valid integer")
			c.Abort()
			return
		}

		// page size should be greater than 0.
		if pageSize <= 0 {
			utils.ErrorResponse(c, http.StatusBadRequest, "The provided page size is negative or 0, please check.")
			c.Abort()
			return
		}

		c.Set("pageSize", pageSize)
		c.Next()
	}
}
