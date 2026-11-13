package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

func nuevo(t *testing.T, maximo int64) *Local {
	t.Helper()
	l, err := NewLocal(t.TempDir(), maximo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func contarArchivos(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func TestGuardarAbrirEliminar(t *testing.T) {
	l := nuevo(t, 1024)
	contenido := []byte("contenido de prueba")

	g, err := l.Guardar(context.Background(), dominio.VarianteOriginal, "jpg", bytes.NewReader(contenido))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(g.Ruta, "originales/") || !patronRuta.MatchString(g.Ruta) {
		t.Fatalf("ruta inesperada: %s", g.Ruta)
	}
	if g.Tamano != int64(len(contenido)) {
		t.Errorf("tamaño = %d, se esperaba %d", g.Tamano, len(contenido))
	}
	suma := sha256.Sum256(contenido)
	if g.SHA256 != hex.EncodeToString(suma[:]) {
		t.Error("sha256 incorrecto")
	}

	f, err := l.Abrir(g.Ruta)
	if err != nil {
		t.Fatal(err)
	}
	leido, _ := io.ReadAll(f)
	_ = f.Close()
	if !bytes.Equal(leido, contenido) {
		t.Error("el contenido leído no coincide")
	}

	if err := l.Eliminar(g.Ruta); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Abrir(g.Ruta); !errors.Is(err, ErrNoEncontrado) {
		t.Errorf("tras eliminar se esperaba ErrNoEncontrado, se obtuvo %v", err)
	}
	if err := l.Eliminar(g.Ruta); err != nil {
		t.Errorf("eliminar dos veces no debería fallar: %v", err)
	}
}

func TestNombresNoPredecibles(t *testing.T) {
	l := nuevo(t, 1024)
	a, _ := l.Guardar(context.Background(), dominio.VarianteOriginal, "png", strings.NewReader("igual"))
	b, _ := l.Guardar(context.Background(), dominio.VarianteOriginal, "png", strings.NewReader("igual"))
	if a == nil || b == nil || a.Ruta == b.Ruta {
		t.Fatal("dos archivos con el mismo contenido deben tener rutas distintas")
	}
}

func TestVarianteProcesada(t *testing.T) {
	l := nuevo(t, 1024)
	g, err := l.Guardar(context.Background(), dominio.VarianteProcesada, "webp", strings.NewReader("x"))
	if err != nil || !strings.HasPrefix(g.Ruta, "procesadas/") {
		t.Fatalf("ruta=%v err=%v", g, err)
	}
	if _, err := l.Guardar(context.Background(), "otra", "jpg", strings.NewReader("x")); err == nil {
		t.Error("una variante inválida debería rechazarse")
	}
}

func TestTamanoMaximoYVacio(t *testing.T) {
	l := nuevo(t, 10)
	if _, err := l.Guardar(context.Background(), dominio.VarianteOriginal, "jpg", strings.NewReader("12345678901")); !errors.Is(err, ErrDemasiadoGrande) {
		t.Errorf("se esperaba ErrDemasiadoGrande, se obtuvo %v", err)
	}
	if _, err := l.Guardar(context.Background(), dominio.VarianteOriginal, "jpg", strings.NewReader("")); !errors.Is(err, ErrVacio) {
		t.Errorf("se esperaba ErrVacio, se obtuvo %v", err)
	}
	if n := contarArchivos(t, l.Dir()); n != 0 {
		t.Errorf("no deberían quedar archivos temporales, hay %d", n)
	}
}

func TestExtensionNoPermitida(t *testing.T) {
	l := nuevo(t, 1024)
	if _, err := l.Guardar(context.Background(), dominio.VarianteOriginal, "exe", strings.NewReader("x")); !errors.Is(err, ErrRutaInvalida) {
		t.Errorf("se esperaba ErrRutaInvalida, se obtuvo %v", err)
	}
}

func TestRutasManipuladas(t *testing.T) {
	l := nuevo(t, 1024)
	for _, ruta := range []string{
		"", "../secreto.txt", "originales/../../etc/passwd", "/etc/passwd",
		`C:\Windows\win.ini`, "originales/2026/11/nombre.jpg", "originales/2026/11/x.jpg.tmp",
	} {
		if _, err := l.Abrir(ruta); !errors.Is(err, ErrRutaInvalida) {
			t.Errorf("Abrir(%q): se esperaba ErrRutaInvalida, se obtuvo %v", ruta, err)
		}
		if err := l.Eliminar(ruta); !errors.Is(err, ErrRutaInvalida) {
			t.Errorf("Eliminar(%q): se esperaba ErrRutaInvalida, se obtuvo %v", ruta, err)
		}
	}
}
