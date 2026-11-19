"""
Muestra el hardware disponible y la variante de YOLO recomendada (HU 05.2).

Uso (desde inference-python/):
    python -m scripts.info_hardware
"""

from __future__ import annotations

from urbia_vision.hardware import detectar, recomendar


def main() -> None:
    info = detectar()
    rec = recomendar(info)

    print("Hardware detectado")
    for clave, valor in info.como_dict().items():
        print(f"  {clave:16}: {valor}")
    print("\nRecomendación")
    print(f"  modelo_base     : {rec.modelo_base}")
    print(f"  device          : {rec.device}")
    print(f"  batch           : {rec.batch}")
    print(f"  nota            : {rec.nota}")


if __name__ == "__main__":
    main()