package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"

	"github.com/PL290723/URBIA/backend-go/internal/apidocs"
	"github.com/PL290723/URBIA/backend-go/internal/auth"
	"github.com/PL290723/URBIA/backend-go/internal/response"
)

// Estas pruebas mantienen openapi.yaml alineado con la implementación (HU 08.1):
// si se agrega, quita o protege una ruta sin actualizar la especificación, fallan.

var metodosHTTP = []string{"get", "post", "put", "patch", "delete", "head", "options"}

var paramGin = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// routerDePrueba arma el router real con handlers nil: sirve para listar rutas y
// probar los middlewares. Un handler nil que se llegue a ejecutar provoca un panic
// que Recovery convierte en 500, nunca en 401.
func routerDePrueba(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens := auth.NewTokenManager("secreto-solo-para-pruebas-de-openapi", time.Hour)
	return New(log, tokens, Handlers{})
}

func cargarEspecificacion(t *testing.T) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal(apidocs.Especificacion(), &doc); err != nil {
		t.Fatalf("openapi.yaml no es YAML válido: %v", err)
	}
	return doc
}

func mapa(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		r := make(map[string]any, len(m))
		for k, val := range m {
			r[strings.TrimSpace(toString(k))] = val
		}
		return r
	}
	return nil
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case uint64:
		return strconv.FormatUint(x, 10)
	case int64:
		return strconv.FormatInt(x, 10)
	}
	return ""
}

// operacion es una operación documentada con lo que necesitan las pruebas.
type operacion struct {
	clave     string // "GET /api/v1/reportes/{id}"
	metodo    string
	ruta      string
	datos     map[string]any
	protegida bool
}

func operaciones(t *testing.T, doc map[string]any) []operacion {
	t.Helper()
	globalProtegida := len(lista(doc["security"])) > 0

	paths := mapa(doc["paths"])
	if len(paths) == 0 {
		t.Fatal("openapi.yaml no tiene paths")
	}
	var ops []operacion
	for ruta, item := range paths {
		for metodo, op := range mapa(item) {
			if !slices.Contains(metodosHTTP, metodo) {
				continue
			}
			datos := mapa(op)
			protegida := globalProtegida
			if sec, ok := datos["security"]; ok {
				protegida = len(lista(sec)) > 0
			}
			ops = append(ops, operacion{
				clave:     strings.ToUpper(metodo) + " " + ruta,
				metodo:    strings.ToUpper(metodo),
				ruta:      ruta,
				datos:     datos,
				protegida: protegida,
			})
		}
	}
	slices.SortFunc(ops, func(a, b operacion) int { return strings.Compare(a.clave, b.clave) })
	return ops
}

func lista(v any) []any {
	l, _ := v.([]any)
	return l
}

func TestOpenAPI_VersionYServidores(t *testing.T) {
	doc := cargarEspecificacion(t)
	if v, _ := doc["openapi"].(string); !strings.HasPrefix(v, "3.1") {
		t.Errorf("se esperaba OpenAPI 3.1.x, llegó %q", v)
	}
	if len(lista(doc["servers"])) == 0 {
		t.Error("falta al menos un servidor en servers")
	}
	if mapa(mapa(mapa(doc["components"])["securitySchemes"])["bearerAuth"]) == nil {
		t.Error("falta el esquema de seguridad bearerAuth")
	}
}

// Toda ruta de Gin está documentada y toda ruta documentada existe.
func TestOpenAPI_RutasAlineadasConGin(t *testing.T) {
	r := routerDePrueba(t)
	doc := cargarEspecificacion(t)

	implementadas := map[string]bool{}
	for _, ri := range r.Routes() {
		implementadas[ri.Method+" "+paramGin.ReplaceAllString(ri.Path, "{$1}")] = true
	}
	documentadas := map[string]bool{}
	for _, op := range operaciones(t, doc) {
		documentadas[op.clave] = true
	}

	for _, clave := range claves(implementadas) {
		if !documentadas[clave] {
			t.Errorf("ruta implementada sin documentar en openapi.yaml: %s", clave)
		}
	}
	for _, clave := range claves(documentadas) {
		if !implementadas[clave] {
			t.Errorf("ruta documentada que no existe en el router: %s", clave)
		}
	}
}

