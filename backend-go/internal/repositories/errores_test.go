package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/models"
)

// Pruebas unitarias de cómo los repositorios traducen los errores de PostgreSQL
// a errores de dominio (HU 09.2). No necesitan base de datos: se simula la fila
// (pgx.Row) o el ejecutor que devolvería pgx.

// filaFalsa es un pgx.Row: Scan devuelve err o copia los valores en los destinos.
type filaFalsa struct {
	err     error
	valores []any
}

func (f filaFalsa) Scan(destinos ...any) error {
	if f.err != nil {
		return f.err
	}
	for i, d := range destinos {
		switch p := d.(type) {
		case *int64:
			*p = f.valores[i].(int64)
		case *string:
			*p = f.valores[i].(string)
		case *time.Time:
			*p = f.valores[i].(time.Time)
		default:
			return fmt.Errorf("tipo de destino no soportado en la prueba: %T", d)
		}
	}
	return nil
}

// ejecutorFalso cumple la interfaz ejecutor y siempre devuelve la misma fila.
type ejecutorFalso struct {
	fila     filaFalsa
	sql      string
	recibido []any
}

func (e *ejecutorFalso) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	e.sql, e.recibido = sql, args
	return e.fila
}

func (e *ejecutorFalso) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func pgError(codigo, restriccion string) error {
	return &pgconn.PgError{Code: codigo, ConstraintName: restriccion, Message: "detalle interno de PostgreSQL"}
}

// ---------- usuarios ----------

func TestScanUsuarioSinFilasEsNoEncontrado(t *testing.T) {
	_, err := scanUsuario(filaFalsa{err: pgx.ErrNoRows})
	if !errors.Is(err, ErrNoEncontrado) {
		t.Fatalf("pgx.ErrNoRows debe traducirse a ErrNoEncontrado, llegó %v", err)
	}
}

func TestScanUsuarioOtroErrorSeEnvuelve(t *testing.T) {
	caida := errors.New("conexión cerrada")
	_, err := scanUsuario(filaFalsa{err: caida})
	if errors.Is(err, ErrNoEncontrado) {
		t.Fatal("una caída de la BD no debe confundirse con 'no encontrado' (daría 401/404 engañoso)")
	}
	if !errors.Is(err, caida) || !strings.Contains(err.Error(), "leer usuario") {
		t.Errorf("el error original debe conservarse con contexto: %v", err)
	}
}

func TestScanUsuarioLeeTodosLosCampos(t *testing.T) {
	ahora := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	u, err := scanUsuario(filaFalsa{valores: []any{
		int64(7), "Ana", "ana@ejemplo.com", "$2a$12$hash", "administrador", ahora, ahora,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 7 || u.Correo != "ana@ejemplo.com" || u.Rol != dominio.RolAdministrador || u.HashContrasena == "" {
		t.Errorf("usuario mal leído: %+v", u)
	}
}

// ---------- reportes ----------

func TestTraducirErrorReporte(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  error
		esperado error
	}{
		{"usuario inexistente", pgError("23503", "fk_reporte_usuario"), ErrUsuarioNoExiste},
		{"categoría inexistente", pgError("23503", "fk_reporte_categoria"), ErrCategoriaNoExiste},
		{"envuelto", fmt.Errorf("insertar: %w", pgError("23503", "fk_reporte_categoria")), ErrCategoriaNoExiste},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			if err := traducirErrorReporte(tc.entrada); !errors.Is(err, tc.esperado) {
				t.Errorf("obtenido %v, esperado %v", err, tc.esperado)
			}
		})
	}
}

func TestTraducirErrorReporteDesconocidoSeEnvuelve(t *testing.T) {
	for _, entrada := range []error{
		pgError("23503", "fk_otra_tabla"), // FK que no conocemos
		pgError("23514", "ck_reporte_descripcion"),
		errors.New("timeout"),
	} {
		err := traducirErrorReporte(entrada)
		if errors.Is(err, ErrUsuarioNoExiste) || errors.Is(err, ErrCategoriaNoExiste) {
			t.Errorf("%v no debe traducirse a un error de dominio", entrada)
		}
		if !errors.Is(err, entrada) {
			t.Errorf("debe conservar el error original: %v", err)
		}
	}
}

