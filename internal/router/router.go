package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/payfake/payfake-api/internal/handler"
	"github.com/payfake/payfake-api/internal/middleware"
	"github.com/payfake/payfake-api/internal/repository"
	"github.com/payfake/payfake-api/internal/service"
	"gorm.io/gorm"
)

type RouterResult struct {
	Engine     *gin.Engine
	WebhookSvc *service.WebhookService
	TxSvc      *service.TransactionService
}

func Setup(db *gorm.DB, jwtSecret, accessExpiry, refreshExpiry, frontendURL, appEnv string) RouterResult {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		c.Next()
	})
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
	otpRepo := repository.NewOTPRepository(db)

	// Services
	authSvc := service.NewAuthService(merchantRepo, jwtSecret, accessExpiry, refreshExpiry)
	merchantSvc := service.NewMerchantService(merchantRepo)
	customerSvc := service.NewCustomerService(customerRepo)
	simulatorSvc := service.NewSimulatorService(scenarioRepo)
	webhookSvc := service.NewWebhookService(webhookRepo, merchantRepo)
	txSvc := service.NewTransactionService(transactionRepo, customerSvc, merchantRepo, frontendURL)
	chargeSvc := service.NewChargeService(chargeRepo, transactionRepo, merchantRepo, customerRepo, otpRepo, simulatorSvc, webhookSvc, frontendURL)
	scenarioSvc := service.NewScenarioService(scenarioRepo)
	logSvc := service.NewLogService(logRepo)
	statsSvc := service.NewStatsService(statsRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(db, authSvc, isProd)
	merchantHandler := handler.NewMerchantHandler(db, merchantSvc, authSvc)
	webhookHandler := handler.NewWebhookHandler(db, merchantSvc, authSvc)
	transactionHandler := handler.NewTransactionHandler(db, txSvc, chargeSvc)
	chargeHandler := handler.NewChargeHandler(db, chargeSvc)
	customerHandler := handler.NewCustomerHandler(db, customerSvc, txSvc)
	controlHandler := handler.NewControlHandler(db, scenarioSvc, webhookSvc, txSvc, logSvc, authSvc, customerSvc, otpRepo)
	statsHandler := handler.NewStatsHandler(db, statsSvc, authSvc)

	// Health
	r.GET("/health", handler.HealthCheck())

	//
	// PAYSTACK-COMPATIBLE ROUTES
	// These mirror https://api.paystack.co exactly.
	// No /api/v1 prefix — developers change only the base URL.
	//

	// Transaction — matches https://api.paystack.co/transaction/*
	transaction := r.Group("/transaction")
	transaction.Use(middleware.PrivateCORS(frontendURL))
	transaction.Use(middleware.RequireSecretKey(db))
	{
		transaction.POST("/initialize", transactionHandler.Initialize)
		transaction.GET("/verify/:reference", transactionHandler.Verify)
		transaction.GET("", transactionHandler.List)
		transaction.GET("/:id", transactionHandler.Fetch)
		transaction.POST("/:id/refund", transactionHandler.Refund)
	}

	// Charge — single unified endpoint matching https://api.paystack.co/charge
	charge := r.Group("/charge")
	charge.Use(middleware.PrivateCORS(frontendURL))
	charge.Use(middleware.RequireSecretKey(db))
	{
		charge.POST("", chargeHandler.Charge)
		charge.POST("/submit_pin", chargeHandler.SubmitPIN)
		charge.POST("/submit_otp", chargeHandler.SubmitOTP)
		charge.POST("/submit_birthday", chargeHandler.SubmitBirthday)
		charge.POST("/submit_address", chargeHandler.SubmitAddress)
		charge.POST("/resend_otp", chargeHandler.ResendOTP)
		charge.GET("/:reference", chargeHandler.FetchCharge)
	}

	// Customer — matches https://api.paystack.co/customer/*
	customer := r.Group("/customer")
	customer.Use(middleware.PrivateCORS(frontendURL))
	customer.Use(middleware.RequireSecretKey(db))
	{
		customer.POST("", customerHandler.Create)
		customer.GET("", customerHandler.List)
		customer.GET("/:code", customerHandler.Fetch)
		customer.PUT("/:code", customerHandler.Update)
		customer.GET("/:code/transactions", customerHandler.Transactions)
	}

	//
	// PAYFAKE-SPECIFIC ROUTES = /api/v1 prefix
	// No Paystack equivalent. Dashboard auth, control panel, merchant.
	//

	v1 := r.Group("/api/v1")

	// Auth (Payfake dashboard auth, no Paystack equivalent)
	auth := v1.Group("/auth")
	auth.Use(middleware.PrivateCORS(frontendURL))
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

	// Merchant profile (Payfake dashboard, no Paystack equivalent)
	merchant := v1.Group("/merchant")
	merchant.Use(middleware.PrivateCORS(frontendURL))
	merchant.Use(middleware.RequireJWT())
	{
		merchant.GET("", merchantHandler.GetProfile)
		merchant.PUT("", merchantHandler.UpdateProfile)
		merchant.PUT("/password", merchantHandler.ChangePassword)
		merchant.GET("/webhook", webhookHandler.GetWebhookURL)
		merchant.POST("/webhook", webhookHandler.UpdateWebhookURL)
		merchant.POST("/webhook/test",
			middleware.HydrateMerchantIDFromJWT(authSvc),
			middleware.RateLimitWebhookTest(),
			webhookHandler.TestWebhook,
		)
	}

	// Control panel (Payfake-specific, no Paystack equivalent)
	control := v1.Group("/control")
	control.Use(middleware.PrivateCORS(frontendURL))
	control.Use(middleware.RequireJWT())
	{
		control.GET("/stats", statsHandler.GetStats)
		control.GET("/transactions", controlHandler.ListTransactions)
		control.GET("/customers", controlHandler.ListCustomers)
		control.GET("/scenario", controlHandler.GetScenario)
		control.PUT("/scenario", controlHandler.UpdateScenario)
		control.POST("/scenario/reset", controlHandler.ResetScenario)
		control.GET("/webhooks", controlHandler.ListWebhooks)
		control.POST("/webhooks/:id/retry", controlHandler.RetryWebhook)
		control.GET("/webhooks/:id/attempts", controlHandler.GetWebhookAttempts)
		control.POST("/transactions/:ref/force", controlHandler.ForceTransaction)
		control.GET("/logs", controlHandler.GetLogs)
		control.DELETE("/logs", controlHandler.ClearLogs)
		control.GET("/otp-logs", controlHandler.GetOTPLogs)
	}

	// Public checkout (no auth, access_code authenticates)
	// Static path registered before param path to avoid Gin conflict.
	public := v1.Group("/public")
	public.Use(middleware.PublicCORS())
	{
		public.GET("/transaction/verify/:reference", transactionHandler.PublicVerify)
		public.GET("/transaction/:access_code", transactionHandler.PublicFetchByAccessCode)
		public.POST("/charge", chargeHandler.PublicCharge)
		public.POST("/charge/submit_pin", chargeHandler.PublicSubmitPIN)
		public.POST("/charge/submit_otp", chargeHandler.PublicSubmitOTP)
		public.POST("/charge/submit_birthday", chargeHandler.PublicSubmitBirthday)
		public.POST("/charge/submit_address", chargeHandler.PublicSubmitAddress)
		public.POST("/charge/resend_otp", chargeHandler.PublicResendOTP)
		public.POST("/simulate/3ds/:reference", chargeHandler.Simulate3DS)
	}

	return RouterResult{
		Engine:     r,
		WebhookSvc: webhookSvc,
		TxSvc:      txSvc,
	}
}
