"""
Normalización del resultado antes de responder por gRPC (HU 05.5).

`ServicioInferencia` (HU 05.3) ya aplica la regla de decisión y redondea. Aquí
se revisa que el resultado cumpla el contrato urbia.inferencia.v1 ANTES de
enviarlo, para que Go nunca reciba algo que no sabe guardar en `deteccion`:

- `resultado` solo puede ser 'detectado' o 'no_detectado' (los errores ya se
  convirtieron en códigos gRPC).
- 'detectado'     -> clase de `clases.py` y confianza entre el umbral y 1.
- 'no_detectado'  -> clase y confianza en None y ninguna caja sobre el umbral.
- Cajas: clase conocida, confianza finita en [umbral, 1] con 4 decimales,
  coordenadas recortadas a la foto con 1 decimal, x1 < x2 y y1 < y2,
  ordenadas de mayor a menor confianza. Las cajas degeneradas se descartan.

Si algo no cuadra es un fallo del modelo o del código, no de la foto:
se lanza FalloInferencia y Go recibe INTERNAL + FALLO_INFERENCIA.
"""

from __future__ import annotations

import math
from dataclasses import replace

from urbia_vision import clases
from urbia_vision.decision import DETECTADO, NO_DETECTADO
from urbia_vision.errores import FalloInferencia
from urbia_vision.modelo import Caja

DECIMALES_CONFIANZA = 4   # NUMERIC(5,4) en la BD
DECIMALES_PIXELES = 1
LADO_MINIMO_CAJA = 1.0    # px; una caja más pequeña no es una detección útil


def _confianza(valor, que: str) -> float:
    try:
        v = float(valor)
    except (TypeError, ValueError):
        raise FalloInferencia(f"Resultado inconsistente: {que} no es un número") from None
    if not math.isfinite(v) or not 0.0 <= v <= 1.0:
        raise FalloInferencia(f"Resultado inconsistente: {que} fuera de 0..1 ({v})")
    return round(v, DECIMALES_CONFIANZA)


def _recortar(a: float, b: float, limite: int) -> tuple[float, float]:
    a, b = sorted((min(max(float(a), 0.0), limite), min(max(float(b), 0.0), limite)))
    return round(a, DECIMALES_PIXELES), round(b, DECIMALES_PIXELES)


def normalizar_cajas(cajas: list[Caja], umbral: float, ancho: int, alto: int) -> list[Caja]:
    salida = []
    for c in cajas:
        if c.clase not in clases.IDS:
            raise FalloInferencia(f"Resultado inconsistente: clase desconocida {c.clase!r}")
        conf = _confianza(c.confianza, "la confianza de una caja")
        if conf < umbral:
            continue
        for v in (c.x1, c.y1, c.x2, c.y2):
            if not math.isfinite(float(v)):
                raise FalloInferencia("Resultado inconsistente: coordenada no finita")
        x1, x2 = _recortar(c.x1, c.x2, ancho)
        y1, y2 = _recortar(c.y1, c.y2, alto)
        if x2 - x1 < LADO_MINIMO_CAJA or y2 - y1 < LADO_MINIMO_CAJA:
            continue
        salida.append(Caja(c.clase, conf, x1, y1, x2, y2))
    salida.sort(key=lambda k: (-k.confianza, k.clase, k.x1, k.y1))
    return salida


def normalizar(r):
    """Copia normalizada de un ResultadoInferencia sin error, o FalloInferencia."""
    if r.resultado not in (DETECTADO, NO_DETECTADO):
        raise FalloInferencia(f"Resultado inconsistente: {r.resultado!r} no es detectado/no_detectado")
    if not r.ancho or not r.alto or r.ancho <= 0 or r.alto <= 0:
        raise FalloInferencia("Resultado inconsistente: faltan las dimensiones de la foto")

    umbral = _confianza(r.umbral_confianza, "el umbral")
    cajas = normalizar_cajas(list(r.cajas), umbral, int(r.ancho), int(r.alto))

    if r.resultado == DETECTADO:
        if r.clase_detectada not in clases.IDS:
            raise FalloInferencia(f"Resultado inconsistente: clase detectada {r.clase_detectada!r}")
        if r.confianza is None:
            raise FalloInferencia("Resultado inconsistente: 'detectado' sin confianza")
        confianza = _confianza(r.confianza, "la confianza")
        if confianza < umbral:
            raise FalloInferencia("Resultado inconsistente: 'detectado' con confianza menor al umbral")
        clase = r.clase_detectada
    else:
        if cajas:
            raise FalloInferencia("Resultado inconsistente: 'no_detectado' con cajas sobre el umbral")
        clase, confianza = None, None

    return replace(r, clase_detectada=clase, confianza=confianza, umbral_confianza=umbral,
                   cajas=cajas, milisegundos=round(float(r.milisegundos), 1))