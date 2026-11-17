package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Pruebas de los middlewares de seguridad de la HU 09.1 (HU 09.2).

func ok(c *gin.Context) { c.Status(http.StatusOK) }

func servir(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func codigoError(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var b response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil || b.Error == nil {
		t.Fatalf("se esperaba un error JSON, llegó %q", w.Body.String())
	}
	return b.Error.Code
}

// ---------- CORS ----------

func routerCORS() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(CORS([]string{"http://localhost:5173"}))
	r.GET("/api", ok)
	r.POST("/api", ok)
	return r
}

func preflight(origen string) *http.Request {
	req := httptest.NewRequest(http.MethodOptions, "/api", nil)
	req.Header.Set("Origin", origen)
	req.Header.Set("Access-Control-Request-Method", "POST")
	return req
}

func TestCORS_PreflightPermitido(t *testing.T) {
	w := servir(routerCORS(), preflight("http://localhost:5173"))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, esperado 204", w.Code)
	}
	h := w.Header()
	if h.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("Allow-Origin = %q", h.Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(h.Get("Access-Control-Allow-Headers"), "Authorization") ||
		!strings.Contains(h.Get("Access-Control-Allow-Methods"), "PATCH") {
		t.Errorf("faltan métodos o encabezados permitidos: %v", h)
	}
	if h.Get("Access-Control-Allow-Credentials") != "" {
		t.Error("no debe permitir credenciales: la API no usa cookies")
	}
	if !strings.Contains(strings.Join(h.Values("Vary"), ","), "Origin") {
		t.Error("falta Vary: Origin (cachés intermedias)")
	}
}

func TestCORS_OrigenSinDistinguirMayusculas(t *testing.T) {
	w := servir(routerCORS(), preflight("HTTP://LOCALHOST:5173"))
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d", w.Code)
	}
}

func TestCORS_PreflightDeOrigenNoPermitido(t *testing.T) {
	w := servir(routerCORS(), preflight("http://sitio-malicioso.example"))
	if w.Code != http.StatusForbidden || codigoError(t, w) != response.CodeForbidden {
		t.Fatalf("status = %d, cuerpo = %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("no debe devolver Allow-Origin a un origen no permitido")
	}
}

func TestCORS_PeticionSimpleDeOrigenNoPermitido(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "http://sitio-malicioso.example")
	w := servir(routerCORS(), req)
	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("se atiende sin encabezados CORS; status=%d ACAO=%q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_SinOrigenNoHaceNada(t *testing.T) {
	w := servir(routerCORS(), httptest.NewRequest(http.MethodGet, "/api", nil))
	if w.Code != http.StatusOK || w.Header().Get("Vary") != "" {
		t.Errorf("sin Origin no debe tocar la respuesta: %d %v", w.Code, w.Header())
	}
}

func TestCORS_SinOrigenesConfigurados(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(nil))
	r.GET("/api", ok)
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	if w := servir(r, req); w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("sin CORS_ALLOWED_ORIGINS ningún origen se permite")
	}
}

// ---------- Límite de cuerpo ----------

func routerCuerpo() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LimiteCuerpo())
	r.POST("/leer", func(c *gin.Context) {
		n, err := io.Copy(io.Discard, c.Request.Body)
		var mbe *http.MaxBytesError
		if err != nil && errors.As(err, &mbe) {
			c.String(http.StatusRequestEntityTooLarge, "cortado en %d", n)
			return
		}
		c.String(http.StatusOK, "%d", n)
	})
	return r
}

func TestLimiteCuerpo_JSONPequenoPasa(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/leer", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	if w := servir(routerCuerpo(), req); w.Code != http.StatusOK || w.Body.String() != "7" {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
}

