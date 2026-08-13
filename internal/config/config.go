package config

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/GordenArcher/godenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

type AppConfig struct {
	Name        string
	Env         string
	Port        string
	FrontendURL string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	DSN      string
}

type JWTConfig struct {
	Secret              string
	AccessExpiryMinutes string
	RefreshExpiryDays   string
}

func Load() (*Config, error) {
	_ = godenv.Load()

	cfg := &Config{
		App: AppConfig{
			Name:        godenv.Get("APP_NAME", "payfake"),
			Env:         godenv.Get("APP_ENV", "development"),
			Port:        godenv.Get("APP_PORT", "8080"),
			FrontendURL: godenv.Get("FRONTEND_URL", "http://localhost:3000"),
		},
		Database: DatabaseConfig{
			Host:     godenv.Get("DB_HOST", "localhost"),
			Port:     godenv.Get("DB_PORT", "5432"),
			User:     godenv.Get("DB_USER", "postgres"),
			Password: godenv.Get("DB_PASSWORD", ""),
			Name:     godenv.Get("DB_NAME", "payfake"),
			SSLMode:  godenv.Get("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:              godenv.Get("JWT_SECRET", ""),
			AccessExpiryMinutes: godenv.Get("JWT_ACCESS_EXPIRY_MINUTES", "15"),
			RefreshExpiryDays:   godenv.Get("JWT_REFRESH_EXPIRY_DAYS", "7"),
		},
	}

	cfg.Database.DSN = fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Africa/Accra",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.App.Env == "production" && len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}
	accessMinutes, err := strconv.Atoi(c.JWT.AccessExpiryMinutes)
	if err != nil || accessMinutes <= 0 {
		return fmt.Errorf("JWT_ACCESS_EXPIRY_MINUTES must be a positive integer")
	}
	refreshDays, err := strconv.Atoi(c.JWT.RefreshExpiryDays)
	if err != nil || refreshDays <= 0 {
		return fmt.Errorf("JWT_REFRESH_EXPIRY_DAYS must be a positive integer")
	}
	frontend, err := url.ParseRequestURI(c.App.FrontendURL)
	if err != nil || (frontend.Scheme != "http" && frontend.Scheme != "https") || frontend.Hostname() == "" {
		return fmt.Errorf("FRONTEND_URL must be an absolute http or https URL")
	}
	// if c.Database.Password == "" {
	// 	return fmt.Errorf("DB_PASSWORD is required")
	// }
	return nil
}
