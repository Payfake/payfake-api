package router

import (
	"time"

	"github.com/GordenArcher/payfake/internal/handler"
	"github.com/GordenArcher/payfake/internal/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Setup wires every route, middleware, and handler together.
// We receive the DB instance here and pass it down to handlers
// and middleware that need it. This is manual dependency injection —
// no DI framework, just function arguments. Simple, explicit, and
// easy to trace. You always know exactly where a dependency comes from.
func Setup(db *gorm.DB) *gin.Engine {

	// gin.New() gives us a blank engine with zero middleware.
	// We use this instead of gin.Default() because gin.Default()
	// adds its own logger and recovery middleware, we want full
	// control over what middleware runs and in what order.
	r := gin.New()

	// Recovery middleware catches any panic that occurs in a handler
	// and returns a 500 instead of crashing the server.
	// This is the safety net, no single bad request should take
	// down the entire server.
	r.Use(gin.Recovery())

	// RequestID must be the first middleware after Recovery.
	// Every subsequent middleware and handler in the chain depends
	// on request_id being in the context, logger needs it, response
	// metadata needs it, introspection logs need it.
	r.Use(middleware.RequestID())

	// Logger runs after RequestID so it can include the request_id
	// in every log line. Order matters here, if Logger ran before
	// RequestID the log lines would have an empty request_id field.
	r.Use(middleware.Logger())

	// Global rate limit, applied to every route before they even
	// reach namespace-specific middleware. This is the first line of
	// defence against hammering. 200 requests per minute is generous
	// for a local simulator but tight enough to catch runaway scripts.
	r.Use(middleware.RateLimit(200, time.Minute))

	// Health check, no auth, no rate limit beyond the global one.
	// Used by Docker health checks and load balancers to verify the
	// server is alive. Returns 200 with a simple payload.

	r.GET("/health", handler.HealthCheck())

	// API v1, all routes live under /api/v1 for clean versioning.
	// When we need to make breaking changes in the future we add /api/v2
	// alongside this without touching existing integrations.

	v1 := r.Group("/api/v1")

	// Auth namespace —> /api/v1/auth
	// No secret key auth here, these routes ARE how you get keys.
	// JWT auth is applied selectively to the protected routes below.

	authHandler := handler.NewAuthHandler(db)
	auth := v1.Group("/auth")
	{
		// Public, no auth required
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)

		// Protected —> requires valid JWT (dashboard session)
		protected := auth.Group("")
		protected.Use(middleware.RequireJWT())
		{
			protected.POST("/logout", authHandler.Logout)
			protected.GET("/keys", authHandler.GetKeys)
			protected.POST("/keys/regenerate", authHandler.RegenerateKeys)
		}
	}

	// Transaction namespace —? /api/v1/transaction
	// Mirrors Paystack's transaction endpoints exactly.
	// All routes require a valid merchant secret key.

	transactionHandler := handler.NewTransactionHandler(db)
	transaction := v1.Group("/transaction")
	transaction.Use(middleware.RequireSecretKey(db))
	{
		transaction.POST("/initialize", transactionHandler.Initialize)
		transaction.GET("/verify/:reference", transactionHandler.Verify)
		transaction.GET("", transactionHandler.List)
		transaction.GET("/:id", transactionHandler.Fetch)
		transaction.POST("/:id/refund", transactionHandler.Refund)
	}

	// Charge namespace —> /api/v1/charge
	// Handles direct charges across all three channels.
	// All routes require a valid merchant secret key.

	chargeHandler := handler.NewChargeHandler(db)
	charge := v1.Group("/charge")
	charge.Use(middleware.RequireSecretKey(db))
	{
		charge.POST("/card", chargeHandler.ChargeCard)
		charge.POST("/mobile_money", chargeHandler.ChargeMobileMoney)
		charge.POST("/bank", chargeHandler.ChargeBank)
		charge.GET("/:reference", chargeHandler.FetchCharge)
	}

	// Customer namespace —> /api/v1/customer
	// Standard CRUD for customer management.
	// All routes require a valid merchant secret key.

	customerHandler := handler.NewCustomerHandler(db)
	customer := v1.Group("/customer")
	customer.Use(middleware.RequireSecretKey(db))
	{
		customer.POST("", customerHandler.Create)
		customer.GET("", customerHandler.List)
		customer.GET("/:code", customerHandler.Fetch)
		customer.PUT("/:code", customerHandler.Update)
		customer.GET("/:code/transactions", customerHandler.Transactions)
	}

	// Control namespace —> /api/v1/control
	// Payfake-specific power layer. No Paystack equivalent.
	// Requires JWT (dashboard session) not a secret key, these routes
	// are for the developer using the dashboard, not their application.

	controlHandler := handler.NewControlHandler(db)
	control := v1.Group("/control")
	control.Use(middleware.RequireJWT())
	{
		// Scenario management
		control.GET("/scenario", controlHandler.GetScenario)
		control.PUT("/scenario", controlHandler.UpdateScenario)
		control.POST("/scenario/reset", controlHandler.ResetScenario)

		// Webhook introspection
		control.GET("/webhooks", controlHandler.ListWebhooks)
		control.POST("/webhooks/:id/retry", controlHandler.RetryWebhook)
		control.GET("/webhooks/:id/attempts", controlHandler.GetWebhookAttempts)

		// Transaction forcing, the core testing tool
		control.POST("/transactions/:ref/force", controlHandler.ForceTransaction)

		// Request/response introspection logs
		control.GET("/logs", controlHandler.GetLogs)
		control.DELETE("/logs", controlHandler.ClearLogs)
	}

	return r
}
