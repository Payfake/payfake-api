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

func Setup(db *gorm.DB, jwtSecret, accessExpiry, refreshExpiry, frontendURL, appEnv string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Single CORS middleware on root engine, runs before everything.
	// Handles OPTIONS preflight for every route in one place.
	r.Use(middleware.CORS(frontendURL))

	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.Use(middleware.RateLimit(200, time.Minute))

	isProd := appEnv == "production"

	// Repositories
	merchantRepo := repository.NewMerchantRepository(db)
	customerRepo := repository.NewCustomerRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	chargeRepo := repository.NewChargeRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	scenarioRepo := repository.NewScenarioRepository(db)
	logRepo := repository.NewLogRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	// Services
	authSvc := service.NewAuthService(merchantRepo, jwtSecret, accessExpiry, refreshExpiry)
	merchantSvc := service.NewMerchantService(merchantRepo)
	customerSvc := service.NewCustomerService(customerRepo)
	simulatorSvc := service.NewSimulatorService(scenarioRepo)
	webhookSvc := service.NewWebhookService(webhookRepo, merchantRepo)
	txSvc := service.NewTransactionService(transactionRepo, customerSvc, merchantRepo)
	chargeSvc := service.NewChargeService(chargeRepo, transactionRepo, merchantRepo, simulatorSvc, webhookSvc)
	scenarioSvc := service.NewScenarioService(scenarioRepo)
	logSvc := service.NewLogService(logRepo)
	statsSvc := service.NewStatsService(statsRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(db, authSvc, isProd)
	merchantHandler := handler.NewMerchantHandler(db, merchantSvc, authSvc)
	transactionHandler := handler.NewTransactionHandler(db, txSvc)
	chargeHandler := handler.NewChargeHandler(db, chargeSvc)
	customerHandler := handler.NewCustomerHandler(db, customerSvc, txSvc)
	controlHandler := handler.NewControlHandler(db, scenarioSvc, webhookSvc, txSvc, logSvc, authSvc)
	statsHandler := handler.NewStatsHandler(db, statsSvc, authSvc)

	r.GET("/health", handler.HealthCheck())

	// Public = no auth middleware, access_code authenticates
	// Public checkout — add submit endpoints
	public := r.Group("/api/v1/public")
	{
		public.GET("/transaction/:access_code", transactionHandler.PublicFetchByAccessCode)
		public.POST("/charge/card", chargeHandler.PublicChargeCard)
		public.POST("/charge/mobile_money", chargeHandler.PublicChargeMobileMoney)
		public.POST("/charge/bank", chargeHandler.PublicChargeBank)

		// Public submit endpoints — called from checkout page
		public.POST("/charge/submit_pin", chargeHandler.PublicSubmitPIN)
		public.POST("/charge/submit_otp", chargeHandler.PublicSubmitOTP)
		public.POST("/charge/submit_birthday", chargeHandler.PublicSubmitBirthday)
	}

	// 3DS simulation, public, called from checkout page after fake 3DS form
	r.POST("/api/v1/simulate/3ds/:reference", chargeHandler.Simulate3DS)

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.Refresh)

		protected := auth.Group("")
		protected.Use(middleware.RequireJWT())
		{
			protected.POST("/logout", authHandler.Logout)
			protected.GET("/me", authHandler.Me)
			protected.GET("/keys", authHandler.GetKeys)
			protected.POST("/keys/regenerate", authHandler.RegenerateKeys)
		}
	}

	merchant := v1.Group("/merchant")
	merchant.Use(middleware.RequireJWT())
	{
		merchant.GET("", merchantHandler.GetProfile)
		merchant.PUT("", merchantHandler.UpdateProfile)
		merchant.PUT("/password", merchantHandler.ChangePassword)
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

		// Multi-step flow submission endpoints
		charge.POST("/submit_pin", chargeHandler.SubmitPIN)
		charge.POST("/submit_otp", chargeHandler.SubmitOTP)
		charge.POST("/submit_birthday", chargeHandler.SubmitBirthday)
		charge.POST("/submit_address", chargeHandler.SubmitAddress)
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
		control.GET("/stats", statsHandler.GetStats)
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
