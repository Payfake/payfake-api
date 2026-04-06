package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

func (h *AuthHandler) Register(c *gin.Context)       {}
func (h *AuthHandler) Login(c *gin.Context)          {}
func (h *AuthHandler) Logout(c *gin.Context)         {}
func (h *AuthHandler) GetKeys(c *gin.Context)        {}
func (h *AuthHandler) RegenerateKeys(c *gin.Context) {}