func TestLimiteCuerpo_ContentLengthGrandeSeRechazaSinLeer(t *testing.T) {
	cuerpo := bytes.Repeat([]byte("x"), int(LimiteCuerpoJSON)+1)
	req := httptest.NewRequest(http.MethodPost, "/leer", bytes.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	w := servir(routerCuerpo(), req)
	if w.Code != http.StatusRequestEntityTooLarge || codigoError(t, w) != response.CodeBodyTooLarge {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
}

// Sin Content-Length (chunked) el corte ocurre al leer.
func TestLimiteCuerpo_ChunkedSeCortaAlLeer(t *testing.T) {
	cuerpo := bytes.Repeat([]byte("x"), int(LimiteCuerpoJSON)+10)
	req := httptest.NewRequest(http.MethodPost, "/leer", io.MultiReader(bytes.NewReader(cuerpo)))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	w := servir(routerCuerpo(), req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, cuerpo = %s", w.Code, w.Body.String())
	}
}

// Un multipart recibe el tope de imagen (10 MB + 1 MB), no el de JSON.
func TestLimiteCuerpo_MultipartUsaElTopeDeImagen(t *testing.T) {
	cuerpo := bytes.Repeat([]byte("x"), 200<<10) // 200 KiB > 64 KiB
	req := httptest.NewRequest(http.MethodPost, "/leer", bytes.NewReader(cuerpo))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	if w := servir(routerCuerpo(), req); w.Code != http.StatusOK {
		t.Errorf("status = %d, un multipart de 200 KiB debe pasar", w.Code)
	}
}

// ---------- Límite de intentos de login ----------

func routerLogin(max int, ventana time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", LimiteIntentos(max, ventana), func(c *gin.Context) {
		if c.GetHeader("X-Clave") == "correcta" {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusUnauthorized)
	})
	return r
}

func intentoLogin(r *gin.Engine, ip, clave string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = ip + ":40000"
	req.Header.Set("X-Clave", clave)
	return servir(r, req)
}

func TestLimiteIntentos_BloqueaTrasMaxFallos(t *testing.T) {
	r := routerLogin(3, time.Minute)
	for i := 1; i <= 3; i++ {
		if w := intentoLogin(r, "192.168.1.20", "mala"); w.Code != http.StatusUnauthorized {
			t.Fatalf("intento %d: status = %d, esperado 401", i, w.Code)
		}
	}
	w := intentoLogin(r, "192.168.1.20", "correcta")
	if w.Code != http.StatusTooManyRequests || codigoError(t, w) != response.CodeTooManyRequests {
		t.Fatalf("tras 3 fallos debe bloquear aun con la clave correcta: %d %s", w.Code, w.Body.String())
	}
	if ra := w.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Errorf("Retry-After = %q", ra)
	}
}

func TestLimiteIntentos_PorIP(t *testing.T) {
	r := routerLogin(2, time.Minute)
	intentoLogin(r, "192.168.1.20", "mala")
	intentoLogin(r, "192.168.1.20", "mala")
	if w := intentoLogin(r, "192.168.1.21", "correcta"); w.Code != http.StatusOK {
		t.Errorf("otra IP no debe quedar bloqueada: %d", w.Code)
	}
}

func TestLimiteIntentos_LoginCorrectoNoCuentaComoFallo(t *testing.T) {
	r := routerLogin(2, time.Minute)
	for i := 0; i < 5; i++ {
		if w := intentoLogin(r, "192.168.1.30", "correcta"); w.Code != http.StatusOK {
			t.Fatalf("intento %d correcto bloqueado: %d", i, w.Code)
		}
	}
}

func TestLimiteIntentos_SeLiberaAlTerminarLaVentana(t *testing.T) {
	r := routerLogin(1, 50*time.Millisecond)
	intentoLogin(r, "192.168.1.40", "mala")
	if w := intentoLogin(r, "192.168.1.40", "correcta"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("debería estar bloqueada: %d", w.Code)
	}
	time.Sleep(80 * time.Millisecond)
	if w := intentoLogin(r, "192.168.1.40", "correcta"); w.Code != http.StatusOK {
		t.Errorf("al terminar la ventana debe liberarse: %d", w.Code)
	}
}

// X-Forwarded-For no sirve para cambiar de "IP" y saltarse el límite.
func TestLimiteIntentos_IgnoraXForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.POST("/login", LimiteIntentos(1, time.Minute), func(c *gin.Context) { c.Status(http.StatusUnauthorized) })

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.168.1.50:40000"
	servir(r, req)

	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.168.1.50:40001"
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	if w := servir(r, req); w.Code != http.StatusTooManyRequests {
		t.Errorf("con X-Forwarded-For falso respondió %d, esperado 429", w.Code)
	}
}

// ---------- Encabezados, X-Request-ID y logs ----------

