//go:build integracion

package integracion

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/services"
)

// Subtarea 4 — Probar reporte con inferencia exitosa (flujo completo).
//
//	HTTP POST /reportes → Go sanea y guarda la foto → INSERT reporte/archivo/historial
//	→ gRPC Detectar (Python lee la foto) → INSERT deteccion → 201 con la sugerencia
func TestReporte_InferenciaExitosa(t *testing.T) {
	cliente, info := requierePython(t)
	api := nuevaAPI(t, opcionesAPI{detector: cliente})
	token := registrarCiudadano(t, api)

	xid := idSolicitud("exitosa")
	r := crearReporte(t, api, token, fotoSintetica(t), xid)
	r.exigir(t, http.StatusCreated)
	t.Logf("X-Request-ID %s (búscalo en los logs de Go y de Python)", xid)

	idReporte := r.entero(t, "data.id_reporte")
	if got, want := r.header.Get("Location"), fmt.Sprintf("/api/v1/reportes/%d", idReporte); got != want {
		t.Errorf("Location = %q, se esperaba %q", got, want)
	}
	if got := r.header.Get("X-Request-ID"); got != xid {
		t.Errorf("X-Request-ID de respuesta = %q, se esperaba %q", got, xid)
	}

	// La respuesta trae la sugerencia de la IA.
	estado := r.texto("data.sugerencia.estado")
	if estado != "sugerida" && estado != "sin_deteccion" {
		t.Fatalf("sugerencia.estado = %q, se esperaba sugerida o sin_deteccion\n%s", estado, r.crudo)
	}
	if r.texto("data.sugerencia.modelo") != info.Modelo {
		t.Errorf("sugerencia.modelo = %q, Python reporta %q", r.texto("data.sugerencia.modelo"), info.Modelo)
	}
	if r.valor("data.sugerencia.id_deteccion") == nil {
		t.Error("sugerencia.id_deteccion es null: la detección no se guardó")
	}

	// PostgreSQL: la detección quedó guardada tal como la devolvió Python.
	d := ultimaDeteccion(t, idReporte)
	if d.Resultado != "detectado" && d.Resultado != "no_detectado" {
		t.Errorf("deteccion.resultado = %q", d.Resultado)
	}
	if d.CodigoError != nil {
		t.Errorf("deteccion.codigo_error = %q, se esperaba NULL", *d.CodigoError)
	}
	if texto(d.Modelo) != info.Modelo || texto(d.Version) != info.VersionModelo {
		t.Errorf("deteccion.modelo/version = %s/%s, Python reporta %s/%s",
			texto(d.Modelo), texto(d.Version), info.Modelo, info.VersionModelo)
	}
	if d.Ms == nil || *d.Ms <= 0 {
		t.Error("deteccion.milisegundos vacío")
	}

	// Disco: la foto está donde dice archivo.ruta.
	ruta := rutaArchivo(t, r.entero(t, "data.foto.id_archivo"))
	if _, err := os.Stat(filepath.Join(almacen.Dir(), filepath.FromSlash(ruta))); err != nil {
		t.Errorf("la foto %s no está en STORAGE_DIR: %v", ruta, err)
	}

	// El autor ve la misma sugerencia en el detalle (lectura desde PostgreSQL).
	det := get(t, api, fmt.Sprintf("/api/v1/reportes/%d", idReporte), token)
	det.exigir(t, http.StatusOK)
	if det.texto("data.sugerencia.estado") != estado {
		t.Errorf("el detalle muestra sugerencia %q y la creación %q", det.texto("data.sugerencia.estado"), estado)
	}
	if det.texto("data.estado") != "pendiente" {
		t.Errorf("estado del reporte = %q, se esperaba pendiente", det.texto("data.estado"))
	}
	t.Logf("reporte %d: resultado %s, sugerencia %s, %.1f ms en Python", idReporte, d.Resultado, estado, *d.Ms)

	t.Run("foto real de bache", func(t *testing.T) {
		ruta := os.Getenv("URBIA_IT_FOTO_BACHE")
		if ruta == "" {
			t.Skip("URBIA_IT_FOTO_BACHE vacía: se omite la comprobación de la clase")
		}
		foto, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		r := crearReporte(t, api, token, foto, idSolicitud("bache"))
		r.exigir(t, http.StatusCreated)
		if r.texto("data.sugerencia.estado") != "sugerida" || r.texto("data.sugerencia.categoria.nombre") != "bache" {
			t.Fatalf("se esperaba la sugerencia bache:\n%s", r.crudo)
		}
		if c, _ := r.valor("data.sugerencia.confianza").(float64); c < info.UmbralConfianza {
			t.Errorf("confianza %.4f menor que el umbral %.2f", c, info.UmbralConfianza)
		}
	})
}

