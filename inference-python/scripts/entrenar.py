"""
Entrena (ajuste fino) el detector de URBIA a partir de un YOLO26 preentrenado
en COCO (HU 05.2).

Uso (desde inference-python/):
    python -m scripts.entrenar                         # usa config/entrenamiento.yaml
    python -m scripts.entrenar --epochs 1 --fraccion 0.05   # prueba rápida (humo)
    python -m scripts.entrenar --modelo yolo26n.pt --device cpu

Resultado: runs/urbia/entrenamiento*/weights/best.pt  (+ gráficas y results.csv)
"""

from __future__ import annotations

import argparse
import json
from datetime import datetime
from pathlib import Path

import yaml

from urbia_vision import clases
from urbia_vision.hardware import detectar, recomendar

RAIZ_SERVICIO = Path(__file__).resolve().parents[1]
CONFIG_POR_DEFECTO = RAIZ_SERVICIO / "config" / "entrenamiento.yaml"

# Claves del YAML que no se pasan tal cual a model.train()
CLAVES_PROPIAS = {"modelo_base"}


def construir_argumentos(config: dict, cli: argparse.Namespace) -> tuple[str, dict, dict]:
    info = detectar()
    rec = recomendar(info)

    modelo = cli.modelo or config.get("modelo_base", "auto")
    if modelo == "auto":
        modelo = rec.modelo_base

    args = {k: v for k, v in config.items() if k not in CLAVES_PROPIAS}
    for clave in ("epochs", "batch", "device", "imgsz", "patience", "name"):
        valor = getattr(cli, clave)
        if valor is not None:
            args[clave] = valor
    if cli.fraccion is not None:
        args["fraction"] = cli.fraccion

    if str(args.get("device", "auto")) == "auto":
        args["device"] = rec.device
    if str(args.get("batch", "auto")) == "auto":
        args["batch"] = rec.batch if args["device"] == rec.device else 8
    if args["device"] == "cpu":
        args["amp"] = False

    # Rutas absolutas para no depender del directorio de trabajo de Ultralytics
    args["data"] = str((RAIZ_SERVICIO / args["data"]).resolve())
    args["project"] = str((RAIZ_SERVICIO / args["project"]).resolve())
    return modelo, args, info.como_dict()


def verificar_dataset(ruta_data: str) -> None:
    ruta = Path(ruta_data)
    if not ruta.exists():
        raise SystemExit(
            f"ERROR: no existe {ruta}. Prepara el dataset primero (HU 05.1):\n"
            "  python -m scripts.preparar_dataset --fuente pothole=... --fuente garbage=..."
        )
    with ruta.open(encoding="utf-8") as f:
        nombres = {int(k): v for k, v in yaml.safe_load(f)["names"].items()}
    if nombres != clases.NOMBRES:
        raise SystemExit(f"ERROR: data.yaml tiene {nombres} y clases.py {clases.NOMBRES}")


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--config", type=Path, default=CONFIG_POR_DEFECTO)
    p.add_argument("--modelo", help="yolo26n.pt, yolo26s.pt, ... (sobrescribe modelo_base)")
    p.add_argument("--epochs", type=int)
    p.add_argument("--batch", type=int)
    p.add_argument("--device", help="cpu, 0, 0,1 ...")
    p.add_argument("--imgsz", type=int)
    p.add_argument("--patience", type=int)
    p.add_argument("--name", help="nombre de la carpeta dentro de runs/urbia")
    p.add_argument("--fraccion", type=float, help="fracción del dataset a usar (prueba rápida)")
    cli = p.parse_args()

    with cli.config.open(encoding="utf-8") as f:
        config = yaml.safe_load(f)

    modelo, args, hardware = construir_argumentos(config, cli)
    verificar_dataset(args["data"])

    print(f"Modelo base: {modelo}")
    print("Argumentos de entrenamiento:")
    for k, v in args.items():
        print(f"  {k:14}: {v}")

    from ultralytics import YOLO  # import tardío: tarda en cargar

    inicio = datetime.now()
    yolo = YOLO(modelo)
    yolo.train(**args)
    fin = datetime.now()

    save_dir = Path(yolo.trainer.save_dir)
    registro = {
        "modelo_base": modelo,
        "argumentos": args,
        "hardware": hardware,
        "inicio": inicio.isoformat(timespec="seconds"),
        "fin": fin.isoformat(timespec="seconds"),
        "duracion_min": round((fin - inicio).total_seconds() / 60, 1),
    }
    (save_dir / "entrenamiento_urbia.json").write_text(
        json.dumps(registro, indent=2, ensure_ascii=False), encoding="utf-8"
    )

    print(f"\nEntrenamiento terminado en {registro['duracion_min']} min")
    print(f"Mejores pesos: {save_dir / 'weights' / 'best.pt'}")
    print("Siguiente paso:")
    print(f"  python -m scripts.evaluar --pesos {save_dir / 'weights' / 'best.pt'}")


if __name__ == "__main__":
    main()