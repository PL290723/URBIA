//go:build integracion

package integracion

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/db"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

// Palabras que delatarían detalles internos en una respuesta al cliente.
var detallesInternos = []string{"pgx", "postgres", "SQLSTATE", "pool", "dial tcp", "storage", "originales/"}

func sinDetallesInternos(t *testing.T, r respuesta) {
	t.Helper()
	for _, p := range detallesInternos {
		if strings.Contains(strings.ToLower(r.crudo), strings.ToLower(p)) {
			t.Errorf("la respuesta filtra un detalle interno (%q):\n%s", p, r.crudo)
		}
	}
}

func errorInferencia(t *testing.T, err error) *clients.ErrorInferencia {
	t.Helper()
	var ei *clients.ErrorInferencia
	if !errors.As(err, &ei) {
		t.Fatalf("se esperaba *clients.ErrorInferencia, llegó %T: %v", err, err)
	}
	return ei
}

// Subtarea 6 — Probar errores de base de datos, timeout y archivo inexistente.
func TestErrores_BaseDeDatos(t *testing.T) {
	requiereBase(t)

	t.Run("PostgreSQL apagado al arrancar", func(t *testing.T) {
		u, err := url.Parse(cfg.DatabaseURL)
		if err != nil {
			t.Fatal(err)
		}
		contrasena, _ := u.User.Password()
		u.Host = puertoCerrado(t)

		inicio := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		p, err := db.NewPool(ctx, u.String())
		if err == nil {
			p.Close()
			t.Fatal("NewPool no falló con PostgreSQL apagado")
		}
		if contrasena != "" && strings.Contains(err.Error(), contrasena) {
			t.Error("el error de conexión expone la contraseña de la BD")
		}
		// NewPool hace ping con límite de 5 s: la API no se queda colgada al arrancar.
		if d := time.Since(inicio); d > 7*time.Second {
			t.Errorf("NewPool tardó %v en fallar", d)
		}
		t.Logf("la API no arranca: %v", err)
	})

	t.Run("PostgreSQL cae con la API corriendo", func(t *testing.T) {
		caido, err := db.NewPool(context.Background(), cfg.DatabaseURL)
		if err != nil {
			t.Fatal(err)
		}
		caido.Close() // a partir de aquí toda consulta falla
		api := nuevaAPI(t, opcionesAPI{pool: caido})

		// Health: 503 "caido" (readiness), sin detalles del error.
		h := get(t, api, "/api/v1/health", "")
		h.exigir(t, http.StatusServiceUnavailable)
		if h.texto("data.status") != "caido" || h.texto("error.code") != "SERVICIO_NO_DISPONIBLE" {
			t.Errorf("health inesperado:\n%s", h.crudo)
		}
		sinDetallesInternos(t, h)

		// Login: 500 genérico, NO un 401 engañoso.
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			strings.NewReader(`{"correo":"nadie@urbia.test","contrasena":"Integracion123"}`))
		req.Header.Set("Content-Type", "application/json")
		l := hacer(t, api, req)
		l.exigir(t, http.StatusInternalServerError)
		if l.texto("error.code") != "ERROR_INTERNO" {
			t.Errorf("error.code = %q, se esperaba ERROR_INTERNO", l.texto("error.code"))
		}
		sinDetallesInternos(t, l)

		// Crear reporte: 500 y la foto que alcanzó a escribirse se borra (compensación).
		token, _, err := tokens.Generar(999_999_999, dominio.RolCiudadano)
		if err != nil {
			t.Fatal(err)
		}
		antes := contarOriginales(t)
		r := crearReporte(t, api, token, fotoSintetica(t), idSolicitud("bdcaida"))
		r.exigir(t, http.StatusInternalServerError)
		sinDetallesInternos(t, r)
		if despues := contarOriginales(t); despues != antes {
			t.Errorf("quedaron %d foto(s) huérfana(s) en STORAGE_DIR", despues-antes)
		}
	})
}

