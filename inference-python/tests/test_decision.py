import pytest

from urbia_vision.clases import NOMBRES
from urbia_vision.decision import DETECTADO, NO_DETECTADO, Deteccion, interpretar

CAJA = (0.0, 0.0, 10.0, 10.0)


def test_sin_detecciones_es_no_detectado():
    r = interpretar([], 0.4, NOMBRES)
    assert (r.resultado, r.clase_detectada, r.confianza) == (NO_DETECTADO, None, None)


def test_todas_bajo_el_umbral_es_no_detectado():
    r = interpretar([Deteccion(0, 0.39, CAJA), Deteccion(1, 0.2, CAJA)], 0.4, NOMBRES)
    assert r.resultado == NO_DETECTADO
    assert r.detecciones == []


def test_gana_la_caja_mas_confiable():
    dets = [Deteccion(0, 0.55, CAJA), Deteccion(1, 0.81, CAJA), Deteccion(0, 0.6, CAJA)]
    r = interpretar(dets, 0.4, NOMBRES)
    assert r.resultado == DETECTADO
    assert r.clase_detectada == "basura"
    assert r.confianza == 0.81
    assert [d.confianza for d in r.detecciones] == [0.81, 0.6, 0.55]


def test_umbral_es_inclusivo():
    r = interpretar([Deteccion(0, 0.4, CAJA)], 0.4, NOMBRES)
    assert r.resultado == DETECTADO and r.clase_detectada == "bache"


def test_confianza_con_4_decimales_para_la_bd():
    r = interpretar([Deteccion(0, 0.123456, CAJA)], 0.1, NOMBRES)
    assert r.confianza == 0.1235


@pytest.mark.parametrize("umbral", [0, 1, -0.1, 1.5])
def test_umbral_invalido(umbral):
    with pytest.raises(ValueError):
        interpretar([], umbral, NOMBRES)