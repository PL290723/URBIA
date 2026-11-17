//go:build integracion

package integracion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/app"
	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/db"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/services"
	"github.com/PL290723/URBIA/backend-go/internal/storage"
)

// Prueba de regresión de punta a punta (HU 09.4).
//
// Recorre la vida completa de UN reporte con PostgreSQL, STORAGE_DIR y el
// servicio de IA reales, y en cada paso revisa lo que quedó en la base y en
// disco, no solo la respuesta HTTP:
//
//  1. Registrar un reporte con imagen y coordenadas
//  2. Persistencia de reporte, archivo y metadatos
//  3. Se guarda la detección original de la IA
//  4. El ciudadano corrige la categoría y la detección original NO cambia
//  5. El administrador cambia el estado y queda el historial (inmutable)
//  6. Listado, detalle y estadísticas reflejan todo lo anterior
//  7. "Reinicio": una API nueva (pool y almacenamiento nuevos) ve los mismos datos y archivos
//
// Uso: go test -tags integracion ./internal/integracion/ -run Regresion -v -count=1
func TestRegresion_RecorridoCompletoDeUnReporte(t *testing.T) {
	cliente, info := requierePython(t)
	api := nuevaAPI(t, opcionesAPI{detector: cliente})
	ctx := context.Background()

	ciudadano := registrarUsuario(t, api, "Ciudadano 09.4")
	otro := registrarUsuario(t, api, "Vecino 09.4")
	admin := registrarAdministrador(t, api)
	base := estadisticasDeHoy(t, api, admin.token)

	// ---------------------------------------------------------------- 1
	lat := 20.50 + rand.Float64()*0.05 // punto aleatorio en Celaya: no choca con otras corridas
	lon := -100.85 + rand.Float64()*0.05
	foto := fotoParaRegresion(t)
	descripcion := fmt.Sprintf("%s Bache frente a la escuela %d", marcaRegresion, time.Now().UnixNano())

	r := crearReporteCon(t, api, ciudadano.token, foto, map[string]string{
		"descripcion": descripcion, "latitud": formatoCoord(lat), "longitud": formatoCoord(lon),
		"origen_ubicacion": "gps", "precision_m": "6.5",
	})
	r.exigir(t, http.StatusCreated)
	idReporte := r.entero(t, "data.id_reporte")
	idArchivo := r.entero(t, "data.foto.id_archivo")
	t.Logf("reporte %d, archivo %d, sugerencia %s", idReporte, idArchivo, r.texto("data.sugerencia.estado"))

	t.Run("1 registrar reporte con imagen y coordenadas", func(t *testing.T) {
		if r.texto("data.estado") != "pendiente" {
			t.Errorf("estado inicial = %q", r.texto("data.estado"))
		}
		if r.header.Get("Location") != fmt.Sprintf("/api/v1/reportes/%d", idReporte) {
			t.Errorf("Location = %q", r.header.Get("Location"))
		}
		if r.texto("data.ubicacion.origen") != "gps" {
			t.Errorf("origen de la ubicación = %q", r.texto("data.ubicacion.origen"))
		}
	})

	// ---------------------------------------------------------------- 2
	var rutaFoto, shaBD string
	t.Run("2 persistencia de reporte, archivo y metadatos", func(t *testing.T) {
		var (
			idUsuario            int64
			desc, estado, origen string
			latBD, lonBD         float64
			precision            *float64
			idCategoria          *int16
		)
		err := pool.QueryRow(ctx, `
			SELECT id_usuario, descripcion, ST_Y(ubicacion), ST_X(ubicacion), estado::text,
			       origen_ubicacion, precision_m::float8, id_categoria
			FROM reporte WHERE id_reporte = $1`, idReporte).
			Scan(&idUsuario, &desc, &latBD, &lonBD, &estado, &origen, &precision, &idCategoria)
		if err != nil {
			t.Fatalf("leer reporte: %v", err)
		}
		if idUsuario != ciudadano.id || desc != descripcion || estado != "pendiente" || origen != "gps" {
			t.Errorf("reporte mal guardado: usuario=%d desc=%q estado=%s origen=%s", idUsuario, desc, estado, origen)
		}
		if diferencia(latBD, lat) > 1e-6 || diferencia(lonBD, lon) > 1e-6 {
			t.Errorf("coordenadas guardadas (%f, %f), enviadas (%f, %f)", latBD, lonBD, lat, lon)
		}
		if precision == nil || *precision != 6.5 {
			t.Errorf("precision_m = %v, se esperaba 6.5", precision)
		}
		if idCategoria != nil {
			t.Errorf("sin elegir categoría, id_categoria debe ser NULL: %d", *idCategoria)
		}

		var idRepArchivo, tamano int64
		var variante, mime string
		err = pool.QueryRow(ctx, `
			SELECT id_reporte, variante, mime, tamano, ruta, sha256
			FROM archivo WHERE id_archivo = $1`, idArchivo).
			Scan(&idRepArchivo, &variante, &mime, &tamano, &rutaFoto, &shaBD)
		if err != nil {
			t.Fatalf("leer archivo: %v", err)
		}
		if idRepArchivo != idReporte || variante != "original" || mime != "image/jpeg" {
			t.Errorf("metadatos: reporte=%d variante=%s mime=%s", idRepArchivo, variante, mime)
		}
		enDisco, err := os.ReadFile(filepath.Join(almacen.Dir(), filepath.FromSlash(rutaFoto)))
		if err != nil {
			t.Fatalf("la foto %s no está en STORAGE_DIR: %v", rutaFoto, err)
		}
		if int64(len(enDisco)) != tamano || hashHex(enDisco) != shaBD {
			t.Errorf("archivo.tamano/sha256 no coinciden con el archivo en disco (%d bytes)", len(enDisco))
		}

		// La foto que entrega la API es exactamente la guardada
		descarga := get(t, api, fmt.Sprintf("/api/v1/archivos/%d", idArchivo), ciudadano.token)
		descarga.exigir(t, http.StatusOK)
		if hashHex([]byte(descarga.crudo)) != shaBD {
			t.Error("GET /archivos devuelve una foto distinta a la registrada")
		}

		var eventos int
		var anterior *string
		var nuevo string
		err = pool.QueryRow(ctx, `
			SELECT count(*) OVER (), estado_anterior::text, estado_nuevo::text
			FROM historial_estado WHERE id_reporte = $1 ORDER BY id_historial_estado LIMIT 1`, idReporte).
			Scan(&eventos, &anterior, &nuevo)
		if err != nil {
			t.Fatalf("leer historial: %v", err)
		}
		if eventos != 1 || anterior != nil || nuevo != "pendiente" {
			t.Errorf("historial inicial: %d eventos, %v → %s", eventos, anterior, nuevo)
		}
	})

	// ---------------------------------------------------------------- 3
	original := deteccionesDe(t, idReporte)
	t.Run("3 se guarda la detección original", func(t *testing.T) {
		if len(original) != 1 {
			t.Fatalf("se esperaba 1 detección, hay %d", len(original))
		}
		d := original[0]
		if d.resultado != "detectado" && d.resultado != "no_detectado" {
			t.Errorf("resultado = %q (codigo_error %s)", d.resultado, texto(d.codigoError))
		}
		if texto(d.modelo) != info.Modelo || texto(d.version) != info.VersionModelo {
			t.Errorf("modelo %s/%s, Python reporta %s/%s", texto(d.modelo), texto(d.version), info.Modelo, info.VersionModelo)
		}
		if id := r.entero(t, "data.sugerencia.id_deteccion"); id != d.id {
			t.Errorf("la respuesta apunta a la detección %d, la BD tiene %d", id, d.id)
		}
	})

	// ---------------------------------------------------------------- 4
	sugerida := ""
	if original[0].clase != nil {
		sugerida = *original[0].clase
	}
	elegida := "bache"
	if sugerida == "bache" {
		elegida = "basura"
	}
	idElegida := idCategoria(t, elegida)
	origenEsperado := "manual"
	if sugerida != "" {
		origenEsperado = "corregida"
	}

	t.Run("4 corregir categoría sin perder la detección original", func(t *testing.T) {
		c := hacerJSON(t, api, http.MethodPut, fmt.Sprintf("/api/v1/reportes/%d/categoria", idReporte),
			ciudadano.token, fmt.Sprintf(`{"id_categoria": %d}`, idElegida))
		c.exigir(t, http.StatusOK)
		if c.texto("data.origen") != origenEsperado || c.entero(t, "data.categoria.id_categoria") != int64(idElegida) {
			t.Errorf("clasificación inesperada (sugerida %q, elegida %s): %s", sugerida, elegida, c.crudo)
		}

		var catReporte int16
		if err := pool.QueryRow(ctx, `SELECT id_categoria FROM reporte WHERE id_reporte = $1`, idReporte).
			Scan(&catReporte); err != nil || catReporte != idElegida {
			t.Errorf("reporte.id_categoria = %d (err %v), se esperaba %d", catReporte, err, idElegida)
		}

		var catClasif int16
		var idDetClasif *int64
		if err := pool.QueryRow(ctx, `
			SELECT id_categoria, id_deteccion FROM clasificacion
			WHERE id_reporte = $1 ORDER BY id_clasificacion DESC LIMIT 1`, idReporte).
			Scan(&catClasif, &idDetClasif); err != nil {
			t.Fatalf("leer clasificacion: %v", err)
		}
		if catClasif != idElegida || idDetClasif == nil || *idDetClasif != original[0].id {
			t.Errorf("clasificacion: categoría %d, detección %v", catClasif, idDetClasif)
		}

		// LO IMPORTANTE: la detección de la IA quedó exactamente igual
		if despues := deteccionesDe(t, idReporte); !slices.EqualFunc(despues, original, deteccionGuardada.igual) {
			t.Errorf("la detección original cambió al corregir la categoría:\nantes   %+v\ndespués %+v", original, despues)
		}
		det := get(t, api, fmt.Sprintf("/api/v1/reportes/%d", idReporte), admin.token)
		det.exigir(t, http.StatusOK)
		if det.texto("data.administrativo.deteccion.resultado") != original[0].resultado ||
			det.texto("data.administrativo.deteccion.clase_detectada") != sugerida {
			t.Errorf("el detalle administrativo ya no muestra la detección original: %s", det.crudo)
		}
	})

	// ---------------------------------------------------------------- 5
	t.Run("5 cambiar estado como administrador y verificar historial", func(t *testing.T) {
		ruta := fmt.Sprintf("/api/v1/reportes/%d/estado", idReporte)

		hacerJSON(t, api, http.MethodPatch, ruta, ciudadano.token, `{"estado":"en_revision"}`).
			exigir(t, http.StatusForbidden)

		anterior := "pendiente"
		for _, nuevo := range []string{"en_revision", "en_proceso", "resuelto"} {
			c := hacerJSON(t, api, http.MethodPatch, ruta, admin.token, fmt.Sprintf(`{"estado":%q}`, nuevo))
			c.exigir(t, http.StatusOK)
			if c.texto("data.estado_anterior") != anterior || c.texto("data.estado_nuevo") != nuevo {
				t.Errorf("%s → %s: %s", anterior, nuevo, c.crudo)
			}
			anterior = nuevo
		}
		hacerJSON(t, api, http.MethodPatch, ruta, admin.token, `{"estado":"en_proceso"}`).
			exigir(t, http.StatusConflict) // resuelto es final

		h := get(t, api, fmt.Sprintf("/api/v1/reportes/%d/historial", idReporte), admin.token)
		h.exigir(t, http.StatusOK)
		if estados := estadosDelHistorial(t, h); !slices.Equal(estados,
			[]string{"pendiente", "en_revision", "en_proceso", "resuelto"}) {
			t.Errorf("secuencia del historial = %v", estados)
		}
		if h.valor("data.solo_lectura") != true {
			t.Error("el historial debe marcarse como solo lectura")
		}

		var cambiosDelAdmin int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM historial_estado
			WHERE id_reporte = $1 AND estado_anterior IS NOT NULL AND id_usuario = $2`,
			idReporte, admin.id).Scan(&cambiosDelAdmin); err != nil || cambiosDelAdmin != 3 {
			t.Errorf("cambios registrados a nombre del administrador = %d (err %v)", cambiosDelAdmin, err)
		}

		// Ni la propia API puede reescribir el historial (trigger + REVOKE, HU 07.3)
		if _, err := pool.Exec(ctx, `UPDATE historial_estado SET estado_nuevo = 'pendiente'
			WHERE id_reporte = $1`, idReporte); err == nil {
			t.Error("se pudo modificar historial_estado: debería ser de solo inserción")
		}
	})

	// ---------------------------------------------------------------- 6
	t.Run("6 consultar listado, detalle y estadísticas", func(t *testing.T) {
		mios := get(t, api, "/api/v1/reportes?mios=true&por_pagina=100", ciudadano.token)
		mios.exigir(t, http.StatusOK)
		el := buscarEnListado(t, mios, idReporte)
		if el == nil || el["estado"] != "resuelto" || el["es_propio"] != true {
			t.Errorf("el ciudadano no ve su reporte resuelto en 'mis reportes': %v", el)
		}

		filtrado := get(t, api, fmt.Sprintf("/api/v1/reportes?estado=resuelto&id_categoria=%d&por_pagina=100", idElegida),
			admin.token)
		filtrado.exigir(t, http.StatusOK)
		if buscarEnListado(t, filtrado, idReporte) == nil {
			t.Error("el filtro estado=resuelto + categoría no encuentra el reporte")
		}

		autor := get(t, api, fmt.Sprintf("/api/v1/reportes/%d", idReporte), ciudadano.token)
		autor.exigir(t, http.StatusOK)
		if autor.texto("data.estado") != "resuelto" || autor.entero(t, "data.categoria.id_categoria") != int64(idElegida) ||
			autor.valor("data.ubicacion.aproximada") != false || autor.valor("data.foto") == nil {
			t.Errorf("detalle del autor inesperado: %s", autor.crudo)
		}

		ajeno := get(t, api, fmt.Sprintf("/api/v1/reportes/%d", idReporte), otro.token)
		ajeno.exigir(t, http.StatusOK)
		if ajeno.valor("data.ubicacion.aproximada") != true || ajeno.valor("data.foto") != nil {
			t.Errorf("un vecino no debe ver la ubicación exacta ni la foto: %s", ajeno.crudo)
		}

		ahora := estadisticasDeHoy(t, api, admin.token)
		if ahora.total != base.total+1 || ahora.resueltos != base.resueltos+1 ||
			ahora.porCategoria[elegida] != base.porCategoria[elegida]+1 {
			t.Errorf("estadísticas: antes %+v, después %+v", base, ahora)
		}
	})

	// ---------------------------------------------------------------- 7
	t.Run("7 tras reiniciar la API los datos y archivos persisten", func(t *testing.T) {
		reiniciada := apiReiniciada(t, cliente)

		det := get(t, reiniciada, fmt.Sprintf("/api/v1/reportes/%d", idReporte), admin.token) // mismo token
		det.exigir(t, http.StatusOK)
		if det.texto("data.estado") != "resuelto" || det.entero(t, "data.categoria.id_categoria") != int64(idElegida) ||
			det.texto("data.administrativo.deteccion.resultado") != original[0].resultado {
			t.Errorf("después del reinicio el reporte cambió: %s", det.crudo)
		}
		h := get(t, reiniciada, fmt.Sprintf("/api/v1/reportes/%d/historial", idReporte), ciudadano.token)
		h.exigir(t, http.StatusOK)
		if n := len(estadosDelHistorial(t, h)); n != 4 {
			t.Errorf("historial tras el reinicio: %d eventos", n)
		}
		descargada := get(t, reiniciada, fmt.Sprintf("/api/v1/archivos/%d", idArchivo), ciudadano.token)
		descargada.exigir(t, http.StatusOK)
		if hashHex([]byte(descargada.crudo)) != shaBD {
			t.Error("tras el reinicio la foto no es la misma")
		}
		// La API reiniciada sigue aceptando reportes nuevos con IA
		nuevo := crearReporteCon(t, reiniciada, ciudadano.token, foto, map[string]string{
			"descripcion": marcaRegresion + " Reporte creado después del reinicio", "latitud": formatoCoord(lat + 0.001),
			"longitud": formatoCoord(lon), "origen_ubicacion": "manual",
		})
		nuevo.exigir(t, http.StatusCreated)
		if nuevo.texto("data.sugerencia.estado") == "no_disponible" {
			t.Error("tras el reinicio la IA no respondió")
		}
	})
}

// =============================================================== utilidades

const marcaRegresion = "[IT 09.4]"

type usuarioPrueba struct {
	id    int64
	token string
}

func registrarUsuario(t *testing.T, h http.Handler, nombre string) usuarioPrueba {
	t.Helper()
	correo := fmt.Sprintf("it094-%d-%d@urbia.test", time.Now().UnixNano(), contIT.Add(1))
	r := hacerJSON(t, h, http.MethodPost, "/api/v1/auth/registro", "",
		fmt.Sprintf(`{"nombre":%q,"correo":%q,"contrasena":"Regresion094"}`, nombre, correo))
	r.exigir(t, http.StatusCreated)
	return usuarioPrueba{id: r.entero(t, "data.usuario.id_usuario"), token: r.texto("data.token")}
}

// registrarAdministrador crea una cuenta y la promueve en la BD, como se hace en
// producción (los administradores no se crean por la API).
func registrarAdministrador(t *testing.T, h http.Handler) usuarioPrueba {
	t.Helper()
	u := registrarUsuario(t, h, "Administrador 09.4")
	if _, err := pool.Exec(context.Background(),
		`UPDATE usuario SET rol = 'administrador', fecha_actualizacion = now() WHERE id_usuario = $1`, u.id); err != nil {
		t.Fatalf("promover a administrador: %v", err)
	}
	tok, _, err := tokens.Generar(u.id, dominio.RolAdministrador)
	if err != nil {
		t.Fatal(err)
	}
	u.token = tok
	return u
}

func hacerJSON(t *testing.T, h http.Handler, metodo, ruta, token, cuerpo string) respuesta {
	t.Helper()
	req := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return hacer(t, h, req)
}

func crearReporteCon(t *testing.T, h http.Handler, token string, foto []byte, campos map[string]string) respuesta {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range campos {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	parte, err := mw.CreateFormFile("foto", "regresion.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = parte.Write(foto)
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reportes", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Request-ID", idSolicitud("regresion"))
	return hacer(t, h, req)
}

// fotoParaRegresion usa la foto real de URBIA_IT_FOTO_BACHE si existe (la IA
// sugerirá y el paso 4 será una corrección); si no, una sintética.
func fotoParaRegresion(t *testing.T) []byte {
	t.Helper()
	if ruta := os.Getenv("URBIA_IT_FOTO_BACHE"); ruta != "" {
		foto, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatalf("URBIA_IT_FOTO_BACHE: %v", err)
		}
		return foto
	}
	return fotoSintetica(t)
}

func formatoCoord(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }

func diferencia(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func hashHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func idCategoria(t *testing.T, nombre string) int16 {
	t.Helper()
	var id int16
	if err := pool.QueryRow(context.Background(),
		`SELECT id_categoria FROM categoria WHERE nombre = $1`, nombre).Scan(&id); err != nil {
		t.Fatalf("categoría %q: %v", nombre, err)
	}
	return id
}

// deteccionGuardada es la fila completa de deteccion; se compara antes y después.
type deteccionGuardada struct {
	id          int64
	resultado   string
	clase       *string
	confianza   string // texto para comparar sin redondeos
	modelo      *string
	version     *string
	codigoError *string
	fecha       time.Time
}

func (a deteccionGuardada) igual(b deteccionGuardada) bool {
	return a.id == b.id && a.resultado == b.resultado && texto(a.clase) == texto(b.clase) &&
		a.confianza == b.confianza && texto(a.modelo) == texto(b.modelo) &&
		texto(a.version) == texto(b.version) && texto(a.codigoError) == texto(b.codigoError) &&
		a.fecha.Equal(b.fecha)
}

func deteccionesDe(t *testing.T, idReporte int64) []deteccionGuardada {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id_deteccion, resultado, clase_detectada, coalesce(confianza::text, ''),
		       modelo, version_modelo, codigo_error, fecha
		FROM deteccion WHERE id_reporte = $1 ORDER BY id_deteccion`, idReporte)
	if err != nil {
		t.Fatalf("leer detecciones: %v", err)
	}
	defer rows.Close()
	var lista []deteccionGuardada
	for rows.Next() {
		var d deteccionGuardada
		if err := rows.Scan(&d.id, &d.resultado, &d.clase, &d.confianza, &d.modelo, &d.version,
			&d.codigoError, &d.fecha); err != nil {
			t.Fatal(err)
		}
		lista = append(lista, d)
	}
	return lista
}

func estadosDelHistorial(t *testing.T, h respuesta) []string {
	t.Helper()
	eventos, _ := h.valor("data.eventos").([]any)
	var estados []string
	for _, e := range eventos {
		estados = append(estados, e.(map[string]any)["estado_nuevo"].(string))
	}
	return estados
}

func buscarEnListado(t *testing.T, r respuesta, id int64) map[string]any {
	t.Helper()
	elementos, _ := r.valor("data.elementos").([]any)
	for _, e := range elementos {
		m := e.(map[string]any)
		if int64(m["id_reporte"].(float64)) == id {
			return m
		}
	}
	return nil
}

type resumenEstadisticas struct {
	total        int64
	resueltos    int64
	porCategoria map[string]int64
}

// estadisticasDeHoy pide las estadísticas del día (hora de Celaya) como administrador.
func estadisticasDeHoy(t *testing.T, h http.Handler, token string) resumenEstadisticas {
	t.Helper()
	zona, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		zona = time.FixedZone("CST", -6*3600)
	}
	hoy := time.Now().In(zona).Format("2006-01-02")
	r := get(t, h, fmt.Sprintf("/api/v1/estadisticas?desde=%s&hasta=%s", hoy, hoy), token)
	r.exigir(t, http.StatusOK)

	res := resumenEstadisticas{total: r.entero(t, "data.total"), porCategoria: map[string]int64{}}
	porEstado, _ := r.valor("data.por_estado").([]any)
	for _, e := range porEstado {
		m := e.(map[string]any)
		if m["estado"] == "resuelto" {
			res.resueltos = int64(m["total"].(float64))
		}
	}
	porCategoria, _ := r.valor("data.por_categoria").([]any)
	for _, c := range porCategoria {
		m := c.(map[string]any)
		res.porCategoria[m["nombre"].(string)] = int64(m["total"].(float64))
	}
	return res
}

