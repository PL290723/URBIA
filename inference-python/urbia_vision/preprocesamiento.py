"""
Preprocesamiento estándar de imágenes antes de inferir (HU 05.3).

Toda imagen pasa por los mismos pasos, venga de STORAGE_DIR o de una prueba:

1. Firma real del contenido (magic bytes): solo JPEG, PNG o WebP.
2. Dimensiones leídas de la cabecera, ANTES de decodificar:
   200–8000 px por lado y máximo 25 MP (los mismos límites que Go).
3. Decodificación completa (detecta archivos truncados o corruptos).
4. Orientación EXIF aplicada (Go ya la aplica, pero así el servicio es
   correcto también con imágenes que no pasaron por Go).
5. Conversión a RGB; la transparencia se aplana sobre fondo blanco (igual que Go).
6. Reducción a `lado_max_inferencia` (1280 px) conservando la proporción.
   Ultralytics después la ajusta a 640 (imgsz) con letterbox; reducir antes
   ahorra memoria y tiempo con fotos de celular de 12+ MP.
7. Arreglo NumPy en orden BGR, que es lo que Ultralytics espera de un ndarray.

Se guarda la escala para devolver las cajas en píxeles de la foto ORIGINAL.
"""

from __future__ import annotations

import io
import sys
import warnings
from dataclasses import dataclass

import numpy as np
from PIL import Image, ImageOps

from urbia_vision.errores import ImagenInvalida

FORMATOS_PERMITIDOS = {"JPEG", "PNG", "WEBP"}


@dataclass(frozen=True)
class LimitesImagen:
    lado_min: int = 200
    lado_max: int = 8000
    max_pixeles: int = 25_000_000
    lado_max_inferencia: int = 1280


@dataclass(frozen=True)
class ImagenPreparada:
    arreglo: np.ndarray       # alto x ancho x 3, uint8, BGR
    ancho_original: int
    alto_original: int
    escala: float             # tamaño preparado / tamaño original (<= 1)
    formato: str              # JPEG, PNG o WEBP


def formato_por_firma(datos: bytes) -> str | None:
    if datos[:3] == b"\xff\xd8\xff":
        return "JPEG"
    if datos[:8] == b"\x89PNG\r\n\x1a\n":
        return "PNG"
    if len(datos) >= 12 and datos[:4] == b"RIFF" and datos[8:12] == b"WEBP":
        return "WEBP"
    return None


def _abrir_con_pillow(datos: bytes, formato: str) -> Image.Image:
    """
    Abre la imagen con el `Image.open` ORIGINAL de Pillow, solo para el formato
    de la firma. Ultralytics reemplaza `PIL.Image.open` al importarse por una
    versión que, si no reconoce el archivo, intenta instalar `pi-heif` con pip.
    """
    abrir = Image.open
    parches = sys.modules.get("ultralytics.utils.patches")
    if parches is not None and getattr(abrir, "__module__", "") == parches.__name__:
        abrir = getattr(parches, "_image_open", abrir)
    return abrir(io.BytesIO(datos), formats=[formato])


def preparar(datos: bytes, limites: LimitesImagen = LimitesImagen()) -> ImagenPreparada:
    firma = formato_por_firma(datos)
    if firma is None:
        raise ImagenInvalida("Formato no permitido: solo JPEG, PNG o WebP")

    with warnings.catch_warnings():
        # Pillow avisa con DecompressionBombWarning; aquí se valida a mano
        warnings.simplefilter("ignore", Image.DecompressionBombWarning)
        try:
            imagen = _abrir_con_pillow(datos, firma)
        except Exception:  # UnidentifiedImageError, OSError, ValueError, …
            raise ImagenInvalida("La cabecera de la imagen está dañada") from None

        if imagen.format != firma:
            raise ImagenInvalida("El contenido no coincide con el formato de la imagen")

        ancho, alto = imagen.size
        if not (limites.lado_min <= ancho <= limites.lado_max
                and limites.lado_min <= alto <= limites.lado_max):
            raise ImagenInvalida(
                f"La imagen mide {ancho}x{alto} px; cada lado debe estar entre "
                f"{limites.lado_min} y {limites.lado_max} px"
            )
        if ancho * alto > limites.max_pixeles:
            raise ImagenInvalida(f"La imagen supera {limites.max_pixeles // 1_000_000} MP")

        try:
            imagen.load()  # decodificación completa
        except Exception:  # OSError, ValueError, DecompressionBombError, …
            raise ImagenInvalida("La imagen está truncada o dañada") from None

    imagen = ImageOps.exif_transpose(imagen)
    imagen = _a_rgb(imagen)

    ancho_o, alto_o = imagen.size
    escala = min(1.0, limites.lado_max_inferencia / max(ancho_o, alto_o))
    if escala < 1.0:
        nuevo = (max(1, round(ancho_o * escala)), max(1, round(alto_o * escala)))
        imagen = imagen.resize(nuevo, Image.Resampling.LANCZOS)

    rgb = np.asarray(imagen, dtype=np.uint8)
    bgr = np.ascontiguousarray(rgb[:, :, ::-1])
    return ImagenPreparada(bgr, ancho_o, alto_o, escala, firma)


def _a_rgb(imagen: Image.Image) -> Image.Image:
    if imagen.mode in ("RGBA", "LA") or (imagen.mode == "P" and "transparency" in imagen.info):
        con_alfa = imagen.convert("RGBA")
        fondo = Image.new("RGBA", con_alfa.size, (255, 255, 255, 255))
        return Image.alpha_composite(fondo, con_alfa).convert("RGB")
    return imagen.convert("RGB")