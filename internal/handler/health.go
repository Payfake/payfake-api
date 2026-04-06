package handler

import (
	"net/http"

	"github.com/GordenArcher/payfake/internal/response"
	"github.com/gin-gonic/gin"
)

// HealthCheck is a simple liveness probe.
// Docker, Railway, Render, and any reverse proxy will hit this
// periodically to verify the server is alive and responding.
// We return 200 with a minimal payload, no DB check here.
// If you want a readiness probe (checks DB connectivity too)
// that's a separate endpoint, liveness and readiness are
// different concerns in production infrastructure.
func HealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Success(c, http.StatusOK, "Payfake is running", response.Code("HEALTH_OK"), gin.H{
			"status":  "ok",
			"service": "payfake",
		})
	}
}
