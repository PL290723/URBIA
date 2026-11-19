package services

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

// InferenciaMonitor revisa el servicio de inferencia en segundo plano.
// El health check solo lee el último resultado, así que nunca espera al servicio Python.
type InferenciaMonitor struct {
	client   *clients.InferenciaClient
	interval time.Duration
	log      *slog.Logger
	estado   atomic.Pointer[models.Dependencia]
}

func NewInferenciaMonitor(client *clients.InferenciaClient, interval time.Duration, log *slog.Logger) *InferenciaMonitor {
	m := &InferenciaMonitor{client: client, interval: interval, log: log}
	inicial := models.Dependencia{Estado: models.DependenciaSinVerificar}
	if !client.Configurado() {
		inicial.Estado = models.DependenciaNoConfigurada
	}
	m.estado.Store(&inicial)
	return m
}

// Iniciar lanza la goroutine de verificación y regresa de inmediato.
func (m *InferenciaMonitor) Iniciar(ctx context.Context) {
	if !m.client.Configurado() {
		m.log.Warn("INFERENCE_URL no configurada: no se monitorea el servicio de inferencia")
		return
	}
	go func() {
		m.verificar(ctx)
		t := time.NewTicker(m.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.verificar(ctx)
			}
		}
	}()
}

func (m *InferenciaMonitor) Estado() models.Dependencia { return *m.estado.Load() }

func (m *InferenciaMonitor) verificar(ctx context.Context) {
	inicio := time.Now()
	err := m.client.Health(ctx)
	if ctx.Err() != nil {
		return // la API se está apagando
	}

	ahora := time.Now().UTC()
	nuevo := models.Dependencia{
		Estado:             models.DependenciaActiva,
		LatenciaMs:         time.Since(inicio).Milliseconds(),
		UltimaVerificacion: &ahora,
	}
	if err != nil {
		nuevo.Estado = models.DependenciaCaida
	}

	// Solo se registra el cambio de estado, no cada verificación.
	anterior := m.estado.Swap(&nuevo)
	if anterior.Estado != nuevo.Estado {
		if err != nil {
			m.log.Error("servicio de inferencia no disponible", "error", err)
		} else {
			m.log.Info("servicio de inferencia disponible", "latencia_ms", nuevo.LatenciaMs)
		}
	}
}
