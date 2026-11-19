//go:build integracion

package integracion

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/db"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
)

// Subtarea 1 — Probar Go conectándose a PostgreSQL.
func TestPostgres_GoSeConecta(t *testing.T) {
	requiereBase(t)
	ctx := context.Background()

	t.Run("usuario de la API y esquema urbia", func(t *testing.T) {
		var usuario, esquema string
		if err := pool.QueryRow(ctx, `SELECT current_user, current_schema()`).Scan(&usuario, &esquema); err != nil {
			t.Fatal(err)
		}
		t.Logf("conectado como %s, esquema %s", usuario, esquema)
		if esquema != "urbia" {
			t.Errorf("current_schema = %q; DATABASE_URL debe llevar search_path=urbia,public", esquema)
		}
		if usuario == "postgres" {
			t.Error("la API no debe conectarse como superusuario (usa urbia_app)")
		}
	})

	t.Run("migraciones aplicadas", func(t *testing.T) {
		for _, tabla := range []string{
			"usuario", "categoria", "reporte", "archivo", "deteccion", "historial_estado",
			"transicion_estado", "clasificacion", "calculo_prioridad",
		} {
			var existe bool
			if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, tabla).Scan(&existe); err != nil {
				t.Fatal(err)
			}
			if !existe {
				t.Errorf("falta la tabla %s (¿corriste goose up?)", tabla)
			}
		}
	})

	t.Run("PostGIS con geography", func(t *testing.T) {
		var version string
		if err := pool.QueryRow(ctx, `SELECT postgis_lib_version()`).Scan(&version); err != nil {
			t.Fatalf("PostGIS no disponible: %v", err)
		}
		// 0.0009° de latitud ≈ 100 m: la misma función que usan cercanos y prioridad.
		var metros float64
		err := pool.QueryRow(ctx, `
			SELECT ST_Distance(
				ST_SetSRID(ST_MakePoint(-100.8157, 20.5235), 4326)::geography,
				ST_SetSRID(ST_MakePoint(-100.8157, 20.5244), 4326)::geography)`).Scan(&metros)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PostGIS %s, distancia de prueba %.1f m", version, metros)
		if metros < 95 || metros > 105 {
			t.Errorf("distancia = %.1f m, se esperaban ~100 m", metros)
		}
	})

	t.Run("catálogo de categorías", func(t *testing.T) {
		repo := repositories.NewCategoriaRepository(pool)
		for _, nombre := range []string{"bache", "basura"} {
			if _, err := repo.BuscarActivaPorNombre(ctx, nombre); err != nil {
				t.Errorf("la categoría %q no está activa: %v", nombre, err)
			}
		}
	})

	t.Run("credenciales incorrectas fallan sin filtrar la contraseña", func(t *testing.T) {
		u, err := url.Parse(cfg.DatabaseURL)
		if err != nil {
			t.Fatal(err)
		}
		const falsa = "contrasena-incorrecta-IT-082"
		u.User = url.UserPassword(u.User.Username(), falsa)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		p, err := db.NewPool(ctx, u.String())
		if err == nil {
			p.Close()
			t.Fatal("se conectó con una contraseña incorrecta")
		}
		if strings.Contains(err.Error(), falsa) {
			t.Errorf("el error expone la contraseña: %v", err)
		}
		t.Logf("error esperado: %v", err)
	})
}
