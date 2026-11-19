package imagenes

import (
	"encoding/binary"
	"image"
	"image/draw"
)

// orientacionEXIF lee el tag Orientation (0x0112) del segmento APP1 de un JPEG.
// Devuelve 1 (normal) si no hay EXIF o si está mal formado.
func orientacionEXIF(datos []byte) int {
	if len(datos) < 4 || datos[0] != 0xFF || datos[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(datos) {
		if datos[i] != 0xFF {
			return 1
		}
		marcador := datos[i+1]
		switch {
		case marcador == 0xFF: // relleno
			i++
			continue
		case marcador == 0x01 || (marcador >= 0xD0 && marcador <= 0xD7): // sin longitud
			i += 2
			continue
		case marcador == 0xDA || marcador == 0xD9: // inicio de datos o fin: ya no hay EXIF
			return 1
		}
		largo := int(binary.BigEndian.Uint16(datos[i+2:]))
		if largo < 2 || i+2+largo > len(datos) {
			return 1
		}
		seg := datos[i+4 : i+2+largo]
		if marcador == 0xE1 && len(seg) >= 6 && string(seg[:6]) == "Exif\x00\x00" {
			return orientacionTIFF(seg[6:])
		}
		i += 2 + largo
	}
	return 1
}

func orientacionTIFF(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(t[2:]) != 42 {
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for k := 0; k < n; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// aplicarOrientacion gira o refleja la imagen para que se vea derecha una vez
// eliminado el EXIF. o sigue la especificación EXIF (1 = normal ... 8).
func aplicarOrientacion(src image.Image, o int) image.Image {
	if o < 2 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)

	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2: // espejo horizontal
				dx, dy = w-1-x, y
			case 3: // 180°
				dx, dy = w-1-x, h-1-y
			case 4: // espejo vertical
				dx, dy = x, h-1-y
			case 5: // transpuesta
				dx, dy = y, x
			case 6: // 90° horario
				dx, dy = h-1-y, x
			case 7: // transversa
				dx, dy = h-1-y, w-1-x
			case 8: // 90° antihorario
				dx, dy = y, w-1-x
			}
			si, di := rgba.PixOffset(x, y), dst.PixOffset(dx, dy)
			copy(dst.Pix[di:di+4], rgba.Pix[si:si+4])
		}
	}
	return dst
}

// aplanarSobreBlanco quita la transparencia (JPEG no la soporta);
// sin esto, las zonas transparentes de un PNG quedarían negras.
func aplanarSobreBlanco(src image.Image) image.Image {
	if o, ok := src.(interface{ Opaque() bool }); ok && o.Opaque() {
		return src
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}
