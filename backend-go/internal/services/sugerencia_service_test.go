package services

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

func TestNuevaDeteccionDetectada(t *testing.T) {
	clase, conf := "basura", 0.9123
	d := nuevaDeteccion(7, &clients.Deteccion{
		Resultado: clients.ResultadoDetectado, ClaseDetectada: &clase, Confianza: &conf,
		Modelo: "urbia-yolo26s", VersionModelo: "1.0.0", UmbralConfianza: 0.25, Milisegundos: 180.5,
	}, nil)

	if d.IDReporte != 7 || d.Resultado != dominio.ResultadoDetectado {
		t.Fatalf("detección mal armada: %+v", d)
	}
	if *d.ClaseDetectada != "basura" || *d.Confianza != 0.9123 {
		t.Errorf("clase o confianza incorrectas: %v %v", *d.ClaseDetectada, *d.Confianza)
	}
	if *d.Modelo != "urbia-yolo26s" || *d.VersionModelo != "1.0.0" || *d.UmbralConfianza != 0.25 {
		t.Error("no se guardó el modelo, la versión o el umbral")
	}
	if d.CodigoError != nil {
		t.Error("una detección correcta no lleva código de error")
	}
}

func TestNuevaDeteccionSinDeteccion(t *testing.T) {
	d := nuevaDeteccion(7, &clients.Deteccion{
		Resultado: clients.ResultadoNoDetectado, Modelo: "urbia-yolo26s", VersionModelo: "1.0.0",
	}, nil)
	if d.Resultado != dominio.ResultadoNoDetectado || d.ClaseDetectada != nil || d.Confianza != nil {
		t.Errorf("sin detección: clase y confianza deben ser NULL: %+v", d)
	}
	if d.Modelo == nil || d.VersionModelo == nil {
		t.Error("sin detección también se guarda el modelo que decidió")
	}
}

func TestNuevaDeteccionErrores(t *testing.T) {
	casos := []struct {
		nombre string
		err    error
		codigo string
	}{
		{"servicio caído", &clients.ErrorInferencia{Codigo: clients.CodigoServicioNoDisponible, GRPC: codes.Unavailable}, "SERVICIO_NO_DISPONIBLE"},
		{"tiempo agotado", &clients.ErrorInferencia{Codigo: clients.CodigoTiempoAgotado, GRPC: codes.DeadlineExceeded}, "TIEMPO_AGOTADO"},
		{"imagen inválida", &clients.ErrorInferencia{Codigo: "IMAGEN_INVALIDA", GRPC: codes.InvalidArgument}, "IMAGEN_INVALIDA"},
		{"no configurado", errNoConfigurado, dominio.CodigoNoConfigurado},
		{"error sin código", errors.New("algo raro"), clients.CodigoDesconocido},
	}
	for _, c := range casos {
		d := nuevaDeteccion(7, nil, c.err)
		if d.Resultado != dominio.ResultadoError || d.CodigoError == nil || *d.CodigoError != c.codigo {
			t.Errorf("%s: se esperaba error %s, se obtuvo %+v", c.nombre, c.codigo, d)
		}
		if d.ClaseDetectada != nil || d.Confianza != nil || d.Modelo != nil {
			t.Errorf("%s: con error no hay clase, confianza ni modelo", c.nombre)
		}
	}
}

func TestNuevaDeteccionDetectadoIncompleto(t *testing.T) {
	// Si llegara "detectado" sin clase, se guarda como no_detectado: nunca una sugerencia vacía.
	d := nuevaDeteccion(7, &clients.Deteccion{Resultado: clients.ResultadoDetectado, Modelo: "m", VersionModelo: "1"}, nil)
	if d.Resultado != dominio.ResultadoNoDetectado {
		t.Errorf("se esperaba no_detectado, se obtuvo %s", d.Resultado)
	}
}
