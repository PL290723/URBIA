package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv            string
	Port              int
	GinMode           string
	LogLevel          string
	DatabaseURL       string
	ShutdownTimeout   time.Duration
	JWTSecret         string
	JWTTTL            time.Duration
	InferenceURL      string
	InferenceInterval time.Duration
	InferenceTimeout  time.Duration
	StorageDir        string
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

	cfg.InferenceURL = strings.TrimRight(os.Getenv("INFERENCE_URL"), "/")
	if cfg.InferenceURL != "" {
		u, err := url.Parse(cfg.InferenceURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, errors.New("INFERENCE_URL inválida (ejemplo: http://localhost:8000)"))
		}
	}
	cfg.InferenceInterval = duracionPositiva("INFERENCE_CHECK_INTERVAL", "15s", &errs)
	cfg.InferenceTimeout = duracionPositiva("INFERENCE_TIMEOUT", "2s", &errs)
	cfg.StorageDir = strings.TrimSpace(getEnv("STORAGE_DIR", "./storage"))

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

func duracionPositiva(key, def string, errs *[]error) time.Duration {
	d, err := time.ParseDuration(getEnv(key, def))
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s inválido (ejemplo: %s)", key, def))
	}
	return d
}

// ResumenSeguro devuelve la configuración apta para logs: sin contraseñas ni secretos.
func (c *Config) ResumenSeguro() []any {
	db := "(formato clave=valor, oculto)"
	if u, err := url.Parse(c.DatabaseURL); err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		u.RawQuery = ""
		db = u.Redacted() // postgres://urbia_app:xxxxx@localhost:5432/urbia
	}
	inferencia := "no configurada"
	if c.InferenceURL != "" {
		inferencia = c.InferenceURL
	}
	return []any{
		"entorno", c.AppEnv,
		"puerto", c.Port,
		"gin_mode", c.GinMode,
		"log_level", c.LogLevel,
		"database", db,
		"jwt_ttl", c.JWTTTL.String(),
		"inferencia", inferencia,
		"inferencia_intervalo", c.InferenceInterval.String(),
		"almacenamiento", c.StorageDir,
	}
}
