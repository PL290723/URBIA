"""
Clases de detección del MVP de URBIA (HU 05.1).

Este archivo es la ÚNICA fuente de verdad de las clases que el modelo
intenta detectar. Reglas:

- El índice (id) es el que se escribe en las etiquetas YOLO y el que
  devuelve el modelo. No se reordena nunca: si se agrega una clase,
  va al final con el siguiente id libre.
- El nombre coincide EXACTAMENTE con `categoria.nombre` de la base de
  datos (migración 00002_datos_semilla.sql), para que el backend en Go
  pueda guardar `deteccion.clase_detectada` y sugerir la categoría sin
  tablas de traducción.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ClaseDeteccion:
    id: int
    nombre: str
    descripcion: str


CLASES: tuple[ClaseDeteccion, ...] = (
    ClaseDeteccion(
        id=0,
        nombre="bache",
        descripcion="Hundimiento u hoyo en el pavimento de la calle.",
    ),
    ClaseDeteccion(
        id=1,
        nombre="basura",
        descripcion="Acumulación de basura o tiradero en vía pública "
                    "(no objetos sueltos aislados).",
    ),
)

# Accesos rápidos
NOMBRES: dict[int, str] = {c.id: c.nombre for c in CLASES}
IDS: dict[str, int] = {c.nombre: c.id for c in CLASES}
NUM_CLASES: int = len(CLASES)


# Cómo se traducen las etiquetas de los datasets públicos a las clases
# de URBIA. Cualquier etiqueta que NO aparezca aquí se descarta al
# preparar el dataset (por ejemplo, grietas de RDD2022 u objetos sueltos).
MAPEO_FUENTES: dict[str, dict[str, str]] = {
    "pothole": {          # dataset de baches (Roboflow Universe, CC BY 4.0)
        "pothole": "bache",
        "potholes": "bache",
        "Pothole": "bache",
    },
    "garbage": {          # dataset de acumulaciones de basura (CC BY 4.0)
        "garbage": "basura",
        "Garbage": "basura",
        "trash": "basura",
    },
    "rdd2022": {          # opcional, RDD2022 (CC BY-SA 4.0)
        "D40": "bache",   # D40 = pothole. D00/D10/D20 (grietas) se descartan.
    },
}


def id_de(nombre: str) -> int:
    """Devuelve el id YOLO de una clase de URBIA o lanza KeyError."""
    return IDS[nombre]


def nombre_de(id_clase: int) -> str:
    """Devuelve el nombre de una clase a partir de su id o lanza KeyError."""
    return NOMBRES[id_clase]


def validar_definicion() -> None:
    """Verifica que los ids sean consecutivos desde 0 y los nombres únicos."""
    ids = [c.id for c in CLASES]
    if ids != list(range(len(CLASES))):
        raise ValueError(f"Los ids deben ser 0..N-1 y en orden, se encontró {ids}")
    nombres = [c.nombre for c in CLASES]
    if len(set(nombres)) != len(nombres):
        raise ValueError(f"Hay nombres de clase repetidos: {nombres}")
    for fuente, mapeo in MAPEO_FUENTES.items():
        for origen, destino in mapeo.items():
            if destino not in IDS:
                raise ValueError(
                    f"MAPEO_FUENTES['{fuente}']['{origen}'] apunta a '{destino}', "
                    f"que no es una clase de URBIA"
                )