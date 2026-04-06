package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChargeHandler struct {
	db *gorm.DB
}

func NewChargeHandler(db *gorm.DB) *ChargeHandler {
	return &ChargeHandler{db: db}
}

func (h *ChargeHandler) ChargeCard(c *gin.Context)        {}
func (h *ChargeHandler) ChargeMobileMoney(c *gin.Context) {}
func (h *ChargeHandler) ChargeBank(c *gin.Context)        {}
func (h *ChargeHandler) FetchCharge(c *gin.Context)       {}