// Subtarea 5 — Probar reporte cuando el servicio de IA está apagado.
// El reporte SIEMPRE se guarda; solo la sugerencia queda "no_disponible".
func TestReporte_IAApagada(t *testing.T) {
	requiereBase(t)

	casos := []struct {
		nombre   string
		detector func(t *testing.T) services.Detector
		codigo   string
	}{
		{
			nombre:   "INFERENCE_GRPC_ADDR sin configurar",
			detector: func(*testing.T) services.Detector { return nil },
			codigo:   "NO_CONFIGURADO",
		},
		{
			nombre: "servicio Python detenido",
			detector: func(t *testing.T) services.Detector {
				c, err := clients.NuevoClienteDeteccion(puertoCerrado(t), cfg.InferenceGRPCTimeout)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = c.Cerrar() })
				return c
			},
			codigo: clients.CodigoServicioNoDisponible,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			api := nuevaAPI(t, opcionesAPI{detector: c.detector(t)})
			token := registrarCiudadano(t, api)

			inicio := time.Now()
			r := crearReporte(t, api, token, fotoSintetica(t), idSolicitud("apagada"))
			duracion := time.Since(inicio)
			r.exigir(t, http.StatusCreated)

			if got := r.texto("data.sugerencia.estado"); got != "no_disponible" {
				t.Errorf("sugerencia.estado = %q, se esperaba no_disponible", got)
			}
			if r.valor("data.sugerencia.categoria") != nil || r.valor("data.sugerencia.modelo") != nil {
				t.Errorf("sin IA no debe haber categoría ni modelo: %s", r.crudo)
			}
			// Sin servicio, la API no espera el timeout completo: falla rápido.
			if duracion >= cfg.InferenceGRPCTimeout {
				t.Errorf("crear el reporte tardó %v (≥ INFERENCE_GRPC_TIMEOUT %v)", duracion, cfg.InferenceGRPCTimeout)
			}

			idReporte := r.entero(t, "data.id_reporte")
			d := ultimaDeteccion(t, idReporte)
			if d.Resultado != "error" || texto(d.CodigoError) != c.codigo {
				t.Errorf("deteccion = %s/%s, se esperaba error/%s", d.Resultado, texto(d.CodigoError), c.codigo)
			}
			if d.Modelo != nil || d.Clase != nil || d.Confianza != nil {
				t.Error("una detección con error no debe guardar modelo, clase ni confianza")
			}

			// El reporte existe y sigue su ciclo normal.
			det := get(t, api, fmt.Sprintf("/api/v1/reportes/%d", idReporte), token)
			det.exigir(t, http.StatusOK)
			if det.texto("data.estado") != "pendiente" {
				t.Errorf("estado = %q, se esperaba pendiente", det.texto("data.estado"))
			}
			t.Logf("reporte %d guardado en %v con sugerencia no_disponible (%s)", idReporte, duracion.Round(time.Millisecond), c.codigo)
		})
	}

	t.Run("health queda degradado, no caído", func(t *testing.T) {
		monitor := services.NewInferenciaMonitor(
			clients.NewInferenciaClient("http://"+puertoCerrado(t), time.Second), time.Hour, logAPI)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		monitor.Iniciar(ctx)
		esperar(t, 5*time.Second, func() bool { return monitor.Estado().Estado != "sin_verificar" })

		api := nuevaAPI(t, opcionesAPI{monitor: monitor})
		r := get(t, api, "/api/v1/health", "")
		r.exigir(t, http.StatusOK)
		if r.texto("data.status") != "degradado" || r.texto("data.dependencias.inferencia.estado") != "caido" {
			t.Errorf("se esperaba status degradado con inferencia caido:\n%s", r.crudo)
		}
		if r.texto("data.dependencias.database.estado") != "activo" {
			t.Errorf("la base de datos debería seguir activa:\n%s", r.crudo)
		}
	})

	// Control: con el servicio arriba, el mismo flujo sí trae sugerencia.
	t.Run("con Python arriba sí hay sugerencia", func(t *testing.T) {
		cliente, _ := requierePython(t)
		api := nuevaAPI(t, opcionesAPI{detector: cliente})
		r := crearReporte(t, api, registrarCiudadano(t, api), fotoSintetica(t), idSolicitud("recupera"))
		r.exigir(t, http.StatusCreated)
		if slices.Contains([]string{"no_disponible", ""}, r.texto("data.sugerencia.estado")) {
			t.Errorf("con Python arriba la sugerencia no debería ser no_disponible:\n%s", r.crudo)
		}
	})
}