// apiReiniciada arma una API NUEVA, con su propio pool de PostgreSQL y su propio
// almacenamiento sobre la misma STORAGE_DIR: lo mismo que pasa al reiniciar el
// proceso (o el contenedor) de la API. No comparte nada en memoria con la anterior.
func apiReiniciada(t *testing.T, detector *clients.ClienteDeteccion) http.Handler {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	poolNuevo, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("reconectar a PostgreSQL: %v", err)
	}
	t.Cleanup(poolNuevo.Close)
	almacenNuevo, err := storage.NewLocal(cfg.StorageDir, dominio.TamanoMaximoArchivo)
	if err != nil {
		t.Fatalf("reabrir STORAGE_DIR: %v", err)
	}
	t.Cleanup(func() { _ = almacenNuevo.Close() })

	r, err := app.NuevoRouter(app.Dependencias{
		Log:        logAPI,
		Pool:       poolNuevo,
		Almacen:    almacenNuevo,
		Tokens:     tokens,
		Inferencia: services.NewInferenciaMonitor(clients.NewInferenciaClient("", time.Second), time.Minute, logAPI),
		Detector:   detector,
		Cercania: dominio.ParametrosCercania{
			RadioM: cfg.DuplicadosRadioM, VentanaDias: cfg.DuplicadosVentanaDias, Limite: dominio.LimiteCercanosDefecto,
		},
		Prioridad: cfg.Prioridad,
		Version:   "integracion-reiniciada",
		Inicio:    time.Now(),
	})
	if err != nil {
		t.Fatalf("armar la API reiniciada: %v", err)
	}
	return r
}
