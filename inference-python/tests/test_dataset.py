from pathlib import Path

import pytest
import yaml

from urbia_vision import clases
from urbia_vision.dataset import (
    clave_grupo,
    dividir,
    preparar,
    remapear_lineas,
    validar_dataset,
    validar_linea_yolo,
)


# ---------------------------------------------------------------- validación

@pytest.mark.parametrize("linea", [
    "0 0.5 0.5 0.2 0.3",
    "1 0 0 1 1",
])
def test_linea_valida(linea):
    assert validar_linea_yolo(linea, 2) is None


@pytest.mark.parametrize("linea, fragmento", [
    ("0 0.5 0.5 0.2", "5 valores"),
    ("2 0.5 0.5 0.2 0.3", "fuera de rango"),
    ("-1 0.5 0.5 0.2 0.3", "fuera de rango"),
    ("a 0.5 0.5 0.2 0.3", "no entero"),
    ("0 1.5 0.5 0.2 0.3", "fuera de [0, 1]"),
    ("0 0.5 0.5 0 0.3", "mayores que 0"),
    ("0 x 0.5 0.2 0.3", "no numéricas"),
])
def test_linea_invalida(linea, fragmento):
    error = validar_linea_yolo(linea, 2)
    assert error is not None and fragmento in error


# ------------------------------------------------------------------- remapeo

def test_remapeo_descarta_clases_fuera_del_mvp():
    nombres_rdd = ["D00", "D10", "D20", "D40"]
    lineas = ["0 0.1 0.1 0.1 0.1", "3 0.5 0.5 0.2 0.2", "2 0.3 0.3 0.1 0.1"]
    salida = remapear_lineas(lineas, nombres_rdd, clases.MAPEO_FUENTES["rdd2022"], clases.IDS)
    assert salida == ["0 0.500000 0.500000 0.200000 0.200000"]


def test_remapeo_cambia_el_id():
    salida = remapear_lineas(["0 0.5 0.5 0.4 0.4"], ["garbage"],
                             clases.MAPEO_FUENTES["garbage"], clases.IDS)
    assert salida[0].startswith("1 ")  # garbage -> basura (id 1)


def test_remapeo_convierte_poligono_a_caja():
    poligono = "0 0.2 0.2 0.6 0.2 0.6 0.8 0.2 0.8"
    salida = remapear_lineas([poligono], ["pothole"], clases.MAPEO_FUENTES["pothole"], clases.IDS)
    id_, cx, cy, w, h = salida[0].split()
    assert id_ == "0"
    assert (float(cx), float(cy), float(w), float(h)) == pytest.approx((0.4, 0.5, 0.4, 0.6))


# -------------------------------------------------------------------- split

def test_clave_grupo_quita_sufijos_de_roboflow():
    assert clave_grupo("img001_jpg.rf.3f9a2b1c.jpg") == "img001"
    assert clave_grupo("img001_jpg.rf.aaaa1111.jpg") == "img001"
    assert clave_grupo("foto_normal.png") == "foto_normal"


def test_division_reproducible_y_sin_fugas():
    nombres = [f"img{i:03d}_jpg.rf.{s}.jpg" for i in range(100) for s in ("aa11", "bb22", "cc33")]
    a = dividir(nombres, semilla=7)
    b = dividir(nombres, semilla=7)
    assert a == b  # misma semilla, misma división

    grupos = {s: {clave_grupo(n) for n in v} for s, v in a.items()}
    assert not grupos["train"] & grupos["val"]
    assert not grupos["train"] & grupos["test"]
    assert not grupos["val"] & grupos["test"]
    assert sum(len(v) for v in a.values()) == 300
    assert [len(grupos[s]) for s in ("train", "val", "test")] == [70, 20, 10]


def test_division_rechaza_proporciones_invalidas():
    with pytest.raises(ValueError):
        dividir(["a.jpg"], proporciones=(0.5, 0.5, 0.5))


# ------------------------------------------------------- flujo de punta a punta

