"""
Prueba manual del servicio de inferencia (HU 05.3).

Carga el modelo UNA vez y procesa varias imágenes seguidas, mostrando el
resultado de cada una y el tiempo. Sirve como evidencia del criterio de
aceptación: "el modelo se carga una vez y puede procesar múltiples imágenes".

Uso (desde inference-python/):
    python -m scripts.probar_servicio --imagenes datasets/urbia/images/test --max 10
    python -m scripts.probar_servicio --imagenes foto1.jpg foto2.png --invalidas
    python -m scripts.probar_servicio --rutas originales/2026/11/<26 caracteres>.jpg

--imagenes  archivos o carpetas locales (se leen como bytes, sin pasar por STORAGE_DIR)
--rutas     rutas relativas a STORAGE_DIR, como las que guarda Go (04.1)
--invalidas agrega casos inválidos para comprobar que el servicio no se cae
"""

from __future__ import annotations

import argparse
import logging
import statistics
import sys
from pathlib import Path

from urbia_vision.errores import ErrorInferencia
from urbia_vision.servicio import obtener_servicio

EXT = {".jpg", ".jpeg", ".png", ".webp"}


def casos_invalidos() -> list[tuple[str, str, object]]:
    """(tipo, etiqueta, entrada) que deben devolver 'error' sin tumbar el servicio."""
    return [
        ("bytes", "texto.jpg (no es imagen)", b"esto no es una imagen"),
        ("bytes", "jpeg truncado", b"\xff\xd8\xff\xe0" + b"\x00" * 64),
        ("ruta", "../../etc/passwd", "../../etc/passwd"),
        ("ruta", "ruta inexistente", "originales/2026/11/" + "a" * 26 + ".jpg"),
    ]


def listar_imagenes(entradas: list[str], maximo: int) -> list[Path]:
    rutas: list[Path] = []
    for e in entradas:
        p = Path(e)
        if p.is_dir():
            rutas += sorted(x for x in p.iterdir() if x.suffix.lower() in EXT)
        elif p.is_file():
            rutas.append(p)
        else:
            print(f"Aviso: no existe {p}", file=sys.stderr)
    return rutas[:maximo] if maximo else rutas


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--imagenes", nargs="*", default=[])
    p.add_argument("--rutas", nargs="*", default=[])
    p.add_argument("--max", type=int, default=0, help="máximo de imágenes locales (0 = todas)")
    p.add_argument("--invalidas", action="store_true")
    p.add_argument("--verbose", action="store_true", help="mostrar también los mensajes de Ultralytics")
    args = p.parse_args()

    logging.basicConfig(level=logging.INFO if args.verbose else logging.WARNING,
                        format="%(asctime)s %(levelname)s %(name)s %(message)s")
    logging.getLogger("urbia.inferencia").setLevel(logging.INFO)

    try:
        servicio = obtener_servicio()
    except ErrorInferencia as e:
        print(f"\nNO SE PUDO INICIAR EL SERVICIO [{e.codigo}]: {e.mensaje}")
        return 2

    info = servicio.detector.info
    print(f"\nModelo cargado UNA vez: {info.nombre} v{info.version} | umbral {info.umbral_confianza} "
          f"| imgsz {info.imgsz} | dispositivo {info.dispositivo} | {info.segundos_carga:.1f} s")

    trabajos: list[tuple[str, str, object]] = [("archivo", str(r), r) for r in listar_imagenes(args.imagenes, args.max)]
    trabajos += [("ruta", r, r) for r in args.rutas]
    if args.invalidas:
        trabajos += casos_invalidos()
    if not trabajos:
        print("No hay imágenes que procesar. Usa --imagenes, --rutas o --invalidas.")
        return 1

    print(f"\n{'#':>3}  {'entrada':42} {'resultado':13} {'clase':7} {'conf':>6} {'cajas':>5} {'ms':>7}")
    tiempos = []
    for i, (tipo, etiqueta, entrada) in enumerate(trabajos, 1):
        actual = obtener_servicio()
        assert actual is servicio, "el servicio se volvió a crear"  # carga única

        if tipo == "archivo":
            r = servicio.inferir_bytes(Path(entrada).read_bytes(), etiqueta)
        elif tipo == "bytes":
            r = servicio.inferir_bytes(entrada, etiqueta)
        else:
            r = servicio.inferir_archivo(entrada)

        if r.resultado != "error":
            tiempos.append(r.milisegundos)
        nombre = Path(etiqueta).name if tipo == "archivo" else etiqueta
        conf = f"{r.confianza:.2f}" if r.confianza is not None else "-"
        detalle = f"  [{r.codigo_error}] {r.mensaje_error}" if r.codigo_error else ""
        print(f"{i:>3}  {nombre[:42]:42} {r.resultado:13} {r.clase_detectada or '-':7} "
              f"{conf:>6} {len(r.cajas):>5} {r.milisegundos:>7.1f}{detalle}")

    e = servicio.estado()
    print(f"\nPeticiones: {e['peticiones']} | errores controlados: {e['errores']} | "
          f"el servicio siguió activo: sí | cargas del modelo: 1")
    if tiempos:
        print(f"Tiempo por imagen (sin errores): mediana {statistics.median(tiempos):.1f} ms, "
              f"máximo {max(tiempos):.1f} ms")
    return 0


if __name__ == "__main__":
    sys.exit(main())