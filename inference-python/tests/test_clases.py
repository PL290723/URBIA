import re
from pathlib import Path

import pytest

from urbia_vision import clases

SEMILLA_BD = (
    Path(__file__).resolve().parents[2] / "database" / "migrations" / "00002_datos_semilla.sql"
)


def test_definicion_valida():
    clases.validar_definicion()  # no debe lanzar


def test_clases_del_mvp():
    assert clases.NOMBRES == {0: "bache", 1: "basura"}
    assert clases.NUM_CLASES == 2


def test_ids_y_nombres_ida_y_vuelta():
    for c in clases.CLASES:
        assert clases.nombre_de(c.id) == c.nombre
        assert clases.id_de(c.nombre) == c.id


def test_clase_inexistente_lanza_error():
    with pytest.raises(KeyError):
        clases.id_de("grafiti")


def test_mapeos_apuntan_a_clases_existentes():
    for mapeo in clases.MAPEO_FUENTES.values():
        assert set(mapeo.values()) <= set(clases.IDS)


@pytest.mark.skipif(not SEMILLA_BD.exists(), reason="no se encontró la migración de semilla")
def test_clases_existen_como_categoria_en_la_bd():
    """Cada clase del modelo debe ser una categoría sembrada en PostgreSQL."""
    sql = SEMILLA_BD.read_text(encoding="utf-8")
    seccion_up = sql.split("-- +goose Down")[0]
    categorias = set(re.findall(r"\(\s*'([a-z_]+)'\s*,", seccion_up))
    assert categorias, "no se pudieron leer las categorías de la migración"
    for nombre in clases.IDS:
        assert nombre in categorias, f"'{nombre}' no está en la tabla categoria"