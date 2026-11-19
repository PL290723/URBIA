"""
Configuración del servidor de inferencia (HU 05.5).

Complementa a `configuracion.py` (HU 05.3, modelo y STORAGE_DIR) con lo que
necesita el proceso que se expone en la red. Todo sale de variables de entorno;
los valores por defecto sirven para desarrollo local en Windows. En Docker se
usan direcciones 0.0.0.0 para escuchar dentro de la red interna.

    Variable                    Por defecto        Para qué
    URBIA_GRPC_DIRECCION        127.0.0.1:50051    gRPC (Go: INFERENCE_GRPC_ADDR)
    URBIA_SALUD_DIRECCION       127.0.0.1:8000     GET /health (Go: INFERENCE_URL)
    URBIA_GRPC_HILOS            4                  peticiones gRPC atendidas a la vez
    URBIA_TIMEOUT_INFERENCIA    8                  segundos máx. por inferencia (< 10 s de Go)
    URBIA_APAGADO_GRACIA        10                 segundos para terminar peticiones al apagar
    URBIA_LOG_NIVEL             INFO               DEBUG, INFO, WARNING, ERROR
"""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass

NIVELES_LOG = ("DEBUG", "INFO", "WARNING", "ERROR")


def validar_direccion(valor: str) -> str | None:
    """Mensaje de error si `valor` no es host:puerto (puerto 0 = uno libre, para pruebas)."""
    host, sep, puerto = valor.rpartition(":")
    if not sep or not host:
        return f"{valor!r} no tiene la forma host:puerto"
    if not puerto.isdigit() or not 0 <= int(puerto) <= 65535:
        return f"{valor!r} tiene un puerto inválido"
    return None


@dataclass(frozen=True)
class ConfigServidor:
    direccion_grpc: str = "127.0.0.1:50051"
    direccion_salud: str = "127.0.0.1:8000"
    hilos: int = 4
    timeout_inferencia: float = 8.0
    gracia_apagado: float = 10.0
    nivel_log: str = "INFO"

    @property
    def nivel_log_numerico(self) -> int:
        return getattr(logging, self.nivel_log)

    @classmethod
    def desde_entorno(cls) -> "ConfigServidor":
        errores: list[str] = []
        base = cls()

        def texto(variable: str, por_defecto: str) -> str:
            return (os.environ.get(variable) or "").strip() or por_defecto

        def direccion(variable: str, por_defecto: str) -> str:
            v = texto(variable, por_defecto)
            error = validar_direccion(v)
            if error:
                errores.append(f"{variable}: {error}")
            return v

        def numero(variable: str, por_defecto, tipo, minimo, maximo):
            v = texto(variable, "")
            if not v:
                return por_defecto
            try:
                n = tipo(v)
            except ValueError:
                errores.append(f"{variable} debe ser un número, se recibió {v!r}")
                return por_defecto
            if not minimo <= n <= maximo:
                errores.append(f"{variable} debe estar entre {minimo} y {maximo}, se recibió {v}")
                return por_defecto
            return n

        nivel = texto("URBIA_LOG_NIVEL", base.nivel_log).upper()
        if nivel not in NIVELES_LOG:
            errores.append(f"URBIA_LOG_NIVEL debe ser uno de {', '.join(NIVELES_LOG)}")
            nivel = base.nivel_log

        config = cls(
            direccion_grpc=direccion("URBIA_GRPC_DIRECCION", base.direccion_grpc),
            direccion_salud=direccion("URBIA_SALUD_DIRECCION", base.direccion_salud),
            hilos=numero("URBIA_GRPC_HILOS", base.hilos, int, 1, 32),
            timeout_inferencia=numero("URBIA_TIMEOUT_INFERENCIA", base.timeout_inferencia, float, 0.5, 60),
            gracia_apagado=numero("URBIA_APAGADO_GRACIA", base.gracia_apagado, float, 0, 120),
            nivel_log=nivel,
        )
        if errores:
            raise ValueError("Configuración inválida del servidor: " + "; ".join(errores))
        return config

    def como_dict(self) -> dict:
        return {
            "grpc": self.direccion_grpc, "salud": self.direccion_salud, "hilos": self.hilos,
            "timeout_inferencia": self.timeout_inferencia, "gracia_apagado": self.gracia_apagado,
            "nivel_log": self.nivel_log,
        }