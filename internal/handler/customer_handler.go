package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CustomerHandler struct {
	db *gorm.DB
}

func NewCustomerHandler(db *gorm.DB) *CustomerHandler {
	return &CustomerHandler{db: db}
}

func (h *CustomerHandler) Create(c *gin.Context)       {}
func (h *CustomerHandler) List(c *gin.Context)         {}
func (h *CustomerHandler) Fetch(c *gin.Context)        {}
func (h *CustomerHandler) Update(c *gin.Context)       {}
func (h *CustomerHandler) Transactions(c *gin.Context) {}
