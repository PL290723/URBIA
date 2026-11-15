"""
Errores del servicio de inferencia (HU 05.3).

Cada error lleva un `codigo` estable en MAYÚSCULAS. En la HU 05.4 estos
códigos se traducen a códigos de estado gRPC; aquí solo se definen.

    Error                  codigo                 ¿Quién lo causa?           ¿Detiene el servicio?
    ModeloNoDisponible     MODELO_NO_DISPONIBLE   faltan pesos o modelo.json  Sí (al iniciar)
    ModeloInvalido         MODELO_INVALIDO        SHA-256 o clases distintas  Sí (al iniciar)
    RutaInvalida           RUTA_INVALIDA          la petición                 No
    ImagenNoEncontrada     IMAGEN_NO_ENCONTRADA   la petición / el disco      No
    ImagenInvalida         IMAGEN_INVALIDA        el archivo                  No
    FalloInferencia        FALLO_INFERENCIA       el modelo / la GPU          No
"""

from __future__ import annotations


class ErrorInferencia(Exception):
    """Base de todos los errores del servicio."""

    codigo = "ERROR_INTERNO"

    def __init__(self, mensaje: str):
        super().__init__(mensaje)
        self.mensaje = mensaje


class ModeloNoDisponible(ErrorInferencia):
    codigo = "MODELO_NO_DISPONIBLE"


class ModeloInvalido(ErrorInferencia):
    codigo = "MODELO_INVALIDO"


class RutaInvalida(ErrorInferencia):
    codigo = "RUTA_INVALIDA"


class ImagenNoEncontrada(ErrorInferencia):
    codigo = "IMAGEN_NO_ENCONTRADA"


class ImagenInvalida(ErrorInferencia):
    codigo = "IMAGEN_INVALIDA"


class FalloInferencia(ErrorInferencia):
    codigo = "FALLO_INFERENCIA"