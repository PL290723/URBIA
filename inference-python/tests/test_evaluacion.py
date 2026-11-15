import pytest

from urbia_vision.clases import NOMBRES
from urbia_vision.decision import Deteccion
from urbia_vision.evaluacion import (
    FONDO,
    ImagenEvaluada,
    Verdad,
    barrer_umbrales,
    clase_esperada,
    elegir_umbral,
    emparejar,
    evaluar,
    evaluar_por_imagen,
    iou,
    leer_verdades,
    yolo_a_caja,
)

A = (0.0, 0.0, 10.0, 10.0)
B = (100.0, 100.0, 120.0, 120.0)
CLASES = [0, 1]


def test_iou():
    assert iou(A, A) == 1.0
    assert iou(A, B) == 0.0
    assert iou(A, (5.0, 0.0, 15.0, 10.0)) == pytest.approx(50 / 150)


def test_yolo_a_caja():
    assert yolo_a_caja(0.5, 0.5, 0.2, 0.4, 100, 50) == pytest.approx((40, 15, 60, 35))


def test_leer_verdades(tmp_path):
    txt = tmp_path / "a.txt"
    txt.write_text("1 0.5 0.5 0.2 0.4\n")
    assert leer_verdades(txt, 100, 50) == [Verdad(1, pytest.approx((40, 15, 60, 35)))]
    assert leer_verdades(tmp_path / "no_existe.txt", 100, 50) == []


def test_emparejar_clasifica_tp_fp_fn_y_confusion():
    img = ImagenEvaluada("x.jpg",
                         verdades=[Verdad(0, A), Verdad(1, B), Verdad(0, (50, 50, 60, 60))],
                         predicciones=[Deteccion(0, 0.9, A),                 # TP bache
                                       Deteccion(0, 0.8, B),                 # confusión basura->bache
                                       Deteccion(1, 0.7, (300, 300, 310, 310))])  # FP
    e = emparejar(img, umbral=0.5)
    assert sorted(e.pares) == sorted([(0, 0), (0, 1), (1, FONDO), (FONDO, 0)])
    assert len(e.falsos_positivos) == 1 and len(e.falsos_negativos) == 1
    assert len(e.confusiones) == 1


def test_evaluar_calcula_metricas_por_clase():
    img = ImagenEvaluada("x.jpg", [Verdad(0, A), Verdad(1, B)],
                         [Deteccion(0, 0.9, A), Deteccion(1, 0.3, B)])
    r = evaluar([img], CLASES, umbral=0.5)
    assert (r.por_clase[0].tp, r.por_clase[0].fp, r.por_clase[0].fn) == (1, 0, 0)
    assert (r.por_clase[1].tp, r.por_clase[1].fn) == (0, 1)  # la de 0.3 no pasa el umbral
    assert r.matriz[FONDO][1] == 1
    assert r.por_clase[0].f1 == 1.0 and r.por_clase[1].f1 == 0.0


def test_elegir_umbral_maximiza_f1_macro():
    # Una predicción correcta con 0.6 y un falso positivo con 0.3:
    # el mejor umbral queda por encima de 0.3 y hasta 0.6
    img = ImagenEvaluada("x.jpg", [Verdad(0, A), Verdad(1, B)],
                         [Deteccion(0, 0.6, A), Deteccion(1, 0.65, B),
                          Deteccion(0, 0.3, (300, 300, 310, 310))])
    elegido = elegir_umbral(barrer_umbrales([img], CLASES))
    assert 0.3 < elegido.umbral <= 0.6
    assert elegido.f1_macro == 1.0


def test_clase_esperada():
    assert clase_esperada([]) == FONDO
    assert clase_esperada([Verdad(1, A), Verdad(1, B), Verdad(0, A)]) == 1


def test_evaluar_por_imagen():
    imgs = [
        ImagenEvaluada("a.jpg", [Verdad(0, A)], [Deteccion(0, 0.9, A)]),   # acierto
        ImagenEvaluada("b.jpg", [Verdad(1, B)], [Deteccion(0, 0.9, B)]),   # error de clase
        ImagenEvaluada("c.jpg", [], [Deteccion(1, 0.2, A)]),               # negativa, acierto
    ]
    r = evaluar_por_imagen(imgs, 0.5, NOMBRES)
    assert r["imagenes"] == 3
    assert r["exactitud"] == pytest.approx(2 / 3, abs=1e-4)
    assert r["matriz"][0][1] == 1 and r["matriz"][FONDO][FONDO] == 1