func TestErrores_Timeout(t *testing.T) {
	_, _ = requierePython(t)
	ctx := context.Background()

	g, err := almacen.Guardar(ctx, dominio.VarianteOriginal, "jpg", bytes.NewReader(fotoSintetica(t)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = almacen.Eliminar(g.Ruta) })

	// 1 ms es menos de lo que tarda cualquier inferencia.
	lento, err := clients.NuevoClienteDeteccion(cfg.InferenceGRPCAddr, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lento.Cerrar() })

	t.Run("el cliente gRPC devuelve TIEMPO_AGOTADO", func(t *testing.T) {
		_, err := lento.Detectar(ctx, clients.SolicitudDeteccion{
			IDSolicitud: idSolicitud("timeout"), IDReporte: 1, IDArchivo: 1, Ruta: g.Ruta,
		})
		if err == nil {
			t.Fatal("la inferencia terminó en menos de 1 ms: no se probó el timeout")
		}
		ei := errorInferencia(t, err)
		if ei.Codigo != clients.CodigoTiempoAgotado || ei.GRPC != codes.DeadlineExceeded {
			t.Errorf("error = %s/%s, se esperaba TIEMPO_AGOTADO/DeadlineExceeded", ei.Codigo, ei.GRPC)
		}
		if !ei.Reintentable() {
			t.Error("un timeout debe ser reintentable")
		}
	})

	t.Run("el reporte se guarda con sugerencia no_disponible", func(t *testing.T) {
		api := nuevaAPI(t, opcionesAPI{detector: lento})
		r := crearReporte(t, api, registrarCiudadano(t, api), fotoSintetica(t), idSolicitud("timeout"))
		r.exigir(t, http.StatusCreated)
		if r.texto("data.sugerencia.estado") != "no_disponible" {
			t.Errorf("sugerencia.estado = %q, se esperaba no_disponible", r.texto("data.sugerencia.estado"))
		}
		d := ultimaDeteccion(t, r.entero(t, "data.id_reporte"))
		if d.Resultado != "error" || texto(d.CodigoError) != clients.CodigoTiempoAgotado {
			t.Errorf("deteccion = %s/%s, se esperaba error/TIEMPO_AGOTADO", d.Resultado, texto(d.CodigoError))
		}
	})
}

func TestErrores_ArchivoInexistente(t *testing.T) {
	requiereBase(t)

	t.Run("Python: ruta válida pero sin archivo", func(t *testing.T) {
		cliente, _ := requierePython(t)
		_, err := cliente.Detectar(context.Background(), clients.SolicitudDeteccion{
			IDSolicitud: idSolicitud("noexiste"), IDReporte: 1, IDArchivo: 1,
			Ruta: "originales/2000/01/aaaaaaaaaaaaaaaaaaaaaaaaaa.jpg", // patrón de Go, archivo inexistente
		})
		ei := errorInferencia(t, err)
		if ei.Codigo != "IMAGEN_NO_ENCONTRADA" || ei.GRPC != codes.NotFound {
			t.Errorf("error = %s/%s, se esperaba IMAGEN_NO_ENCONTRADA/NotFound", ei.Codigo, ei.GRPC)
		}
		if ei.Reintentable() {
			t.Error("un archivo inexistente no se arregla reintentando")
		}
	})

	t.Run("Python: ruta fuera de STORAGE_DIR", func(t *testing.T) {
		cliente, _ := requierePython(t)
		_, err := cliente.Detectar(context.Background(), clients.SolicitudDeteccion{
			IDSolicitud: idSolicitud("ruta"), IDReporte: 1, IDArchivo: 1, Ruta: "../../backend-go/.env",
		})
		ei := errorInferencia(t, err)
		if ei.Codigo != "RUTA_INVALIDA" || ei.GRPC != codes.InvalidArgument {
			t.Errorf("error = %s/%s, se esperaba RUTA_INVALIDA/InvalidArgument", ei.Codigo, ei.GRPC)
		}
	})

	t.Run("Go: archivo registrado en la BD pero borrado del disco", func(t *testing.T) {
		api := nuevaAPI(t, opcionesAPI{}) // sin IA: aquí solo importa el archivo
		token := registrarCiudadano(t, api)
		r := crearReporte(t, api, token, fotoSintetica(t), idSolicitud("borrado"))
		r.exigir(t, http.StatusCreated)
		idReporte := r.entero(t, "data.id_reporte")
		idArchivo := r.entero(t, "data.foto.id_archivo")

		ruta := rutaArchivo(t, idArchivo)
		if err := os.Remove(filepath.Join(almacen.Dir(), filepath.FromSlash(ruta))); err != nil {
			t.Fatalf("borrar la foto para la prueba: %v", err)
		}

		// Descarga: 404 controlado, sin revelar la ruta interna.
		d := get(t, api, fmt.Sprintf("/api/v1/archivos/%d", idArchivo), token)
		d.exigir(t, http.StatusNotFound)
		if d.texto("error.code") != "NO_ENCONTRADO" {
			t.Errorf("error.code = %q, se esperaba NO_ENCONTRADO", d.texto("error.code"))
		}
		sinDetallesInternos(t, d)

		// El reporte sigue consultable aunque falte la foto.
		get(t, api, fmt.Sprintf("/api/v1/reportes/%d", idReporte), token).exigir(t, http.StatusOK)
	})
}
