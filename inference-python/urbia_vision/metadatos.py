"""
Metadatos de versión del modelo de URBIA (HU 05.2).

Los pesos (.pt) NO se versionan en git (están en .gitignore). Lo que sí se
versiona es config/modelo.json, que dice exactamente qué archivo de pesos
usa el servicio, con su SHA-256, sus clases, su umbral y sus métricas.
El servicio (05.3) se negará a cargar unos pesos cuyo SHA-256 no coincida.
"""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path

PATRON_SEMVER = re.compile(r"^\d+\.\d+\.\d+$")
CAMPOS_OBLIGATORIOS = (
    "nombre", "version", "archivo", "sha256", "clases",
    "imgsz", "umbral_confianza", "modelo_base",
)


def sha256_archivo(ruta: Path) -> str:
    h = hashlib.sha256()
    with ruta.open("rb") as f:
        for bloque in iter(lambda: f.read(1 << 20), b""):
            h.update(bloque)
    return h.hexdigest()


def validar_metadatos(meta: dict, clases_esperadas: dict[int, str]) -> list[str]:
    errores = [f"falta el campo '{c}'" for c in CAMPOS_OBLIGATORIOS if c not in meta]
    if errores:
        return errores
    if not PATRON_SEMVER.match(str(meta["version"])):
        errores.append(f"version '{meta['version']}' no es X.Y.Z")
    # Límites de la tabla deteccion: modelo VARCHAR(60), version_modelo VARCHAR(20)
    if len(meta["nombre"]) > 60:
        errores.append("nombre excede 60 caracteres (deteccion.modelo)")
    if len(str(meta["version"])) > 20:
        errores.append("version excede 20 caracteres (deteccion.version_modelo)")
    clases = {int(k): v for k, v in meta["clases"].items()}
    if clases != clases_esperadas:
        errores.append(f"las clases {clases} no coinciden con clases.py {clases_esperadas}")
    if not 0 < float(meta["umbral_confianza"]) < 1:
        errores.append("umbral_confianza debe estar entre 0 y 1")
    if not re.fullmatch(r"[0-9a-f]{64}", str(meta["sha256"])):
        errores.append("sha256 inválido")
    return errores


def guardar(meta: dict, ruta: Path) -> None:
    ruta.parent.mkdir(parents=True, exist_ok=True)
    ruta.write_text(json.dumps(meta, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def cargar(ruta: Path) -> dict:
    return json.loads(ruta.read_text(encoding="utf-8"))