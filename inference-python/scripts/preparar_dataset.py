"""
Prepara el dataset de URBIA a partir de exports YOLOv8 de Roboflow (HU 05.1).

Uso (desde inference-python/):

    python -m scripts.preparar_dataset ^
        --fuente pothole=datasets/fuentes/pothole ^
        --fuente garbage=datasets/fuentes/garbage

Resultado:
    datasets/urbia/images/{train,val,test}
    datasets/urbia/labels/{train,val,test}
    datasets/urbia/data.yaml              (para Ultralytics, se usa en 05.2)
    config/manifiesto_split.csv           (se versiona en git)
"""

from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path

import yaml

from urbia_vision import clases
from urbia_vision.dataset import preparar

RAIZ_SERVICIO = Path(__file__).resolve().parents[1]


def leer_nombres_fuente(ruta_export: Path) -> list[str]:
    """Lee la lista de clases del data.yaml que viene en el export."""
    data_yaml = ruta_export / "data.yaml"
    if not data_yaml.exists():
        sys.exit(f"ERROR: no encontré {data_yaml}. Descarga el dataset en formato YOLOv8.")
    with data_yaml.open(encoding="utf-8") as f:
        datos = yaml.safe_load(f)
    nombres = datos["names"]
    if isinstance(nombres, dict):  # formato {0: 'a', 1: 'b'}
        nombres = [nombres[k] for k in sorted(nombres)]
    return list(nombres)


def escribir_data_yaml(destino: Path) -> Path:
    ruta = destino / "data.yaml"
    contenido = {
        "path": str(destino.resolve()),
        "train": "images/train",
        "val": "images/val",
        "test": "images/test",
        "names": dict(clases.NOMBRES),
    }
    with ruta.open("w", encoding="utf-8") as f:
        yaml.safe_dump(contenido, f, allow_unicode=True, sort_keys=False)
    return ruta


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--fuente", action="append", required=True, metavar="NOMBRE=RUTA",
                   help=f"Fuente a incluir. NOMBRE debe ser uno de {sorted(clases.MAPEO_FUENTES)}")
    p.add_argument("--destino", type=Path, default=RAIZ_SERVICIO / "datasets" / "urbia")
    p.add_argument("--manifiesto", type=Path, default=RAIZ_SERVICIO / "config" / "manifiesto_split.csv")
    p.add_argument("--semilla", type=int, default=42)
    p.add_argument("--proporciones", type=float, nargs=3, default=(0.7, 0.2, 0.1),
                   metavar=("TRAIN", "VAL", "TEST"))
    p.add_argument("--limpiar", action="store_true", help="Borra el destino antes de preparar")
    args = p.parse_args()

    clases.validar_definicion()

    fuentes = {}
    for item in args.fuente:
        if "=" not in item:
            sys.exit(f"ERROR: '{item}' debe tener la forma NOMBRE=RUTA")
        nombre, ruta = item.split("=", 1)
        if nombre not in clases.MAPEO_FUENTES:
            sys.exit(f"ERROR: fuente '{nombre}' desconocida. Usa una de {sorted(clases.MAPEO_FUENTES)}")
        ruta = Path(ruta)
        fuentes[nombre] = (ruta, leer_nombres_fuente(ruta), clases.MAPEO_FUENTES[nombre])

    if args.limpiar and args.destino.exists():
        shutil.rmtree(args.destino)

    resumen = preparar(
        fuentes=fuentes,
        destino=args.destino,
        ids_destino=clases.IDS,
        manifiesto=args.manifiesto,
        proporciones=tuple(args.proporciones),
        semilla=args.semilla,
    )
    data_yaml = escribir_data_yaml(args.destino)

    print(f"Imágenes preparadas: {resumen.imagenes}")
    for split in ("train", "val", "test"):
        print(f"  {split:5}: {resumen.por_split[split]}")
    print("Cajas por clase:")
    for id_clase, nombre in clases.NOMBRES.items():
        print(f"  {id_clase} {nombre:8}: {resumen.cajas_por_clase[id_clase]}")
    print(f"Cajas descartadas (clases fuera del MVP): {resumen.cajas_descartadas}")
    print(f"data.yaml:   {data_yaml}")
    print(f"Manifiesto:  {args.manifiesto}")


if __name__ == "__main__":
    main()