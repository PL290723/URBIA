"""
Lectura segura de fotos desde STORAGE_DIR (HU 05.3).

El servicio de Python NO recibe la imagen por la red: Go (HU 04.1) ya la guardó
saneada en STORAGE_DIR y solo le pasa la ruta relativa. Por eso aquí se valida
la ruta con las mismas reglas que Go antes de abrir nada:

1. La ruta debe cumplir EXACTAMENTE el patrón que genera Go
   (`originales|procesadas/AAAA/MM/<26 caracteres base32>.jpg|png|webp`),
   el mismo de `ck_archivo_ruta` en la base de datos. Eso descarta `..`,
   rutas absolutas, `\\`, unidades de Windows y cualquier nombre inventado.
2. Ya resuelta (siguiendo symlinks o junctions), la ruta debe quedar DENTRO
   de STORAGE_DIR.
3. Debe ser un archivo regular y pesar como máximo `max_bytes`.

El servicio solo LEE: nunca escribe, mueve ni borra fotos.
"""

from __future__ import annotations

import re
from pathlib import Path

from urbia_vision.errores import ImagenInvalida, ImagenNoEncontrada, RutaInvalida

# Igual que patronRuta en backend-go/internal/storage/local.go
PATRON_RUTA = re.compile(r"^(originales|procesadas)/[0-9]{4}/[0-9]{2}/[a-z2-7]{26}\.(jpg|png|webp)$")


class LectorImagenes:
    def __init__(self, storage_dir: Path, max_bytes: int):
        self.raiz = Path(storage_dir).resolve()
        self.max_bytes = max_bytes

    def resolver(self, ruta_relativa: str) -> Path:
        """Valida la ruta y devuelve la ruta absoluta del archivo."""
        if not isinstance(ruta_relativa, str) or not PATRON_RUTA.fullmatch(ruta_relativa):
            raise RutaInvalida("La ruta no tiene el formato de una foto guardada por URBIA")

        candidata = self.raiz.joinpath(*ruta_relativa.split("/"))
        try:
            real = candidata.resolve(strict=True)
        except FileNotFoundError:
            raise ImagenNoEncontrada("La imagen no existe en el almacenamiento") from None
        except OSError:
            raise ImagenNoEncontrada("La imagen no se pudo abrir") from None

        if not real.is_relative_to(self.raiz):
            # Un symlink o junction que apunta fuera de STORAGE_DIR
            raise RutaInvalida("La ruta sale del directorio de almacenamiento")
        if not real.is_file():
            raise ImagenNoEncontrada("La ruta no corresponde a un archivo")
        return real

    def leer(self, ruta_relativa: str) -> bytes:
        real = self.resolver(ruta_relativa)
        try:
            with real.open("rb") as f:
                datos = f.read(self.max_bytes + 1)  # nunca se lee más del límite
        except OSError:
            raise ImagenNoEncontrada("La imagen no se pudo leer") from None
        if len(datos) > self.max_bytes:
            raise ImagenInvalida(f"La imagen supera {self.max_bytes // (1024 * 1024)} MiB")
        if not datos:
            raise ImagenNoEncontrada("La imagen está vacía")
        return datos