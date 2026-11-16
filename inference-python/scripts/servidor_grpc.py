"""
Servidor gRPC mínimo para validar el contrato con Go (HU 05.4).

Carga el modelo UNA vez (HU 05.3) y atiende el contrato urbia.inferencia.v1.
La versión completa (health check HTTP, logging por petición, apagado
ordenado con SIGTERM, Docker) se hace en la HU 05.5.

Uso (desde inference-python/):
    python -m scripts.servidor_grpc                     # escucha en 127.0.0.1:50051
    python -m scripts.servidor_grpc --direccion 0.0.0.0:50051

Ctrl+C para detenerlo.
"""

from __future__ import annotations

import argparse
import logging
import os
import sys

from urbia_vision.contrato_grpc import VERSION_CONTRATO, crear_servidor
from urbia_vision.errores import ErrorInferencia
from urbia_vision.servicio import obtener_servicio


def main() -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--direccion", default=os.environ.get("URBIA_GRPC_DIRECCION", "127.0.0.1:50051"))
    args = p.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    log = logging.getLogger("urbia.inferencia")

    try:
        servicio = obtener_servicio()
    except ErrorInferencia as e:
        log.error("no se pudo iniciar [%s]: %s", e.codigo, e.mensaje)
        return 2

    servidor, puerto = crear_servidor(servicio, args.direccion)
    servidor.start()
    log.info("servidor gRPC escuchando direccion=%s contrato=v%s", args.direccion, VERSION_CONTRATO)
    try:
        servidor.wait_for_termination()
    except KeyboardInterrupt:
        log.info("deteniendo servidor…")
        servidor.stop(grace=5).wait()
    return 0


if __name__ == "__main__":
    sys.exit(main())