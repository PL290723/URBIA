//go:build integracion

package integracion

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/PL290723/URBIA/backend-go/internal/clients"
	"github.com/PL290723/URBIA/backend-go/internal/dominio"
	"github.com/PL290723/URBIA/backend-go/internal/repositories"
)

// Subtarea 2 — Probar Go invocando Python mediante gRPC.
func TestGRPC_GoInvocaPython(t *testing.T) {
	_, info := requierePython(t)
	t.Logf("modelo %s %s (umbral %.2f, dispositivo %s, contrato %s)",
		info.Modelo, info.VersionModelo, info.UmbralConfianza, info.Dispositivo, info.VersionContrato)

	if info.VersionContrato != clients.VersionContrato {
		t.Errorf("Python usa el contrato %s y Go el %s: regenera el código del .proto",
			info.VersionContrato, clients.VersionContrato)
	}
	if info.Modelo == "" || info.VersionModelo == "" {
		t.Error("el servicio no informa nombre y versión del modelo")
	}
	if info.UmbralConfianza <= 0 || info.UmbralConfianza >= 1 {
		t.Errorf("umbral de confianza fuera de (0, 1): %v", info.UmbralConfianza)
	}

	// Cada clase que puede sugerir Python debe existir como categoría activa en PostgreSQL;
	// si no, la sugerencia se perdería (SugerenciaService no encontraría la categoría).
	repo := repositories.NewCategoriaRepository(pool)
	for _, clase := range info.Clases {
		if _, err := repo.BuscarActivaPorNombre(context.Background(), clase); err != nil {
			t.Errorf("la clase %q del modelo no es una categoría activa en la BD: %v", clase, err)
		}
	}
	for _, esperada := range []string{"bache", "basura"} {
		if !slices.Contains(info.Clases, esperada) {
			t.Errorf("el modelo no tiene la clase %q (clases: %v)", esperada, info.Clases)
		}
	}
}

// Subtarea 3 — Probar acceso compartido a imágenes entre Go y Python.
// Go escribe en SU STORAGE_DIR y Python lee la misma ruta relativa en el SUYO:
// si las dos variables no apuntan a la misma carpeta (o al mismo volumen en
// Docker), esta prueba falla con IMAGEN_NO_ENCONTRADA.
func TestAlmacenamiento_CompartidoEntreGoYPython(t *testing.T) {
	cliente, _ := requierePython(t)
	ctx := context.Background()
	foto := fotoSintetica(t)

	g, err := almacen.Guardar(ctx, dominio.VarianteOriginal, "jpg", bytes.NewReader(foto))
	if err != nil {
		t.Fatalf("Go no pudo escribir en STORAGE_DIR (%s): %v", almacen.Dir(), err)
	}
	t.Cleanup(func() { _ = almacen.Eliminar(g.Ruta) })
	t.Logf("Go escribió %s en %s", g.Ruta, almacen.Dir())

	cfgFoto, _, err := image.DecodeConfig(bytes.NewReader(foto))
	if err != nil {
		t.Fatal(err)
	}

	det, err := cliente.Detectar(ctx, clients.SolicitudDeteccion{
		IDSolicitud: idSolicitud("compartido"), IDReporte: 1, IDArchivo: 1, Ruta: g.Ruta,
	})
	if err != nil {
		var ei *clients.ErrorInferencia
		if errors.As(err, &ei) && ei.Codigo == "IMAGEN_NO_ENCONTRADA" {
			t.Fatalf("Python no encuentra la foto que Go acaba de escribir: STORAGE_DIR de Go (%s) "+
				"y de Python no apuntan a la misma carpeta", almacen.Dir())
		}
		t.Fatalf("Python no pudo leer la foto de Go: %v", err)
	}

	// Python leyó exactamente el archivo de Go: mismas dimensiones.
	if det.Ancho != cfgFoto.Width || det.Alto != cfgFoto.Height {
		t.Errorf("Python leyó %dx%d y Go escribió %dx%d", det.Ancho, det.Alto, cfgFoto.Width, cfgFoto.Height)
	}

	// Python solo lee: el archivo sigue ahí y sin cambios.
	contenido, err := os.ReadFile(filepath.Join(almacen.Dir(), filepath.FromSlash(g.Ruta)))
	if err != nil {
		t.Fatalf("la foto desapareció después de la inferencia: %v", err)
	}
	if !bytes.Equal(contenido, foto) {
		t.Error("la foto cambió después de la inferencia: Python debe abrir STORAGE_DIR en solo lectura")
	}
	t.Logf("Python leyó %dx%d: resultado %s en %.1f ms", det.Ancho, det.Alto, det.Resultado, det.Milisegundos)
}
