// Package imagenes valida y sanea las fotografías antes de guardarlas:
// formato real, tamaño, dimensiones, integridad y eliminación de metadatos.
package imagenes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // registra el decodificador PNG
	"io"
	"net/http"

	_ "golang.org/x/image/webp" // registra el decodificador WebP

	"github.com/PL290723/URBIA/backend-go/internal/dominio"
)

const calidadJPEG = 90

var (
	ErrVacio              = errors.New("archivo vacío")
	ErrDemasiadoGrande    = errors.New("archivo demasiado grande")
	ErrFormatoNoPermitido = errors.New("formato no permitido")
	ErrImagenDanada       = errors.New("imagen dañada o ilegible")
	ErrDimensiones        = errors.New("dimensiones fuera de rango")
)

// decodificadorEsperado: formato que debe reportar el decodificador para cada firma.
// Si no coinciden, el contenido es inconsistente (archivo manipulado o políglota).
var decodificadorEsperado = map[string]string{
	"image/jpeg": "jpeg",
	"image/png":  "png",
	"image/webp": "webp",
}

type Limites struct {
	MaxBytes   int64
	MinLado    int
	MaxLado    int
	MaxPixeles int
}

func LimitesPorDefecto() Limites {
	return Limites{
		MaxBytes:   dominio.TamanoMaximoArchivo,
		MinLado:    dominio.LadoMinimoImagen,
		MaxLado:    dominio.LadoMaximoImagen,
		MaxPixeles: dominio.MegapixelesMaximos * 1_000_000,
	}
}

// Resultado es la imagen ya saneada, lista para guardarse.
type Resultado struct {
	Contenido       []byte
	MIME            string
	Ancho, Alto     int
	FormatoRecibido string
}

// Saneador limita cuántas imágenes se decodifican a la vez: una foto de
// 25 MP ocupa ~100 MB en memoria mientras se procesa.
type Saneador struct {
	lim Limites
	sem chan struct{}
}

func NewSaneador(lim Limites, concurrencia int) *Saneador {
	if concurrencia < 1 {
		concurrencia = 1
	}
	return &Saneador{lim: lim, sem: make(chan struct{}, concurrencia)}
}

func (s *Saneador) Limites() Limites { return s.lim }

// Sanear valida la imagen y la devuelve re-codificada como JPEG, sin metadatos.
func (s *Saneador) Sanear(ctx context.Context, r io.Reader) (*Resultado, error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	// 1) Tamaño en bytes
	datos, err := io.ReadAll(io.LimitReader(r, s.lim.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("leer archivo: %w", err)
	}
	if len(datos) == 0 {
		return nil, ErrVacio
	}
	if int64(len(datos)) > s.lim.MaxBytes {
		return nil, ErrDemasiadoGrande
	}

	// 2) Formato real por firma (magic bytes), no por extensión ni Content-Type del cliente
	mime := http.DetectContentType(datos)
	esperado, ok := decodificadorEsperado[mime]
	if _, permitido := dominio.ExtensionPorMIME(mime); !ok || !permitido {
		return nil, fmt.Errorf("%w: %s", ErrFormatoNoPermitido, mime)
	}

	// 3) Cabecera: formato coherente y dimensiones ANTES de decodificar (evita bombas de descompresión)
	cfg, formato, err := image.DecodeConfig(bytes.NewReader(datos))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImagenDanada, err)
	}
	if formato != esperado {
		return nil, fmt.Errorf("%w: firma %s pero contenido %s", ErrImagenDanada, mime, formato)
	}
	if err := s.validarDimensiones(cfg.Width, cfg.Height); err != nil {
		return nil, err
	}

	// 4) Decodificación completa: detecta archivos truncados o corruptos
	img, _, err := image.Decode(bytes.NewReader(datos))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImagenDanada, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 5) Saneamiento: orientación EXIF, transparencia y re-codificación sin metadatos
	if formato == "jpeg" {
		img = aplicarOrientacion(img, orientacionEXIF(datos))
	}
	img = aplanarSobreBlanco(img)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: calidadJPEG}); err != nil {
		return nil, fmt.Errorf("re-codificar imagen: %w", err)
	}
	b := img.Bounds()
	return &Resultado{
		Contenido:       buf.Bytes(),
		MIME:            dominio.MIMEImagenNormalizada,
		Ancho:           b.Dx(),
		Alto:            b.Dy(),
		FormatoRecibido: formato,
	}, nil
}

func (s *Saneador) validarDimensiones(w, h int) error {
	if w < s.lim.MinLado || h < s.lim.MinLado ||
		w > s.lim.MaxLado || h > s.lim.MaxLado || w*h > s.lim.MaxPixeles {
		return fmt.Errorf("%w: %dx%d", ErrDimensiones, w, h)
	}
	return nil
}
