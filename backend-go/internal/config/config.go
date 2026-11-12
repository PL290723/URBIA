package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv          string
	Port            int
	GinMode         string
	LogLevel        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	JWTSecret       string
	JWTTTL          time.Duration
}

// Load lee las variables de entorno (y .env si existe) y las valida.
// Devuelve todos los errores juntos para corregirlos de una vez.
func Load() (*Config, error) {
	_ = godotenv.Load() // .env es opcional; en Docker las variables vienen del entorno

	cfg := &Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		GinMode:     getEnv("GIN_MODE", "debug"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	var errs []error

	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, errors.New("PORT debe ser un número entre 1 y 65535"))
	}
	cfg.Port = port

	timeout, err := time.ParseDuration(getEnv("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		errs = append(errs, errors.New("SHUTDOWN_TIMEOUT inválido (ejemplo: 10s)"))
	}
	cfg.ShutdownTimeout = timeout

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL es obligatoria"))
	}
	if !oneOf(cfg.AppEnv, "development", "production", "test") {
		errs = append(errs, fmt.Errorf("APP_ENV inválido: %q", cfg.AppEnv))
	}
	if !oneOf(cfg.GinMode, "debug", "release", "test") {
		errs = append(errs, fmt.Errorf("GIN_MODE inválido: %q", cfg.GinMode))
	}
	if !oneOf(cfg.LogLevel, "debug", "info", "warn", "error") {
		errs = append(errs, fmt.Errorf("LOG_LEVEL inválido: %q", cfg.LogLevel))
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if len(cfg.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET es obligatoria y debe tener al menos 32 caracteres"))
	}

	ttl, err := time.ParseDuration(getEnv("JWT_TTL", "2h"))
	if err != nil || ttl <= 0 {
		errs = append(errs, errors.New("JWT_TTL inválido (ejemplo: 2h)"))
	}
	cfg.JWTTTL = ttl

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}

func (c *Config) Addr() string { return fmt.Sprintf(":%d", c.Port) }

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func oneOf(v string, opts ...string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}
