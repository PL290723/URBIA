package config

import "testing"

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
	cfg, err := Load()
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cfg.Port != 9090 {
		t.Fatalf("PORT = %d, se esperaba 9090", cfg.Port)
	}
}
