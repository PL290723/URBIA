//go:build integracion

// Package integracion prueba el flujo real Go → PostgreSQL → Python (HU 08.2).
//
// No usa dobles de prueba: abre la base de datos de .env, escribe fotos en
// STORAGE_DIR y llama al servicio de inferencia por gRPC. Por eso va detrás de
// la etiqueta "integracion" y NO corre con un `go test ./...` normal.
//
// Uso (desde backend-go/, con PostgreSQL y el servidor Python levantados):
//
//	go test -tags integracion ./internal/integracion/ -v -count=1
//
// Variables (se leen de backend-go/.env y del entorno):
//
//	DATABASE_URL, JWT_SECRET    obligatorias (las mismas de la API)
//	STORAGE_DIR                 debe ser la MISMA carpeta que STORAGE_DIR de Python
//	INFERENCE_GRPC_ADDR         vacía = se omiten las pruebas que necesitan a Python
//	URBIA_IT_FOTO_BACHE         opcional: foto real de un bache para exigir "sugerida/bache"
//	URBIA_IT_LOG=1              opcional: muestra los logs de la API durante las pruebas
package integracion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/PL290723/URBIA/backend-go/internal/app"
	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/config"
	"github.com/PL290723/URBIA/backend-go/internal/db"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/services"
	"github.com/PL290723/URBIA/backend-go/internal/storage"
)

// raizBackend es backend-go/ visto desde este paquete (go test corre en la carpeta del paquete).
const raizBackend = "../.."

// Marca para reconocer en la BD los reportes que crean estas pruebas.
const marcaIT = "[IT 08.2]"

var (
	cfg     *config.Config
	errCfg  error
	pool    *pgxpool.Pool
	errPool error
	almacen *storage.Local
	tokens  *auth.TokenManager
	logAPI  *slog.Logger
	contIT  atomic.Int64
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	_ = godotenv.Load(filepath.Join(raizBackend, ".env")) // no pisa variables ya definidas

	logAPI = slog.New(slog.NewTextHandler(io.Discard, nil))
	if os.Getenv("URBIA_IT_LOG") == "1" {
		logAPI = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	cfg, errCfg = config.Load()
	if errCfg == nil {
		// STORAGE_DIR relativo (./storage) es relativo a backend-go/, no a este paquete.
		if !filepath.IsAbs(cfg.StorageDir) {
			cfg.StorageDir = filepath.Join(raizBackend, cfg.StorageDir)
		}
		tokens = auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, errPool = db.NewPool(ctx, cfg.DatabaseURL)
		cancel()

		var errAlm error
		almacen, errAlm = storage.NewLocal(cfg.StorageDir, dominio.TamanoMaximoArchivo)
		if errAlm != nil {
			errCfg = errAlm
		}
	}

	codigo := m.Run()

	if pool != nil {
		pool.Close()
	}
	if almacen != nil {
		_ = almacen.Close()
	}
	os.Exit(codigo)
}

// requiereBase detiene la prueba si la configuración o PostgreSQL no están listos.
func requiereBase(t *testing.T) {
	t.Helper()
	if errCfg != nil {
		t.Fatalf("configuración de la API inválida (revisa backend-go/.env):\n%v", errCfg)
	}
	if errPool != nil {
		t.Fatalf("PostgreSQL no responde con DATABASE_URL de .env: %v", errPool)
	}
}

// requierePython devuelve un cliente gRPC con el servicio Python ya respondiendo.
func requierePython(t *testing.T) (*clients.ClienteDeteccion, *clients.InfoServicio) {
	t.Helper()
	requiereBase(t)
	if cfg.InferenceGRPCAddr == "" {
		t.Skip("INFERENCE_GRPC_ADDR vacía: se omite (levanta inference-python/scripts/servidor_grpc.py y configúrala)")
	}
	c, err := clients.NuevoClienteDeteccion(cfg.InferenceGRPCAddr, cfg.InferenceGRPCTimeout)
	if err != nil {
		t.Fatalf("cliente gRPC: %v", err)
	}
	t.Cleanup(func() { _ = c.Cerrar() })

	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatalf("el servicio Python no responde en %s (¿está corriendo servidor_grpc.py?): %v",
			cfg.InferenceGRPCAddr, err)
	}
	return c, info
}

// ------------------------------------------------------------------ API

type opcionesAPI struct {
	pool     *pgxpool.Pool               // nil = el pool compartido
	detector services.Detector           // nil = IA no configurada
	monitor  *services.InferenciaMonitor // nil = INFERENCE_URL no configurada
}

// nuevaAPI arma la API con el MISMO ensamblado que cmd/api (internal/app).
func nuevaAPI(t *testing.T, o opcionesAPI) http.Handler {
	t.Helper()
	requiereBase(t)
	if o.pool == nil {
		o.pool = pool
	}
	if o.monitor == nil {
		o.monitor = services.NewInferenciaMonitor(clients.NewInferenciaClient("", time.Second), time.Minute, logAPI)
	}
	r, err := app.NuevoRouter(app.Dependencias{
		Log:        logAPI,
		Pool:       o.pool,
		Almacen:    almacen,
		Tokens:     tokens,
		Inferencia: o.monitor,
		Detector:   o.detector,
		Cercania: dominio.ParametrosCercania{
			RadioM: cfg.DuplicadosRadioM, VentanaDias: cfg.DuplicadosVentanaDias, Limite: dominio.LimiteCercanosDefecto,
		},
		Prioridad: cfg.Prioridad,
		Version:   "integracion",
		Inicio:    time.Now(),
	})
	if err != nil {
		t.Fatalf("armar la API: %v", err)
	}
	return r
}

