"""
Valida un dataset de URBIA ya preparado (HU 05.1).

Revisa: formato YOLO de cada etiqueta, ids de clase válidos, imágenes sin
etiqueta, etiquetas sin imagen, fugas entre splits y que el conjunto de
prueba coincida con el manifiesto versionado.

Uso (desde inference-python/):
    python -m scripts.validar_dataset
    python -m scripts.validar_dataset --raiz datasets/urbia --sin-manifiesto

Código de salida 0 = válido, 1 = hay errores.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from urbia_vision import clases
from urbia_vision.dataset import SPLITS, validar_dataset

RAIZ_SERVICIO = Path(__file__).resolve().parents[1]


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--raiz", type=Path, default=RAIZ_SERVICIO / "datasets" / "urbia")
    p.add_argument("--manifiesto", type=Path, default=RAIZ_SERVICIO / "config" / "manifiesto_split.csv")
    p.add_argument("--sin-manifiesto", action="store_true", help="No comparar contra el manifiesto")
    args = p.parse_args()

    rep = validar_dataset(
        args.raiz,
        clases.NUM_CLASES,
        manifiesto=None if args.sin_manifiesto else args.manifiesto,
    )

    print(f"Dataset: {args.raiz}")
    encabezado = f"{'split':6} {'imágenes':>9} {'negativas':>10} " + " ".join(
        f"{n:>8}" for n in clases.NOMBRES.values()
    )
    print(encabezado)
    for split in SPLITS:
        cajas = " ".join(f"{rep.cajas[split][i]:>8}" for i in clases.NOMBRES)
        print(f"{split:6} {rep.imagenes_por_split[split]:>9} {rep.negativas_por_split[split]:>10} {cajas}")

    if rep.ok:
        print("\nOK: el dataset es válido.")
        return 0

    print(f"\nSe encontraron {len(rep.errores)} errores:")
    for e in rep.errores[:50]:
        print(f"  - {e}")
    if len(rep.errores) > 50:
        print(f"  ... y {len(rep.errores) - 50} más")
    return 1


if __name__ == "__main__":
    sys.exit(main())