func claves(m map[string]bool) []string {
	r := make([]string, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	slices.Sort(r)
	return r
}

// Cada operación trae lo mínimo que necesita el frontend.
func TestOpenAPI_OperacionesCompletas(t *testing.T) {
	doc := cargarEspecificacion(t)
	ids := map[string]string{}

	for _, op := range operaciones(t, doc) {
		t.Run(op.clave, func(t *testing.T) {
			id, _ := op.datos["operationId"].(string)
			switch {
			case id == "":
				t.Error("falta operationId")
			case ids[id] != "":
				t.Errorf("operationId %q repetido (también en %s)", id, ids[id])
			default:
				ids[id] = op.clave
			}
			if s, _ := op.datos["summary"].(string); s == "" {
				t.Error("falta summary")
			}
			if len(lista(op.datos["tags"])) == 0 {
				t.Error("falta tags")
			}
			if d, _ := op.datos["description"].(string); !strings.Contains(d, "**Permisos:**") {
				t.Error("la descripción debe indicar **Permisos:**")
			}

			respuestas := mapa(op.datos["responses"])
			exito := false
			for codigo := range respuestas {
				if strings.HasPrefix(codigo, "2") {
					exito = true
				}
				if n, err := strconv.Atoi(codigo); err != nil || n < 100 || n > 599 {
					t.Errorf("código HTTP inválido %q (debe ir entre comillas, p. ej. '200')", codigo)
				}
			}
			if !exito {
				t.Error("no documenta ninguna respuesta 2xx")
			}
			if op.protegida {
				if _, ok := respuestas["401"]; !ok {
					t.Error("está protegida con bearerAuth pero no documenta el 401")
				}
			}
			if strings.Contains(op.ruta, "{") {
				if _, ok := respuestas["400"]; !ok && op.protegida {
					t.Error("recibe un id en la ruta pero no documenta el 400 de id inválido")
				}
			}
		})
	}
}

// La seguridad documentada coincide con el middleware real: sin token, una ruta
// protegida responde 401 y una pública no.
func TestOpenAPI_SeguridadCoincideConMiddleware(t *testing.T) {
	r := routerDePrueba(t)
	doc := cargarEspecificacion(t)

	for _, op := range operaciones(t, doc) {
		t.Run(op.clave, func(t *testing.T) {
			ruta := regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(op.ruta, "1")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(op.metodo, ruta, nil))

			if op.protegida && w.Code != http.StatusUnauthorized {
				t.Errorf("documentada como protegida, pero sin token respondió %d", w.Code)
			}
			if !op.protegida && w.Code == http.StatusUnauthorized {
				t.Error("documentada como pública (security: []), pero sin token respondió 401")
			}
		})
	}
}

// Todo "$ref" apunta a algo que existe (un error de dedo rompe el visor).
func TestOpenAPI_ReferenciasResuelven(t *testing.T) {
	doc := cargarEspecificacion(t)
	var recorrer func(v any, donde string)
	recorrer = func(v any, donde string) {
		if m := mapa(v); m != nil {
			for k, val := range m {
				if k == "$ref" {
					ref, _ := val.(string)
					if !resolver(doc, ref) {
						t.Errorf("%s: $ref %q no existe", donde, ref)
					}
					continue
				}
				recorrer(val, donde+"/"+k)
			}
			return
		}
		for i, val := range lista(v) {
			recorrer(val, donde+"/"+strconv.Itoa(i))
		}
	}
	recorrer(doc, "#")
}

func resolver(doc map[string]any, ref string) bool {
	puntero, ok := strings.CutPrefix(ref, "#/")
	if !ok {
		return false // solo se usan referencias internas
	}
	var actual any = doc
	for _, parte := range strings.Split(puntero, "/") {
		parte = strings.NewReplacer("~1", "/", "~0", "~").Replace(parte)
		m := mapa(actual)
		if m == nil {
			return false
		}
		if actual, ok = m[parte]; !ok {
			return false
		}
	}
	return true
}

// El catálogo de códigos de error documentado es exactamente el del paquete response.
func TestOpenAPI_CodigosDeError(t *testing.T) {
	doc := cargarEspecificacion(t)
	esquema := mapa(mapa(mapa(doc["components"])["schemas"])["CodigoError"])

	var documentados []string
	for _, v := range lista(esquema["enum"]) {
		s, _ := v.(string)
		documentados = append(documentados, s)
	}
	implementados := []string{
		response.CodeBadRequest, response.CodeValidation, response.CodeUnauthorized,
		response.CodeForbidden, response.CodeNotFound, response.CodeMethodNotAllowed,
		response.CodeConflict, response.CodeInternal, response.CodeServiceUnavailable,
		response.CodePayloadTooLarge, response.CodeUnsupportedMedia, response.CodeInvalidImage,
	}
	slices.Sort(documentados)
	slices.Sort(implementados)
	if !slices.Equal(documentados, implementados) {
		t.Errorf("CodigoError no coincide con response.Code*\n documentados: %v\nimplementados: %v",
			documentados, implementados)
	}
}

func TestOpenAPI_Publicacion(t *testing.T) {
	r := routerDePrueba(t)

	t.Run("especificación YAML", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, apidocs.RutaEspecificacion, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, se esperaba 200", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
			t.Errorf("Content-Type = %q", ct)
		}
		if w.Body.String() != string(apidocs.Especificacion()) {
			t.Error("el cuerpo no es la especificación embebida")
		}
	})

	t.Run("visor HTML", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, apidocs.RutaVisor, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, se esperaba 200", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q", ct)
		}
		if !strings.Contains(w.Body.String(), apidocs.RutaEspecificacion) {
			t.Error("el visor no carga /api/v1/openapi.yaml")
		}
	})

	t.Run("solo lectura", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, apidocs.RutaEspecificacion, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST respondió %d, se esperaba 405", w.Code)
		}
	})
}