type respuesta struct {
	status int
	header http.Header
	crudo  string
	json   map[string]any
}

func hacer(t *testing.T, h http.Handler, req *http.Request) respuesta {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	r := respuesta{status: w.Code, header: w.Header(), crudo: w.Body.String()}
	if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(w.Body.Bytes(), &r.json); err != nil {
			t.Fatalf("respuesta JSON inválida (%d): %v\n%s", w.Code, err, r.crudo)
		}
	}
	return r
}

// valor navega el JSON con una ruta "data.sugerencia.estado". nil si no existe.
func (r respuesta) valor(ruta string) any {
	var actual any = r.json
	for _, parte := range strings.Split(ruta, ".") {
		m, ok := actual.(map[string]any)
		if !ok {
			return nil
		}
		actual = m[parte]
	}
	return actual
}

func (r respuesta) texto(ruta string) string {
	s, _ := r.valor(ruta).(string)
	return s
}

func (r respuesta) entero(t *testing.T, ruta string) int64 {
	t.Helper()
	f, ok := r.valor(ruta).(float64)
	if !ok {
		t.Fatalf("%s no es numérico en la respuesta: %s", ruta, r.crudo)
	}
	return int64(f)
}

func (r respuesta) exigir(t *testing.T, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("status = %d, se esperaba %d\n%s", r.status, status, r.crudo)
	}
}

// ------------------------------------------------------------- usuarios

// registrarCiudadano crea una cuenta nueva y devuelve su token.
func registrarCiudadano(t *testing.T, h http.Handler) string {
	t.Helper()
	correo := fmt.Sprintf("it-%d-%d@urbia.test", time.Now().UnixNano(), contIT.Add(1))
	cuerpo := fmt.Sprintf(`{"nombre":"Integración 08.2","correo":%q,"contrasena":"Integracion123"}`, correo)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/registro", strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	r := hacer(t, h, req)
	r.exigir(t, http.StatusCreated)
	return r.texto("data.token")
}

// ------------------------------------------------------------- reportes

// idSolicitud es el X-Request-ID: con él se encuentra la petición en los logs de Go y de Python.
func idSolicitud(caso string) string {
	return fmt.Sprintf("it082-%s-%d", caso, time.Now().UnixMilli()%1_000_000)
}

func crearReporte(t *testing.T, h http.Handler, token string, foto []byte, xRequestID string) respuesta {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	n := contIT.Add(1)
	campos := map[string]string{
		"descripcion":      marcaIT + " Prueba de integración Go–Python–PostgreSQL",
		"latitud":          strconv.FormatFloat(20.5235+float64(n%50)*0.0003, 'f', 6, 64), // Celaya
		"longitud":         "-100.815700",
		"origen_ubicacion": "gps",
		"precision_m":      "8.5",
	}
	for k, v := range campos {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	part, err := mw.CreateFormFile("foto", "integracion.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(foto); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reportes", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Request-ID", xRequestID)
	return hacer(t, h, req)
}

func get(t *testing.T, h http.Handler, ruta, token string) respuesta {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, ruta, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return hacer(t, h, req)
}

// ------------------------------------------------------------- imágenes

// fotoSintetica es un JPEG válido de 640×480 (degradado). La IA debe procesarla
// sin error; lo normal es que no detecte nada.
func fotoSintetica(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / 640), G: uint8(y * 255 / 480), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// contarOriginales cuenta las fotos (sin .tmp) que hay en STORAGE_DIR/originales.
func contarOriginales(t *testing.T) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(filepath.Join(almacen.Dir(), "originales"), func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !strings.HasSuffix(d.Name(), ".tmp") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("recorrer STORAGE_DIR: %v", err)
	}
	return n
}

// ------------------------------------------------------------- base de datos

type filaDeteccion struct {
	Resultado   string
	Clase       *string
	Confianza   *float64
	Modelo      *string
	Version     *string
	CodigoError *string
	Ms          *float64
}

func ultimaDeteccion(t *testing.T, idReporte int64) filaDeteccion {
	t.Helper()
	var d filaDeteccion
	err := pool.QueryRow(context.Background(), `
		SELECT resultado, clase_detectada, confianza::float8, modelo, version_modelo,
		       codigo_error, milisegundos::float8
		FROM deteccion WHERE id_reporte = $1
		ORDER BY id_deteccion DESC LIMIT 1`, idReporte).
		Scan(&d.Resultado, &d.Clase, &d.Confianza, &d.Modelo, &d.Version, &d.CodigoError, &d.Ms)
	if err != nil {
		t.Fatalf("leer la detección del reporte %d: %v", idReporte, err)
	}
	return d
}

func rutaArchivo(t *testing.T, idArchivo int64) string {
	t.Helper()
	var ruta string
	if err := pool.QueryRow(context.Background(),
		`SELECT ruta FROM archivo WHERE id_archivo = $1`, idArchivo).Scan(&ruta); err != nil {
		t.Fatalf("leer archivo.ruta de %d: %v", idArchivo, err)
	}
	return ruta
}

func texto(p *string) string {
	if p == nil {
		return "<NULL>"
	}
	return *p
}

// puertoCerrado devuelve host:puerto donde nadie escucha (servicio "apagado").
func puertoCerrado(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dir := l.Addr().String()
	_ = l.Close()
	return dir
}

// esperar repite cond hasta que sea true o se acabe el tiempo.
func esperar(t *testing.T, limite time.Duration, cond func() bool) {
	t.Helper()
	fin := time.Now().Add(limite)
	for !cond() {
		if time.Now().After(fin) {
			t.Fatalf("la condición no se cumplió en %v", limite)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
