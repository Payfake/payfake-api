package router

import (
	"time"

	"github.com/GordenArcher/payfake/internal/handler"
	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/GordenArcher/payfake/internal/repository"
	"github.com/GordenArcher/payfake/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(db *gorm.DB, jwtSecret, jwtExpiry string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.RateLimit(200, time.Minute))

	// Repositories

	merchantRepo := repository.NewMerchantRepository(db)
	customerRepo := repository.NewCustomerRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	chargeRepo := repository.NewChargeRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	scenarioRepo := repository.NewScenarioRepository(db)
	logRepo := repository.NewLogRepository(db)

	// Services

	authSvc := service.NewAuthService(merchantRepo, jwtSecret, jwtExpiry)
	customerSvc := service.NewCustomerService(customerRepo)
	simulatorSvc := service.NewSimulatorService(scenarioRepo)
	webhookSvc := service.NewWebhookService(webhookRepo, merchantRepo)
	txSvc := service.NewTransactionService(transactionRepo, customerSvc)
	chargeSvc := service.NewChargeService(chargeRepo, transactionRepo, merchantRepo, simulatorSvc, webhookSvc)
	scenarioSvc := service.NewScenarioService(scenarioRepo)
	logSvc := service.NewLogService(logRepo)

	// Handlers

	authHandler := handler.NewAuthHandler(db, authSvc)
	transactionHandler := handler.NewTransactionHandler(db, txSvc)
	chargeHandler := handler.NewChargeHandler(db, chargeSvc)
	customerHandler := handler.NewCustomerHandler(db, customerSvc, txSvc)
	controlHandler := handler.NewControlHandler(db, scenarioSvc, webhookSvc, txSvc, logSvc, authSvc)

	// Routes

	r.GET("/health", handler.HealthCheck())

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)

		protected := auth.Group("")
		protected.Use(middleware.RequireJWT())
		{
			protected.POST("/logout", authHandler.Logout)
			protected.GET("/keys", authHandler.GetKeys)
			protected.POST("/keys/regenerate", authHandler.RegenerateKeys)
		}
	}

	transaction := v1.Group("/transaction")
	transaction.Use(middleware.RequireSecretKey(db))
	{
		transaction.POST("/initialize", transactionHandler.Initialize)
		transaction.GET("/verify/:reference", transactionHandler.Verify)
		transaction.GET("", transactionHandler.List)
		transaction.GET("/:id", transactionHandler.Fetch)
		transaction.POST("/:id/refund", transactionHandler.Refund)
	}

	charge := v1.Group("/charge")
	charge.Use(middleware.RequireSecretKey(db))
	{
		charge.POST("/card", chargeHandler.ChargeCard)
		charge.POST("/mobile_money", chargeHandler.ChargeMobileMoney)
		charge.POST("/bank", chargeHandler.ChargeBank)
		charge.GET("/:reference", chargeHandler.FetchCharge)
	}

	r.GET("/pay/:access_code", handler.CheckoutPage())

	// Public checkout routes, authenticated via access_code embedded in the
	// request body, not a secret key. Safe to call from the browser because
	// access codes are single-use, short-lived, and tied to one transaction.
	// The secret key never touches the frontend.
	public := v1.Group("/public")
	{
		public.POST("/charge/card", chargeHandler.PublicChargeCard)
		public.POST("/charge/mobile_money", chargeHandler.PublicChargeMobileMoney)
		public.POST("/charge/bank", chargeHandler.PublicChargeBank)
		public.GET("/pay/:access_code", handler.CheckoutPage())
	}

	customer := v1.Group("/customer")
	customer.Use(middleware.RequireSecretKey(db))
	{
		customer.POST("", customerHandler.Create)
		customer.GET("", customerHandler.List)
		customer.GET("/:code", customerHandler.Fetch)
		customer.PUT("/:code", customerHandler.Update)
		customer.GET("/:code/transactions", customerHandler.Transactions)
	}

	control := v1.Group("/control")
	control.Use(middleware.RequireJWT())
	{
		control.GET("/scenario", controlHandler.GetScenario)
		control.PUT("/scenario", controlHandler.UpdateScenario)
		control.POST("/scenario/reset", controlHandler.ResetScenario)
		control.GET("/webhooks", controlHandler.ListWebhooks)
		control.POST("/webhooks/:id/retry", controlHandler.RetryWebhook)
		control.GET("/webhooks/:id/attempts", controlHandler.GetWebhookAttempts)
		control.POST("/transactions/:ref/force", controlHandler.ForceTransaction)
		control.GET("/logs", controlHandler.GetLogs)
		control.DELETE("/logs", controlHandler.ClearLogs)
	}

	return r
}
