// Package storage guarda las fotografías en disco, fuera de PostgreSQL.
// En la base solo quedan los metadatos y la ruta relativa (tabla archivo).
package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

const (
	permDir     = 0o750 // dueño rwx, grupo rx, otros nada
	permArchivo = 0o640 // dueño rw, grupo r, otros nada
)

var (
	ErrNoEncontrado    = errors.New("archivo no encontrado en el almacenamiento")
	ErrRutaInvalida    = errors.New("ruta de archivo inválida")
	ErrVacio           = errors.New("el archivo está vacío")
	ErrDemasiadoGrande = errors.New("el archivo excede el tamaño máximo")
)

// patronRuta solo acepta rutas generadas por este paquete.
// Es la misma regla que ck_archivo_ruta en la base de datos.
var patronRuta = regexp.MustCompile(`^(originales|procesadas)/[0-9]{4}/[0-9]{2}/[a-z2-7]{26}\.(jpg|png|webp)$`)

// Guardado es el resultado de escribir un archivo.
type Guardado struct {
	Ruta   string // relativa a STORAGE_DIR, siempre con "/": originales/2026/11/<aleatorio>.jpg
	Tamano int64
	SHA256 string
}

// Local guarda archivos en un directorio. Todas las operaciones pasan por os.Root,
// así que ninguna ruta puede salir de ese directorio (ni con "../" ni con symlinks).
type Local struct {
	dir    string
	root   *os.Root
	maximo int64
}

// NewLocal crea (si no existen) dir/originales y dir/procesadas y comprueba que se pueda escribir.
func NewLocal(dir string, maximo int64) (*Local, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("STORAGE_DIR inválido: %w", err)
	}
	for _, v := range []dominio.VarianteArchivo{dominio.VarianteOriginal, dominio.VarianteProcesada} {
		if err := os.MkdirAll(filepath.Join(abs, v.Carpeta()), permDir); err != nil {
			return nil, fmt.Errorf("no se pudo crear el almacenamiento en %s: %w", abs, err)
		}
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el almacenamiento %s: %w", abs, err)
	}
	l := &Local{dir: abs, root: root, maximo: maximo}
	if err := l.comprobarEscritura(); err != nil {
		_ = root.Close()
		return nil, err
	}
	return l, nil
}

func (l *Local) Dir() string  { return l.dir }
func (l *Local) Close() error { return l.root.Close() }

// Guardar escribe el contenido con un nombre aleatorio. Escribe primero un .tmp y
// luego lo renombra, para que nunca quede a la vista un archivo a medias.
func (l *Local) Guardar(ctx context.Context, v dominio.VarianteArchivo, ext string, src io.Reader) (*Guardado, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	carpeta := v.Carpeta()
	if carpeta == "" {
		return nil, fmt.Errorf("variante inválida: %q", v)
	}

	ahora := time.Now().UTC()
	dirRel := path.Join(carpeta, ahora.Format("2006"), ahora.Format("01"))
	ruta := path.Join(dirRel, strings.ToLower(rand.Text())+"."+ext)
	if !patronRuta.MatchString(ruta) { // también rechaza extensiones fuera de la lista
		return nil, ErrRutaInvalida
	}
	if err := l.root.MkdirAll(dirRel, permDir); err != nil {
		return nil, fmt.Errorf("crear carpeta %s: %w", dirRel, err)
	}

	tmp := ruta + ".tmp"
	f, err := l.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permArchivo)
	if err != nil {
		return nil, fmt.Errorf("crear archivo temporal: %w", err)
	}
	listo := false
	defer func() {
		if !listo { // ante cualquier error no queda basura en disco
			_ = f.Close()
			_ = l.root.Remove(tmp)
		}
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(lectorConContexto{ctx, src}, l.maximo+1))
	if err != nil {
		return nil, fmt.Errorf("escribir archivo: %w", err)
	}
	if n == 0 {
		return nil, ErrVacio
	}
	if n > l.maximo {
		return nil, ErrDemasiadoGrande
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("sincronizar archivo: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("cerrar archivo: %w", err)
	}
	if err := l.root.Rename(tmp, ruta); err != nil {
		return nil, fmt.Errorf("publicar archivo: %w", err)
	}
	listo = true
	return &Guardado{Ruta: ruta, Tamano: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// Abrir devuelve el archivo para lectura. Quien llama debe cerrarlo.
func (l *Local) Abrir(ruta string) (*os.File, error) {
	if !patronRuta.MatchString(ruta) {
		return nil, ErrRutaInvalida
	}
	f, err := l.root.Open(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("abrir archivo: %w", err)
	}
	return f, nil
}

// Eliminar borra el archivo. Si ya no existe no es error (operación idempotente).
func (l *Local) Eliminar(ruta string) error {
	if !patronRuta.MatchString(ruta) {
		return ErrRutaInvalida
	}
	if err := l.root.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("eliminar archivo: %w", err)
	}
	return nil
}

func (l *Local) comprobarEscritura() error {
	nombre := ".prueba-" + strings.ToLower(rand.Text())
	f, err := l.root.OpenFile(nombre, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permArchivo)
	if err != nil {
		return fmt.Errorf("el almacenamiento %s no permite escritura: %w", l.dir, err)
	}
	_ = f.Close()
	return l.root.Remove(nombre)
}

// lectorConContexto corta la copia si el cliente cancela la petición.
type lectorConContexto struct {
	ctx context.Context
	r   io.Reader
}

func (l lectorConContexto) Read(p []byte) (int, error) {
	if err := l.ctx.Err(); err != nil {
		return 0, err
	}
	return l.r.Read(p)
}
