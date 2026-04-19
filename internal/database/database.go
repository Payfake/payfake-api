package database

import (
	"log"

	"github.com/GordenArcher/payfake/internal/config"
	"github.com/GordenArcher/payfake/internal/domain"
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
		gormCfg.Logger = logger.Default.LogMode(logger.Info)
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
	)

	if err != nil {
		return err
	}

	log.Println("[payfake] migrations complete")
	return nil
}
