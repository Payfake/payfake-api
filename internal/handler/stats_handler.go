package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/response"
	"github.com/payfake/payfake-api/internal/service"
	"gorm.io/gorm"
)

type StatsHandler struct {
	db       *gorm.DB
	statsSvc *service.StatsService
	authSvc  *service.AuthService
}

func NewStatsHandler(db *gorm.DB, statsSvc *service.StatsService, authSvc *service.AuthService) *StatsHandler {
	return &StatsHandler{db: db, statsSvc: statsSvc, authSvc: authSvc}
}

// GetStats handles GET /api/v1/control/stats
func (h *StatsHandler) GetStats(c *gin.Context) {
	merchantID, ok := middleware.GetMerchantIDFromJWT(c, h.authSvc)
	if !ok {
		response.UnauthorizedErr(c, "Invalid or expired session")
		return
	}

	stats, err := h.statsSvc.GetStats(merchantID)
	if err != nil {
		response.InternalErr(c, "An error occurred, please try again later")
		return
	}

	response.Success(c, http.StatusOK, "Stats retrieved",
		response.StatsFetched, gin.H{
			"transactions": gin.H{
				"total":        stats.TotalTransactions,
				"successful":   stats.SuccessfulCount,
				"failed":       stats.FailedCount,
				"pending":      stats.PendingCount,
				"abandoned":    stats.AbandonedCount,
				"success_rate": stats.SuccessRate,
			},
			"volume": gin.H{
				"total_amount": stats.TotalVolume,
			},
			"customers": gin.H{
				"total": stats.TotalCustomers,
			},
			"webhooks": gin.H{
				"total":     stats.TotalWebhooks,
				"delivered": stats.DeliveredWebhooks,
				"failed":    stats.FailedWebhooks,
			},
			"daily_activity": stats.DailyActivity,
		})
}