def _crear_export(raiz: Path, nombres_clases: list[str], n: int, linea: str) -> Path:
    """Simula un export YOLOv8 de Roboflow con imágenes 'falsas'."""
    for i in range(n):
        split = "train" if i % 2 else "valid"
        (raiz / split / "images").mkdir(parents=True, exist_ok=True)
        (raiz / split / "labels").mkdir(parents=True, exist_ok=True)
        (raiz / split / "images" / f"f{i}_jpg.rf.ab{i}.jpg").write_bytes(f"img{i}".encode())
        (raiz / split / "labels" / f"f{i}_jpg.rf.ab{i}.txt").write_text(linea + "\n")
    (raiz / "data.yaml").write_text(yaml.safe_dump({"names": nombres_clases}))
    return raiz


def test_preparar_y_validar(tmp_path):
    baches = _crear_export(tmp_path / "pothole", ["pothole"], 20, "0 0.5 0.5 0.2 0.2")
    basura = _crear_export(tmp_path / "garbage", ["garbage"], 20, "0 0.4 0.4 0.3 0.3")
    destino = tmp_path / "urbia"
    manifiesto = tmp_path / "manifiesto.csv"

    resumen = preparar(
        fuentes={
            "pothole": (baches, ["pothole"], clases.MAPEO_FUENTES["pothole"]),
            "garbage": (basura, ["garbage"], clases.MAPEO_FUENTES["garbage"]),
        },
        destino=destino,
        ids_destino=clases.IDS,
        manifiesto=manifiesto,
    )
    assert resumen.imagenes == 40
    assert resumen.cajas_por_clase[0] == 20 and resumen.cajas_por_clase[1] == 20
    assert manifiesto.exists()

    rep = validar_dataset(destino, clases.NUM_CLASES, manifiesto)
    assert rep.ok, rep.errores
    assert sum(rep.imagenes_por_split.values()) == 40
    assert rep.imagenes_por_split["test"] > 0


def test_validar_detecta_problemas(tmp_path):
    for split in ("train", "val", "test"):
        (tmp_path / "images" / split).mkdir(parents=True)
        (tmp_path / "labels" / split).mkdir(parents=True)
        (tmp_path / "images" / split / f"{split}.jpg").write_bytes(b"x")
        (tmp_path / "labels" / split / f"{split}.txt").write_text("0 0.5 0.5 0.1 0.1\n")

    (tmp_path / "labels" / "train" / "train.txt").write_text("5 0.5 0.5 0.1 0.1\n")   # clase inválida
    (tmp_path / "labels" / "val" / "val.txt").unlink()                                # sin etiqueta
    (tmp_path / "labels" / "test" / "huerfana.txt").write_text("0 0.5 0.5 0.1 0.1\n") # sin imagen
    (tmp_path / "images" / "test" / "train_jpg.rf.ab9.jpg").write_bytes(b"y")         # fuga train->test
    (tmp_path / "labels" / "test" / "train_jpg.rf.ab9.txt").write_text("")

    rep = validar_dataset(tmp_path, clases.NUM_CLASES)
    texto = "\n".join(rep.errores)
    assert not rep.ok
    assert "fuera de rango" in texto
    assert "no tiene archivo de etiquetas" in texto
    assert "etiqueta sin imagen" in texto
    assert "fuga de datos" in texto


def test_manifiesto_detecta_cambio_en_test(tmp_path):
    export = _crear_export(tmp_path / "pothole", ["pothole"], 20, "0 0.5 0.5 0.2 0.2")
    destino, manifiesto = tmp_path / "urbia", tmp_path / "m.csv"
    preparar({"pothole": (export, ["pothole"], clases.MAPEO_FUENTES["pothole"])},
             destino, clases.IDS, manifiesto)

    una_de_test = next((destino / "images" / "test").iterdir())
    una_de_test.write_bytes(b"modificada")

    rep = validar_dataset(destino, clases.NUM_CLASES, manifiesto)
    assert any("sha256 distinto" in e for e in rep.errores)