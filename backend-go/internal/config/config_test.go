package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestLoad_FaltaDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("se esperaba error por DATABASE_URL vacía")
	}
}

func TestLoad_PortInvalido(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
	t.Setenv("PORT", "abc")
	if _, err := Load(); err == nil {
		t.Fatal("se esperaba error por PORT inválido")
	}
}

func TestLoad_Valido(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
	t.Setenv("PORT", "9090")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cfg.Port != 9090 {
		t.Fatalf("PORT = %d, se esperaba 9090", cfg.Port)
	}
}

func TestLoad_JWTSecretCorto(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
	t.Setenv("JWT_SECRET", "corto")
	if _, err := Load(); err == nil {
		t.Fatal("se esperaba error por JWT_SECRET corto")
	}
}

func TestLoad_InferenceURLInvalida(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("INFERENCE_URL", "localhost:8000") // sin http://
	if _, err := Load(); err == nil {
		t.Fatal("se esperaba error por INFERENCE_URL sin esquema")
	}
}

func TestLoad_InferenceGRPC(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("INFERENCE_GRPC_ADDR", "localhost:50051")
	t.Setenv("INFERENCE_GRPC_TIMEOUT", "15s")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cfg.InferenceGRPCAddr != "localhost:50051" || cfg.InferenceGRPCTimeout.String() != "15s" {
		t.Fatalf("configuración gRPC inesperada: %q %v", cfg.InferenceGRPCAddr, cfg.InferenceGRPCTimeout)
	}
}

func TestLoad_InferenceGRPCInvalida(t *testing.T) {
	casos := map[string]string{
		"INFERENCE_GRPC_ADDR":    "http://localhost:50051",
		"INFERENCE_GRPC_TIMEOUT": "0s",
	}
	for variable, valor := range casos {
		t.Run(variable, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
			t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
			t.Setenv(variable, valor)
			if _, err := Load(); err == nil {
				t.Fatalf("se esperaba error por %s=%s", variable, valor)
			}
		})
	}
}

func TestResumenSeguro_NoExponeSecretos(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "postgres://urbia_app:SuperSecreta@localhost:5432/urbia?sslmode=disable",
		JWTSecret:   "jwt-secreto-que-no-debe-salir-en-logs",
	}
	texto := fmt.Sprint(cfg.ResumenSeguro()...)
	for _, s := range []string{"SuperSecreta", cfg.JWTSecret} {
		if strings.Contains(texto, s) {
			t.Errorf("el resumen expone %q", s)
		}
	}

	cfg.DatabaseURL = "host=localhost user=urbia_app password=SuperSecreta dbname=urbia"
	if strings.Contains(fmt.Sprint(cfg.ResumenSeguro()...), "SuperSecreta") {
		t.Error("el resumen expone la contraseña en formato clave=valor")
	}
}

func TestLoad_PrioridadPorDefecto(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cfg.Prioridad.Pesos.Gravedad != 0.40 || cfg.Prioridad.CoberturaMinima != 0.50 {
		t.Fatalf("pesos por defecto inesperados: %+v", cfg.Prioridad)
	}
}

func TestLoad_PrioridadPesosNoSumanUno(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x@localhost/urbia")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("PRIORIDAD_PESO_GRAVEDAD", "0.60") // 0.60 + 0.30 + 0.20 + 0.10 = 1.20
	if _, err := Load(); err == nil {
		t.Fatal("se esperaba error porque los pesos no suman 1")
	}
}
