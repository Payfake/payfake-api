package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TransactionHandler struct {
	db *gorm.DB
}

func NewTransactionHandler(db *gorm.DB) *TransactionHandler {
	return &TransactionHandler{db: db}
}

func (h *TransactionHandler) Initialize(c *gin.Context) {}
func (h *TransactionHandler) Verify(c *gin.Context)     {}
func (h *TransactionHandler) List(c *gin.Context)       {}
func (h *TransactionHandler) Fetch(c *gin.Context)      {}
func (h *TransactionHandler) Refund(c *gin.Context)     {}
