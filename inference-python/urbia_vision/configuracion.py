"""
Configuración del servicio de inferencia (HU 05.3).

Todo se lee de variables de entorno, con valores por defecto pensados para
desarrollo local (Go en backend-go/ con STORAGE_DIR=./storage y Python en
inference-python/). En Docker se sobrescriben.

    Variable                     Por defecto                     Para qué
    URBIA_MODELO_CONFIG          config/modelo.json              metadatos del modelo (05.2)
    STORAGE_DIR                  ../backend-go/storage           fotos guardadas por Go (04.1)
    URBIA_DEVICE                 auto                            auto, cpu, 0 (primera GPU)
    URBIA_MAX_BYTES              10485760 (10 MiB)               igual que Go
    URBIA_LADO_MAX_INFERENCIA    1280                            se reduce la foto antes de inferir
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

RAIZ_SERVICIO = Path(__file__).resolve().parents[1]


def blindar_ultralytics() -> None:
    """
    Ultralytics, por defecto, instala paquetes con pip cuando le falta algo
    (por ejemplo `pi-heif` si recibe un archivo que Pillow no reconoce) y consulta
    internet al importarse. Un servicio en producción no debe hacer ninguna de
    las dos cosas. Debe llamarse ANTES de importar ultralytics; respeta valores
    que ya vengan definidos en el entorno.
    """
    os.environ.setdefault("YOLO_AUTOINSTALL", "False")
    os.environ.setdefault("YOLO_OFFLINE", "True")


# Mismos límites que backend-go/internal/imagenes (docs/backend/validacion-imagenes.md)
MAX_BYTES = 10 * 1024 * 1024
LADO_MIN = 200
LADO_MAX = 8000
MAX_PIXELES = 25_000_000


@dataclass(frozen=True)
class Configuracion:
    modelo_config: Path
    storage_dir: Path
    device: str = "auto"
    max_bytes: int = MAX_BYTES
    lado_min: int = LADO_MIN
    lado_max: int = LADO_MAX
    max_pixeles: int = MAX_PIXELES
    lado_max_inferencia: int = 1280

    @classmethod
    def desde_entorno(cls) -> "Configuracion":
        def ruta(variable: str, por_defecto: Path) -> Path:
            valor = os.environ.get(variable)
            p = Path(valor) if valor else por_defecto
            return p if p.is_absolute() else (RAIZ_SERVICIO / p)

        def entero(variable: str, por_defecto: int) -> int:
            valor = os.environ.get(variable)
            if not valor:
                return por_defecto
            try:
                n = int(valor)
            except ValueError:
                raise ValueError(f"{variable} debe ser un entero, se recibió {valor!r}") from None
            if n <= 0:
                raise ValueError(f"{variable} debe ser mayor que 0, se recibió {n}")
            return n

        return cls(
            modelo_config=ruta("URBIA_MODELO_CONFIG", Path("config/modelo.json")),
            storage_dir=ruta("STORAGE_DIR", Path("../backend-go/storage")),
            device=os.environ.get("URBIA_DEVICE", "auto").strip() or "auto",
            max_bytes=entero("URBIA_MAX_BYTES", MAX_BYTES),
            lado_max_inferencia=entero("URBIA_LADO_MAX_INFERENCIA", 1280),
        )