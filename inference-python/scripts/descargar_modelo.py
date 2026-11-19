"""
Descarga los pesos del modelo desde el GitHub Release y verifica su SHA-256 (HU 05.3).

Los pesos (.pt) no están en git. `config/modelo.json` dice qué archivo usar y
cuál es su SHA-256; este script lo descarga del Release `modelo-v<version>`.

Uso (desde inference-python/):
    python -m scripts.descargar_modelo
    python -m scripts.descargar_modelo --url https://.../urbia-yolo26s-v1.0.0.pt

Si el archivo ya existe y su SHA-256 es correcto, no descarga nada.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.request
from pathlib import Path

from urbia_vision.configuracion import RAIZ_SERVICIO
from urbia_vision.metadatos import sha256_archivo

REPO = "PL290723/URBIA"


def url_release(meta: dict) -> str:
    return (f"https://github.com/{REPO}/releases/download/"
            f"modelo-v{meta['version']}/{Path(meta['archivo']).name}")


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--config", type=Path, default=RAIZ_SERVICIO / "config" / "modelo.json")
    p.add_argument("--url", default=os.environ.get("URBIA_MODELO_URL"))
    args = p.parse_args()

    meta = json.loads(args.config.read_text(encoding="utf-8"))
    destino = RAIZ_SERVICIO / meta["archivo"]

    if destino.is_file() and sha256_archivo(destino) == meta["sha256"]:
        print(f"Ya existe y el SHA-256 es correcto: {destino}")
        return 0

    url = args.url or url_release(meta)
    print(f"Descargando {url}")
    destino.parent.mkdir(parents=True, exist_ok=True)
    temporal = destino.with_suffix(".descarga")
    try:
        with urllib.request.urlopen(url, timeout=60) as resp, temporal.open("wb") as f:
            while bloque := resp.read(1 << 20):
                f.write(bloque)
    except OSError as e:
        temporal.unlink(missing_ok=True)
        print(f"ERROR: no se pudo descargar: {e}")
        return 1

    sha = sha256_archivo(temporal)
    if sha != meta["sha256"]:
        temporal.unlink(missing_ok=True)
        print(f"ERROR: SHA-256 distinto (esperado {meta['sha256']}, descargado {sha}). Se borró la descarga.")
        return 1

    temporal.replace(destino)
    print(f"OK: {destino} ({destino.stat().st_size / 1024**2:.1f} MB), SHA-256 verificado")
    return 0


if __name__ == "__main__":
    sys.exit(main())