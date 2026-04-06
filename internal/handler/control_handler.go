package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ControlHandler struct {
	db *gorm.DB
}

func NewControlHandler(db *gorm.DB) *ControlHandler {
	return &ControlHandler{db: db}
}

func (h *ControlHandler) GetScenario(c *gin.Context)        {}
func (h *ControlHandler) UpdateScenario(c *gin.Context)     {}
func (h *ControlHandler) ResetScenario(c *gin.Context)      {}
func (h *ControlHandler) ListWebhooks(c *gin.Context)       {}
func (h *ControlHandler) RetryWebhook(c *gin.Context)       {}
func (h *ControlHandler) GetWebhookAttempts(c *gin.Context) {}
func (h *ControlHandler) ForceTransaction(c *gin.Context)   {}
func (h *ControlHandler) GetLogs(c *gin.Context)            {}
func (h *ControlHandler) ClearLogs(c *gin.Context)          {}
