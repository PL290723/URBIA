package imagenes

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

var limPrueba = Limites{MaxBytes: 10 << 20, MinLado: 200, MaxLado: 8000, MaxPixeles: 25_000_000}

func sanear(t *testing.T, lim Limites, datos []byte) (*Resultado, error) {
	t.Helper()
	return NewSaneador(lim, 1).Sanear(context.Background(), bytes.NewReader(datos))
}

func imagenRelleno(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

func jpegDePrueba(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, imagenRelleno(w, h, color.RGBA{200, 50, 50, 255}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pngDePrueba(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// conEXIF inserta un APP1 con Orientation y un texto extra (simula GPS u otros datos).
func conEXIF(jpg []byte, orientacion byte, extra string) []byte {
	tiff := []byte{'I', 'I', 0x2A, 0x00, 0x08, 0, 0, 0,
		0x01, 0x00, // 1 entrada
		0x12, 0x01, 0x03, 0x00, 0x01, 0, 0, 0, orientacion, 0, 0, 0,
		0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	payload = append(payload, extra...)
	largo := len(payload) + 2
	out := append([]byte{}, jpg[:2]...)
	out = append(out, 0xFF, 0xE1, byte(largo>>8), byte(largo))
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

func TestJPEGValido(t *testing.T) {
	r, err := sanear(t, limPrueba, jpegDePrueba(t, 400, 300))
	if err != nil {
		t.Fatal(err)
	}
	if r.MIME != "image/jpeg" || r.Ancho != 400 || r.Alto != 300 {
		t.Fatalf("resultado inesperado: %+v", r)
	}
}

func TestEliminaEXIFyRespetaOrientacion(t *testing.T) {
	datos := conEXIF(jpegDePrueba(t, 400, 300), 6, "GPS-SECRETO-20.5420,-100.8157")
	if orientacionEXIF(datos) != 6 {
		t.Fatal("no se leyó la orientación del EXIF de prueba")
	}
	r, err := sanear(t, limPrueba, datos)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(r.Contenido, []byte("Exif")) || bytes.Contains(r.Contenido, []byte("GPS-SECRETO")) {
		t.Error("la imagen saneada no debe conservar EXIF ni datos sensibles")
	}
	if r.Ancho != 300 || r.Alto != 400 {
		t.Errorf("con orientación 6 se esperaba 300x400, se obtuvo %dx%d", r.Ancho, r.Alto)
	}
}

func TestPNGTransparenteQuedaBlanco(t *testing.T) {
	r, err := sanear(t, limPrueba, pngDePrueba(t, image.NewNRGBA(image.Rect(0, 0, 300, 300))))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(r.Contenido))
	if err != nil {
		t.Fatal(err)
	}
	rr, gg, bb, _ := img.At(10, 10).RGBA()
	if rr>>8 < 240 || gg>>8 < 240 || bb>>8 < 240 {
		t.Errorf("la transparencia debería quedar blanca, quedó (%d,%d,%d)", rr>>8, gg>>8, bb>>8)
	}
}

func TestFormatosNoPermitidos(t *testing.T) {
	var g bytes.Buffer
	pal := image.NewPaletted(image.Rect(0, 0, 300, 300), color.Palette{color.White, color.Black})
	if err := gif.Encode(&g, pal, nil); err != nil {
		t.Fatal(err)
	}
	casos := map[string][]byte{
		"texto con extensión .jpg": []byte("esto no es una imagen"),
		"gif":                      g.Bytes(),
		"html":                     []byte("<html><script>alert(1)</script></html>"),
	}
	for nombre, datos := range casos {
		if _, err := sanear(t, limPrueba, datos); !errors.Is(err, ErrFormatoNoPermitido) {
			t.Errorf("%s: se esperaba ErrFormatoNoPermitido, se obtuvo %v", nombre, err)
		}
	}
}

func TestImagenesDanadas(t *testing.T) {
	jpg := jpegDePrueba(t, 400, 300)
	truncado := jpg[:len(jpg)/2]

	p := pngDePrueba(t, imagenRelleno(300, 300, color.RGBA{10, 120, 10, 255}))
	alterado := append([]byte{}, p...)
	if i := bytes.Index(alterado, []byte("IDAT")); i > 0 && i+10 < len(alterado) {
		alterado[i+10] ^= 0xFF // rompe datos y CRC
	}

	for nombre, datos := range map[string][]byte{"jpeg truncado": truncado, "png alterado": alterado} {
		if _, err := sanear(t, limPrueba, datos); !errors.Is(err, ErrImagenDanada) {
			t.Errorf("%s: se esperaba ErrImagenDanada, se obtuvo %v", nombre, err)
		}
	}
}

func TestDimensiones(t *testing.T) {
	if _, err := sanear(t, limPrueba, jpegDePrueba(t, 50, 50)); !errors.Is(err, ErrDimensiones) {
		t.Errorf("imagen muy chica: se esperaba ErrDimensiones, se obtuvo %v", err)
	}
	lim := limPrueba
	lim.MaxLado = 500
	if _, err := sanear(t, lim, jpegDePrueba(t, 600, 400)); !errors.Is(err, ErrDimensiones) {
		t.Errorf("lado excedido: se esperaba ErrDimensiones, se obtuvo %v", err)
	}
	lim = limPrueba
	lim.MaxPixeles = 100_000
	if _, err := sanear(t, lim, jpegDePrueba(t, 400, 300)); !errors.Is(err, ErrDimensiones) {
		t.Errorf("megapíxeles excedidos: se esperaba ErrDimensiones, se obtuvo %v", err)
	}
}

func TestTamanoYVacio(t *testing.T) {
	lim := limPrueba
	lim.MaxBytes = 100
	if _, err := sanear(t, lim, jpegDePrueba(t, 400, 300)); !errors.Is(err, ErrDemasiadoGrande) {
		t.Errorf("se esperaba ErrDemasiadoGrande, se obtuvo %v", err)
	}
	if _, err := sanear(t, limPrueba, nil); !errors.Is(err, ErrVacio) {
		t.Errorf("se esperaba ErrVacio, se obtuvo %v", err)
	}
}
