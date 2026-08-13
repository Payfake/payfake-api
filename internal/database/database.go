package database

import (
	"log"

	"github.com/payfake/payfake-api/internal/config"
	"github.com/payfake/payfake-api/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	DB *gorm.DB
}

func Connect(cfg *config.Config) (*Database, error) {
	gormCfg := &gorm.Config{}

	if cfg.App.Env == "development" {
		// Info-level SQL logging renders bound values, including password
		// hashes, API keys, refresh-token identifiers, and raw payment fields.
		// Keep warnings and errors visible while the structured request logger
		// provides redacted request tracing for normal development diagnostics.
		gormCfg.Logger = logger.Default.LogMode(logger.Warn)
	} else {
		gormCfg.Logger = logger.Default.LogMode(logger.Silent)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), gormCfg)
	if err != nil {
		return nil, err
	}

	log.Println("[payfake] database connected")

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

func migrate(db *gorm.DB) error {
	log.Println("[payfake] running migrations...")

	err := db.AutoMigrate(
		&domain.Merchant{},
		&domain.Customer{},
		&domain.Transaction{},
		&domain.Charge{},
		&domain.WebhookEndpoint{},
		&domain.WebhookEvent{},
		&domain.WebhookAttempt{},
		&domain.ScenarioConfig{},
		&domain.RequestLog{},
		&domain.OTPLog{},
		&domain.RefreshSession{},
	)

	if err != nil {
		return err
	}

	// Access-code expiry was added after transactions already existed. Backfill
	// those rows from their creation time so an upgrade does not make historical
	// checkout records unreadable merely because the new column started as NULL.
	if err := db.Exec(`
		UPDATE transactions
		SET access_code_expires_at = created_at + INTERVAL '1 hour'
		WHERE access_code_expires_at IS NULL
	`).Error; err != nil {
		return err
	}

	// Earlier releases used a read-before-create customer lookup, so concurrent
	// requests could leave multiple active rows for one merchant/email pair.
	// Repoint transactions to the oldest canonical row before soft-deleting the
	// redundant rows; this preserves history while allowing the new uniqueness
	// guarantee to be installed safely on an existing database.
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			WITH ranked_customers AS (
				SELECT id,
					FIRST_VALUE(id) OVER (
						PARTITION BY merchant_id, email
						ORDER BY created_at, id
					) AS canonical_id,
					ROW_NUMBER() OVER (
						PARTITION BY merchant_id, email
						ORDER BY created_at, id
					) AS duplicate_rank
				FROM customers
				WHERE deleted_at IS NULL
			)
			UPDATE transactions
			SET customer_id = ranked_customers.canonical_id
			FROM ranked_customers
			WHERE transactions.customer_id = ranked_customers.id
				AND ranked_customers.duplicate_rank > 1
		`).Error; err != nil {
			return err
		}
		return tx.Exec(`
			WITH ranked_customers AS (
				SELECT id,
					ROW_NUMBER() OVER (
						PARTITION BY merchant_id, email
						ORDER BY created_at, id
					) AS duplicate_rank
				FROM customers
				WHERE deleted_at IS NULL
			)
			UPDATE customers
			SET deleted_at = NOW(), updated_at = NOW()
			FROM ranked_customers
			WHERE customers.id = ranked_customers.id
				AND ranked_customers.duplicate_rank > 1
		`).Error
	}); err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_merchant_email
		ON customers (merchant_id, email)
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return err
	}

	// Historical terminal attempts remain valid records, but only one pending
	// charge may exist for a transaction. A partial unique index enforces the
	// concurrency rule. Reconcile legacy duplicate pending attempts first so an
	// upgrade from a release with the race does not fail while creating it.
	if err := db.Exec(`
		WITH ranked_charges AS (
			SELECT id,
				ROW_NUMBER() OVER (
					PARTITION BY transaction_id
					ORDER BY created_at DESC, id DESC
				) AS duplicate_rank
			FROM charges
			WHERE status = 'pending' AND deleted_at IS NULL
		)
		UPDATE charges
		SET status = 'abandoned',
			flow_status = 'abandoned',
			deleted_at = NOW(),
			updated_at = NOW()
		FROM ranked_charges
		WHERE charges.id = ranked_charges.id
			AND ranked_charges.duplicate_rank > 1
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_charges_one_pending_per_transaction
		ON charges (transaction_id)
		WHERE status = 'pending' AND deleted_at IS NULL
	`).Error; err != nil {
		return err
	}

	migrator := db.Migrator()
	if migrator.HasIndex(&domain.Transaction{}, "idx_transactions_reference") {
		if err := migrator.DropIndex(&domain.Transaction{}, "idx_transactions_reference"); err != nil {
			return err
		}
	}
	if !migrator.HasIndex(&domain.Transaction{}, "idx_transactions_merchant_reference") {
		if err := migrator.CreateIndex(&domain.Transaction{}, "idx_transactions_merchant_reference"); err != nil {
			return err
		}
	}

	log.Println("[payfake] migrations complete")
	return nil
}
