"""
Servidor del servicio de inferencia de URBIA (HU 05.5).

Carga el modelo UNA vez (HU 05.3), atiende el contrato gRPC urbia.inferencia.v1
(HU 05.4) y expone GET /health para el backend en Go.

Uso (desde inference-python/):
    python -m scripts.servidor_grpc
    python -m scripts.servidor_grpc --grpc 0.0.0.0:50051 --salud 0.0.0.0:8000

Los argumentos tienen prioridad sobre las variables de entorno
(URBIA_GRPC_DIRECCION, URBIA_SALUD_DIRECCION, ver configuracion_servidor.py).
Ctrl+C o SIGTERM lo detienen de forma ordenada.

Códigos de salida: 0 detenido normalmente, 2 el modelo no cargó, 3 configuración o puerto inválidos.
"""

from __future__ import annotations

import argparse
import dataclasses
import logging
import sys

from urbia_vision.configuracion_servidor import ConfigServidor, validar_direccion
from urbia_vision.errores import ErrorInferencia
from urbia_vision.registro import configurar_logging
from urbia_vision.servidor import ServidorInferencia, instalar_senales

log = logging.getLogger("urbia.inferencia")


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--grpc", help="host:puerto del servidor gRPC")
    p.add_argument("--salud", help="host:puerto del GET /health")
    args = p.parse_args(argv)

    try:
        config = ConfigServidor.desde_entorno()
        cambios = {k: v for k, v in (("direccion_grpc", args.grpc), ("direccion_salud", args.salud)) if v}
        for valor in cambios.values():
            if error := validar_direccion(valor):
                raise ValueError(error)
        config = dataclasses.replace(config, **cambios)
    except ValueError as e:
        configurar_logging()
        log.error("%s", e)
        return 3

    configurar_logging(config.nivel_log_numerico)
    log.info("configuración %s", " ".join(f"{k}={v}" for k, v in config.como_dict().items()))

    servidor = ServidorInferencia(config)
    instalar_senales(servidor)
    try:
        servidor.iniciar()
    except ErrorInferencia as e:
        log.error("no se pudo iniciar [%s]: %s", e.codigo, e.mensaje)
        return 2
    except (OSError, RuntimeError) as e:   # puerto ocupado o dirección inválida
        log.error("no se pudo abrir el servidor: %s", e)
        return 3

    servidor.esperar()
    return 0


if __name__ == "__main__":
    sys.exit(main())