// ---------- archivos ----------

func TestInsertarArchivoReporteInexistente(t *testing.T) {
	db := &ejecutorFalso{fila: filaFalsa{err: pgError("23503", "fk_archivo_reporte")}}
	err := insertarArchivo(context.Background(), db, &models.Archivo{IDReporte: 999})
	if !errors.Is(err, ErrReporteNoExiste) {
		t.Fatalf("FK violada debe ser ErrReporteNoExiste, llegó %v", err)
	}
}

func TestInsertarArchivoOtroErrorSeEnvuelve(t *testing.T) {
	db := &ejecutorFalso{fila: filaFalsa{err: pgError("23514", "ck_archivo_tamano")}}
	err := insertarArchivo(context.Background(), db, &models.Archivo{})
	if errors.Is(err, ErrReporteNoExiste) || !strings.Contains(err.Error(), "crear archivo") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestInsertarArchivoUsaParametros(t *testing.T) {
	ahora := time.Now()
	db := &ejecutorFalso{fila: filaFalsa{valores: []any{int64(31), ahora}}}
	a := &models.Archivo{IDReporte: 12, Tipo: "imagen", Variante: dominio.VarianteOriginal,
		Ruta: "originales/2026/11/abc.jpg", MIME: "image/jpeg", Tamano: 1024, SHA256: "f00"}

	if err := insertarArchivo(context.Background(), db, a); err != nil {
		t.Fatal(err)
	}
	if a.ID != 31 || !a.FechaSubida.Equal(ahora) {
		t.Errorf("no se leyó el RETURNING: %+v", a)
	}
	if strings.Contains(db.sql, "originales/") || !strings.Contains(db.sql, "$7") {
		t.Errorf("la ruta debe ir como parámetro, no dentro del SQL: %s", db.sql)
	}
	if len(db.recibido) != 7 || db.recibido[3] != a.Ruta {
		t.Errorf("parámetros inesperados: %v", db.recibido)
	}
}

// ---------- consultas parametrizadas ----------

func TestConstruirFiltrosNuncaConcatenaValores(t *testing.T) {
	cat := int16(3)
	desde := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	malicioso := dominio.Estado("pendiente'); DROP TABLE reporte; --")
	f := dominio.FiltrosReporte{
		Pagina: 1, PorPagina: 20,
		Estados:   []dominio.Estado{dominio.EstadoPendiente, malicioso},
		Categoria: &cat,
		Desde:     &desde,
		BBox:      &dominio.BBox{MinLon: -100.9, MinLat: 20.5, MaxLon: -100.8, MaxLat: 20.6},
		SoloMios:  true,
	}

	q := construirFiltros(f, 42)
	where := q.where()

	if strings.Contains(where, "DROP") || strings.Contains(where, "pendiente") || strings.Contains(where, "-100.9") {
		t.Fatalf("un valor terminó dentro del SQL: %s", where)
	}
	// estados, categoría, desde, 4 de la bbox y el usuario = 8 parámetros
	if len(q.args) != 8 || !strings.Contains(where, "$8") {
		t.Errorf("se esperaban 8 parámetros ($1..$8): %d — %s", len(q.args), where)
	}
	if q.args[len(q.args)-1] != int64(42) {
		t.Errorf("el último parámetro debe ser el id del usuario: %v", q.args)
	}
}

func TestConstruirFiltrosSinFiltros(t *testing.T) {
	q := construirFiltros(dominio.FiltrosReporte{Pagina: 1, PorPagina: 20}, 1)
	if q.where() != "" || len(q.args) != 0 {
		t.Errorf("sin filtros no hay WHERE: %q %v", q.where(), q.args)
	}
}
