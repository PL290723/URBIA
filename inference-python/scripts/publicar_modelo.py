"""
Guarda los pesos elegidos con un nombre versionado y genera config/modelo.json
con sus metadatos (HU 05.2).

Uso (desde inference-python/):
    python -m scripts.publicar_modelo ^
        --pesos runs/urbia/entrenamiento/weights/best.pt ^
        --metricas runs/urbia/entrenamiento/evaluacion_test/metricas.json ^
        --version 1.0.0

Resultado:
    models/urbia-yolo26s-v1.0.0.pt   (NO se sube a git; se sube como Release)
    config/modelo.json               (SÍ se sube a git)
"""

from __future__ import annotations

import argparse
import csv
import json
import platform
import shutil
from datetime import datetime
from pathlib import Path

from urbia_vision import clases
from urbia_vision import metadatos as md

RAIZ_SERVICIO = Path(__file__).resolve().parents[1]


def leer_historial(pesos: Path) -> dict:
    """
    Lee runs/.../results.csv. Ultralytics guarda como best.pt la época con
    mayor mAP50-95 en val, así que esa es la "época mejor".
    """
    ruta = pesos.parent.parent / "results.csv"
    if not ruta.exists():
        return {"epochs_entrenadas": None, "epoch_mejor": None}
    with ruta.open(encoding="utf-8") as f:
        filas = [{k.strip(): v for k, v in fila.items()} for fila in csv.DictReader(f)]
    mejor = max(filas, key=lambda r: float(r["metrics/mAP50-95(B)"]))
    return {
        "epochs_entrenadas": len(filas),
        "epoch_mejor": int(float(mejor["epoch"])),
        "val_mAP50_95_mejor": round(float(mejor["metrics/mAP50-95(B)"]), 4),
    }


def leer_checkpoint(pesos: Path) -> dict:
    import torch

    ckpt = torch.load(pesos, map_location="cpu", weights_only=False)
    args = ckpt.get("train_args", {}) or {}
    return {
        "fecha": ckpt.get("date"),
        "ultralytics": ckpt.get("version"),
        "modelo_base": Path(str(args.get("model", "desconocido"))).name,
        "imgsz": args.get("imgsz", 640),
        "epochs_configuradas": args.get("epochs"),
        "batch": args.get("batch"),
        "seed": args.get("seed"),
        "device": args.get("device"),
    }


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--pesos", type=Path, required=True)
    p.add_argument("--metricas", type=Path, required=True, help="metricas.json de scripts.evaluar")
    p.add_argument("--version", required=True, help="versión semántica, por ejemplo 1.0.0")
    p.add_argument("--destino", type=Path, default=RAIZ_SERVICIO / "models")
    p.add_argument("--salida", type=Path, default=RAIZ_SERVICIO / "config" / "modelo.json")
    args = p.parse_args()

    import torch
    from ultralytics import YOLO

    nombres_modelo = {int(k): v for k, v in YOLO(str(args.pesos)).names.items()}
    if nombres_modelo != clases.NOMBRES:
        raise SystemExit(f"ERROR: el modelo tiene clases {nombres_modelo}, se esperaba {clases.NOMBRES}")

    ckpt = leer_checkpoint(args.pesos)
    historial = leer_historial(args.pesos)
    metricas = json.loads(args.metricas.read_text(encoding="utf-8"))
    base = ckpt["modelo_base"].removesuffix(".pt")  # yolo26s
    nombre = f"urbia-{base}"
    archivo = args.destino / f"{nombre}-v{args.version}.pt"

    args.destino.mkdir(parents=True, exist_ok=True)
    shutil.copy2(args.pesos, archivo)

    manifiesto = RAIZ_SERVICIO / "config" / "manifiesto_split.csv"
    meta = {
        "nombre": nombre,
        "version": args.version,
        "archivo": archivo.relative_to(RAIZ_SERVICIO).as_posix(),
        "sha256": md.sha256_archivo(archivo),
        "tamano_mb": round(archivo.stat().st_size / 1024**2, 2),
        "tarea": "detect",
        "modelo_base": ckpt["modelo_base"],
        "clases": {str(k): v for k, v in clases.NOMBRES.items()},
        "imgsz": ckpt["imgsz"],
        "umbral_confianza": metricas["umbral_elegido"]["umbral"],
        "criterio_umbral": metricas["umbral_elegido"]["criterio"],
        "comportamiento_sin_deteccion": (
            "Si ninguna caja supera umbral_confianza el resultado es 'no_detectado' "
            "(clase y confianza NULL) y el reporte queda sin categoría sugerida."
        ),
        "dataset": {
            "manifiesto": "config/manifiesto_split.csv",
            "sha256_manifiesto": md.sha256_archivo(manifiesto) if manifiesto.exists() else None,
            "split_evaluacion": metricas["split"],
            "imagenes_evaluacion": metricas["imagenes"],
        },
        "entrenamiento": {
            "epochs_configuradas": ckpt["epochs_configuradas"],
            **historial,
            "batch": ckpt["batch"],
            "seed": ckpt["seed"],
            "device": ckpt["device"],
            "fecha": ckpt["fecha"],
        },
        "entorno": {
            "ultralytics": ckpt["ultralytics"],
            "torch": torch.__version__,
            "python": platform.python_version(),
        },
        "metricas_test": {
            "ultralytics": metricas["ultralytics"],
            "al_umbral": {k: v for k, v in metricas["umbral_elegido"]["por_clase"].items()},
            "f1_macro": metricas["umbral_elegido"]["f1_macro"],
            "exactitud_por_foto": metricas["por_imagen"]["exactitud"],
        },
        "licencia": "AGPL-3.0 (modelo entrenado con Ultralytics YOLO)",
        "publicado": datetime.now().isoformat(timespec="seconds"),
    }

    errores = md.validar_metadatos(meta, clases.NOMBRES)
    if errores:
        archivo.unlink(missing_ok=True)
        raise SystemExit("ERROR en metadatos:\n  - " + "\n  - ".join(errores))

    md.guardar(meta, args.salida)
    print(f"Pesos:      {archivo}  ({meta['tamano_mb']} MB)")
    print(f"SHA-256:    {meta['sha256']}")
    print(f"Metadatos:  {args.salida}")
    print(f"Umbral:     {meta['umbral_confianza']}")


if __name__ == "__main__":
    main()