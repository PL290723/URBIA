"""
Regla de decisión del detector de URBIA (HU 05.2).

Convierte las cajas que devuelve el modelo en UN resultado por imagen,
con los mismos valores que la tabla `deteccion` de la base de datos:

    resultado        clase_detectada   confianza
    'detectado'      'bache'/'basura'  0..1
    'no_detectado'   NULL              NULL
    'error'          NULL              NULL      (lo genera el servicio, 05.3/05.5)

Regla:
1. Se descartan las cajas con confianza menor al umbral.
2. Si no queda ninguna  -> 'no_detectado' (no hay detección concluyente).
3. Si quedan cajas      -> 'detectado' con la clase de la caja más confiable.

El modelo solo SUGIERE la categoría; la confirma el administrador.
"""

from __future__ import annotations

from dataclasses import dataclass, field

DETECTADO = "detectado"
NO_DETECTADO = "no_detectado"
ERROR = "error"


@dataclass(frozen=True)
class Deteccion:
    clase: int
    confianza: float
    caja: tuple[float, float, float, float]  # x1, y1, x2, y2 en píxeles


@dataclass
class ResultadoImagen:
    resultado: str
    clase_detectada: str | None
    confianza: float | None
    detecciones: list[Deteccion] = field(default_factory=list)


def interpretar(
    detecciones: list[Deteccion],
    umbral: float,
    nombres: dict[int, str],
) -> ResultadoImagen:
    if not 0.0 < umbral < 1.0:
        raise ValueError(f"El umbral debe estar entre 0 y 1, se recibió {umbral}")

    validas = sorted(
        (d for d in detecciones if d.confianza >= umbral),
        key=lambda d: d.confianza,
        reverse=True,
    )
    if not validas:
        return ResultadoImagen(NO_DETECTADO, None, None, [])

    mejor = validas[0]
    return ResultadoImagen(
        resultado=DETECTADO,
        clase_detectada=nombres[mejor.clase],
        confianza=round(mejor.confianza, 4),  # NUMERIC(5,4) en la BD
        detecciones=validas,
    )