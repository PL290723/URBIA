package models

import "time"

type EstadoDependencia string

const (
	DependenciaActiva        EstadoDependencia = "activo"
	DependenciaCaida         EstadoDependencia = "caido"
	DependenciaNoConfigurada EstadoDependencia = "no_configurado"
	DependenciaSinVerificar  EstadoDependencia = "sin_verificar"
)

const (
	EstadoOK        = "ok"        // todo responde
	EstadoDegradado = "degradado" // la API sirve, pero la inferencia no responde
	EstadoCaido     = "caido"     // sin base de datos no se puede atender
)

// Dependencia solo expone estado y tiempos: nunca URLs, hosts ni mensajes de error.
type Dependencia struct {
	Estado             EstadoDependencia `json:"estado"`
	LatenciaMs         int64             `json:"latencia_ms"`
	UltimaVerificacion *time.Time        `json:"ultima_verificacion,omitempty"`
}

type Dependencias struct {
	Database   Dependencia `json:"database"`
	Inferencia Dependencia `json:"inferencia"`
}

type HealthStatus struct {
	Status         string       `json:"status"`
	Version        string       `json:"version"`
	UptimeSegundos int64        `json:"uptime_segundos"`
	Timestamp      time.Time    `json:"timestamp"`
	Dependencias   Dependencias `json:"dependencias"`
}
