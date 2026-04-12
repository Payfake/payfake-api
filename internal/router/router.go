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

func Setup(db *gorm.DB, jwtSecret, jwtExpiry, frontendURL string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())

	// CORS must be registered on the root engine BEFORE any group is defined.
	// The browser sends an OPTIONS preflight before every cross-origin POST —
	// if OPTIONS hits a 404 (no handler) the browser blocks the real request.
	// Registering on the root engine ensures OPTIONS is handled for every route
	// including /api/v1/public/* before group-level middleware even runs.
	r.Use(middleware.CORSPublic())

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
	txSvc := service.NewTransactionService(transactionRepo, customerSvc, merchantRepo)
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

	// Public checkout routes, no secret key required.
	// Called directly from the React checkout page running in the customer's browser.
	// Authenticated via access_code in the request body or URL param.
	// The merchant's secret key never touches the frontend at any point.
	//
	// IMPORTANT: Public routes are defined at the root level, NOT as a subgroup of v1.
	// This prevents the restrictive CORSPrivate middleware (applied to v1) from
	// interfering with the permissive CORSPublic middleware needed for the checkout.
	public := r.Group("/api/v1/public")
	{
		public.GET("/transaction/:access_code", transactionHandler.PublicFetchByAccessCode)
		public.POST("/charge/card", chargeHandler.PublicChargeCard)
		public.POST("/charge/mobile_money", chargeHandler.PublicChargeMobileMoney)
		public.POST("/charge/bank", chargeHandler.PublicChargeBank)
	}

	v1 := r.Group("/api/v1")

	// Auth, transaction, charge, customer, control routes stay exactly
	// as they are, private CORS applied to the whole v1 group.
	v1.Use(middleware.CORSPrivate(frontendURL))

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
