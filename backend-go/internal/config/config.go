package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
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
	// gRPC del servicio de inferencia (HU 05.4). Vacío = no configurado.
	InferenceGRPCAddr    string
	InferenceGRPCTimeout time.Duration
	StorageDir           string
	// Búsqueda de reportes cercanos/duplicados (HU 06.5).
	DuplicadosRadioM      int
	DuplicadosVentanaDias int
	Prioridad             dominio.ConfigPrioridad
	// Seguridad (HU 09.1)
	CORSAllowedOrigins []string // vacío = ningún navegador de otro origen puede leer la API
	LoginMaxIntentos   int
	LoginVentana       time.Duration
}

// marcadorEjemplo es el texto de los valores de .env.example que hay que reemplazar.
const marcadorEjemplo = "CAMBIAR"

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
	// El valor de .env.example mide más de 32 caracteres y es público (está en GitHub):
	// con él cualquiera podría firmar un token de administrador.
	if strings.Contains(strings.ToUpper(cfg.JWTSecret), marcadorEjemplo) {
		errs = append(errs, errors.New("JWT_SECRET sigue con el valor de ejemplo de .env.example: genera uno propio"))
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
	cfg.InferenceGRPCAddr = strings.TrimSpace(os.Getenv("INFERENCE_GRPC_ADDR"))
	if cfg.InferenceGRPCAddr != "" {
		if host, puerto, err := net.SplitHostPort(cfg.InferenceGRPCAddr); err != nil || host == "" || puerto == "" {
			errs = append(errs, errors.New("INFERENCE_GRPC_ADDR inválida (ejemplo: localhost:50051)"))
		}
	}
	cfg.InferenceGRPCTimeout = duracionPositiva("INFERENCE_GRPC_TIMEOUT", "10s", &errs)
	cfg.StorageDir = strings.TrimSpace(getEnv("STORAGE_DIR", "./storage"))
	cfg.DuplicadosRadioM = enteroEnRango("DUPLICADOS_RADIO_M", 50, 1, 1000, &errs)
	cfg.DuplicadosVentanaDias = enteroEnRango("DUPLICADOS_VENTANA_DIAS", 30, 1, 365, &errs)
	cfg.Prioridad = cargarPrioridad(&errs)
	cfg.CORSAllowedOrigins = parseOrigenes(os.Getenv("CORS_ALLOWED_ORIGINS"), &errs)
	cfg.LoginMaxIntentos = enteroEnRango("LOGIN_MAX_INTENTOS", 10, 3, 100, &errs)
	cfg.LoginVentana = duracionPositiva("LOGIN_VENTANA", "5m", &errs)

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

func enteroEnRango(key string, def, minimo, maximo int, errs *[]error) int {
	n, err := strconv.Atoi(getEnv(key, strconv.Itoa(def)))
	if err != nil || n < minimo || n > maximo {
		*errs = append(*errs, fmt.Errorf("%s debe ser un entero entre %d y %d", key, minimo, maximo))
	}
	return n
}

func decimalEnRango(key string, def, minimo, maximo float64, errs *[]error) float64 {
	n, err := strconv.ParseFloat(getEnv(key, strconv.FormatFloat(def, 'f', -1, 64)), 64)
	if err != nil || n < minimo || n > maximo {
		*errs = append(*errs, fmt.Errorf("%s debe ser un número entre %g y %g", key, minimo, maximo))
	}
	return n
}

// parseOrigenes lee CORS_ALLOWED_ORIGINS: orígenes exactos separados por coma
// (esquema://host[:puerto], sin ruta ni "/" final). "*" no se acepta.
func parseOrigenes(valor string, errs *[]error) []string {
	var origenes []string
	for _, o := range strings.Split(valor, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			*errs = append(*errs, errors.New(`CORS_ALLOWED_ORIGINS no admite "*": lista los orígenes exactos del frontend`))
			continue
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			*errs = append(*errs, fmt.Errorf("CORS_ALLOWED_ORIGINS: origen inválido %q (ejemplo: http://192.168.1.50:5173, sin / final)", o))
			continue
		}
		origenes = append(origenes, strings.ToLower(o))
	}
	return origenes
}

// cargarPrioridad lee la fórmula de prioridad (HU 07.1); los pesos deben sumar 1.
func cargarPrioridad(errs *[]error) dominio.ConfigPrioridad {
	p := dominio.ConfigPrioridad{
		Pesos: dominio.PesosPrioridad{
			Gravedad:    decimalEnRango("PRIORIDAD_PESO_GRAVEDAD", dominio.PesoGravedadDefecto, 0, 1, errs),
			Afectacion:  decimalEnRango("PRIORIDAD_PESO_AFECTACION", dominio.PesoAfectacionDefecto, 0, 1, errs),
			Recurrencia: decimalEnRango("PRIORIDAD_PESO_RECURRENCIA", dominio.PesoRecurrenciaDefecto, 0, 1, errs),
			Antiguedad:  decimalEnRango("PRIORIDAD_PESO_ANTIGUEDAD", dominio.PesoAntiguedadDefecto, 0, 1, errs),
		},
		AntiguedadMaxDias: enteroEnRango("PRIORIDAD_ANTIGUEDAD_MAX_DIAS", dominio.AntiguedadMaxDiasDefecto, 1, 365, errs),
		RecurrenciaMaxima: enteroEnRango("PRIORIDAD_RECURRENCIA_MAX", dominio.RecurrenciaMaximaDefecto, 1, 50, errs),
		CoberturaMinima:   decimalEnRango("PRIORIDAD_COBERTURA_MIN", dominio.CoberturaMinimaDefecto, 0.1, 1, errs),
	}
	if err := p.Pesos.Validar(); err != nil {
		*errs = append(*errs, fmt.Errorf("PRIORIDAD_PESO_*: %w", err))
	}
	return p
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
	inferenciaGRPC := "no configurada"
	if c.InferenceGRPCAddr != "" {
		inferenciaGRPC = c.InferenceGRPCAddr
	}
	cors := "ninguno (solo clientes sin navegador)"
	if len(c.CORSAllowedOrigins) > 0 {
		cors = strings.Join(c.CORSAllowedOrigins, ",")
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
		"inferencia_grpc", inferenciaGRPC,
		"inferencia_grpc_timeout", c.InferenceGRPCTimeout.String(),
		"almacenamiento", c.StorageDir,
		"duplicados_radio_m", c.DuplicadosRadioM,
		"duplicados_ventana_dias", c.DuplicadosVentanaDias,
		"prioridad_formula", dominio.VersionFormula,
		"prioridad_pesos", fmt.Sprintf("%.2f/%.2f/%.2f/%.2f", c.Prioridad.Pesos.Gravedad,
			c.Prioridad.Pesos.Afectacion, c.Prioridad.Pesos.Recurrencia, c.Prioridad.Pesos.Antiguedad),
		"cors_origenes", cors,
		"login_max_intentos", c.LoginMaxIntentos,
		"login_ventana", c.LoginVentana.String(),
	}
}