func TestCabecerasSeguridad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CabecerasSeguridad("/docs"))
	r.GET("/api", ok)
	r.GET("/docs", ok)
	r.GET("/foto", func(c *gin.Context) { c.Header("Cache-Control", "private, no-store"); c.Status(http.StatusOK) })

	w := servir(r, httptest.NewRequest(http.MethodGet, "/api", nil))
	esperados := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}
	for k, v := range esperados {
		if w.Header().Get(k) != v {
			t.Errorf("%s = %q, esperado %q", k, w.Header().Get(k), v)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Error("falta la CSP en la API")
	}
	if w := servir(r, httptest.NewRequest(http.MethodGet, "/docs", nil)); w.Header().Get("Content-Security-Policy") != "" {
		t.Error("el visor de documentación no lleva CSP (carga scripts del CDN)")
	}
	if w := servir(r, httptest.NewRequest(http.MethodGet, "/foto", nil)); w.Header().Get("Cache-Control") != "private, no-store" {
		t.Error("un handler debe poder sobrescribir Cache-Control")
	}
}

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, c.GetString(RequestIDKey)) })

	casos := []struct {
		nombre, enviado string
		reutiliza       bool
	}{
		{"limpio", "prueba-091.ok_1", true},
		{"vacío", "", false},
		{"con espacios y comillas", `falso" usuario=admin`, false},
		{"salto de línea", "a\nb", false},
		{"más de 64", strings.Repeat("a", 65), false},
		{"exactamente 64", strings.Repeat("a", 64), true},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.enviado != "" {
				req.Header.Set(RequestIDHeader, tc.enviado)
			}
			w := servir(r, req)
			id := w.Header().Get(RequestIDHeader)
			if tc.reutiliza && id != tc.enviado {
				t.Errorf("debió reutilizar %q, llegó %q", tc.enviado, id)
			}
			if !tc.reutiliza && (id == tc.enviado || !patronRequestID.MatchString(id)) {
				t.Errorf("debió generar uno nuevo, llegó %q", id)
			}
			if w.Body.String() != id {
				t.Error("el id del contexto y el del encabezado deben coincidir")
			}
		})
	}
}

func TestConsultaSeguraOcultaDatosSensibles(t *testing.T) {
	casos := map[string]string{
		"bbox=-100.9,20.5,-100.8,20.6&pagina=2":   "bbox=***&pagina=2",
		"token=eyJhbGci&por_pagina=10":            "por_pagina=10&token=***",
		"Latitud=20.5&LONGITUD=-100.8":            "LONGITUD=***&Latitud=***",
		"correo=ana%40ejemplo.com&contrasena=abc": "contrasena=***&correo=***",
		"estado=pendiente":                        "estado=pendiente",
		"%zz":                                     "(consulta ilegible omitida)",
	}
	for entrada, esperado := range casos {
		if obtenido := consultaSegura(entrada); obtenido != esperado {
			t.Errorf("consultaSegura(%q) = %q, esperado %q", entrada, obtenido, esperado)
		}
	}
}

func TestLoggerNoEscribeCoordenadasNiTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	r := gin.New()
	r.Use(RequestID(), Logger(log))
	r.GET("/reportes", ok)

	req := httptest.NewRequest(http.MethodGet, "/reportes?bbox=-100.81,20.52,-100.80,20.53&token=secreto123", nil)
	req.Header.Set("Authorization", "Bearer secreto-del-encabezado")
	servir(r, req)

	linea := buf.String()
	for _, prohibido := range []string{"-100.81", "20.52", "secreto123", "secreto-del-encabezado"} {
		if strings.Contains(linea, prohibido) {
			t.Errorf("el log contiene %q: %s", prohibido, linea)
		}
	}
	if !strings.Contains(linea, "status=200") || !strings.Contains(linea, "request_id=") {
		t.Errorf("el log debe conservar status y request_id: %s", linea)
	}
}

func TestRecoveryConviertePanicEn500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	r := gin.New()
	r.Use(Recovery(slog.New(slog.NewTextHandler(&buf, nil))))
	r.GET("/", func(c *gin.Context) { panic("detalle interno: conexión a 10.0.0.5") })

	w := servir(r, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusInternalServerError || codigoError(t, w) != response.CodeInternal {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "10.0.0.5") {
		t.Error("la respuesta no debe filtrar el panic")
	}
	if !strings.Contains(buf.String(), "panic recuperado") {
		t.Error("el panic debe quedar en el log")
	}
}
