package services

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

func monitorDePrueba(url string, timeout time.Duration) *InferenciaMonitor {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewInferenciaMonitor(clients.NewInferenciaClient(url, timeout), time.Minute, log)
}

func TestInferencia_NoConfigurada(t *testing.T) {
	if e := monitorDePrueba("", time.Second).Estado().Estado; e != models.DependenciaNoConfigurada {
		t.Fatalf("estado = %s", e)
	}
}

func TestInferencia_ActivaYCaida(t *testing.T) {
	casos := []struct {
		nombre   string
		codigo   int
		esperado models.EstadoDependencia
	}{
		{"responde 200", http.StatusOK, models.DependenciaActiva},
		{"responde 500", http.StatusInternalServerError, models.DependenciaCaida},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.codigo)
			}))
			defer srv.Close()

			m := monitorDePrueba(srv.URL, time.Second)
			m.verificar(context.Background())
			if e := m.Estado().Estado; e != tc.esperado {
				t.Fatalf("estado = %s, esperado %s", e, tc.esperado)
			}
		})
	}
}

func TestInferencia_LentaNoBloquea(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // simula un servicio colgado
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()

	m := monitorDePrueba(srv.URL, 100*time.Millisecond)
	inicio := time.Now()
	m.verificar(context.Background())

	if d := time.Since(inicio); d > time.Second {
		t.Fatalf("la verificación tardó %v; el timeout no se respetó", d)
	}
	if e := m.Estado().Estado; e != models.DependenciaCaida {
		t.Fatalf("estado = %s, esperado caido", e)
	}
}
