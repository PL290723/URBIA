from pathlib import Path

import pytest
import yaml

from urbia_vision import clases
from urbia_vision.dataset import (
    clave_grupo,
    dividir,
    preparar,
    split_de_origen,
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
        (raiz / split / "images" / f"f{i}_jpg.rf.ab{i}.jpg").write_bytes(f"{raiz.name}{i}".encode())
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
        (tmp_path / "images" / split / f"{split}.jpg").write_bytes(split.encode())
        (tmp_path / "labels" / split / f"{split}.txt").write_text("0 0.5 0.5 0.1 0.1\n")

    (tmp_path / "labels" / "train" / "train.txt").write_text("5 0.5 0.5 0.1 0.1\n")   # clase inválida
    (tmp_path / "labels" / "val" / "val.txt").unlink()                                # sin etiqueta
    (tmp_path / "labels" / "test" / "huerfana.txt").write_text("0 0.5 0.5 0.1 0.1\n") # sin imagen
    (tmp_path / "images" / "test" / "copia.jpg").write_bytes(b"train")                # misma foto que train
    (tmp_path / "labels" / "test" / "copia.txt").write_text("")

    rep = validar_dataset(tmp_path, clases.NUM_CLASES)
    texto = "\n".join(rep.errores)
    assert not rep.ok
    assert "fuera de rango" in texto
    assert "no tiene archivo de etiquetas" in texto
    assert "etiqueta sin imagen" in texto
    assert "fuga de datos (archivo idéntico): train/train.jpg ~ test/copia.jpg" in texto


def _foto(ruta: Path, semilla: int, ruido: int = 0) -> None:
    """Imagen JPG con un patrón propio de la semilla; `ruido` altera algunos píxeles."""
    import random
    from PIL import Image

    rnd = random.Random(semilla)
    im = Image.new("RGB", (96, 96))
    for x in range(0, 96, 12):
        for y in range(0, 96, 12):
            color = tuple(rnd.randint(0, 255) for _ in range(3))
            for dx in range(12):
                for dy in range(12):
                    im.putpixel((x + dx, y + dy), color)
    rnd2 = random.Random(1000 + semilla)
    for _ in range(ruido):
        im.putpixel((rnd2.randrange(96), rnd2.randrange(96)), (255, 255, 255))
    ruta.parent.mkdir(parents=True, exist_ok=True)
    im.save(ruta, quality=90)


def _dataset_vacio(raiz: Path) -> None:
    for split in ("train", "val", "test"):
        (raiz / "images" / split).mkdir(parents=True, exist_ok=True)
        (raiz / "labels" / split).mkdir(parents=True, exist_ok=True)


def test_mismo_nombre_fotos_distintas_no_es_fuga(tmp_path):
    """Caso real del dataset de basura: 'images46.jpg' en train y en val, pero son otra foto."""
    _dataset_vacio(tmp_path)
    for split, semilla in (("train", 1), ("val", 2), ("test", 3)):
        nombre = f"garbage__images46_jpg.rf.{semilla:02x}a.jpg"
        _foto(tmp_path / "images" / split / nombre, semilla)
        (tmp_path / "labels" / split / nombre.replace(".jpg", ".txt")).write_text("1 0.5 0.5 0.2 0.2\n")

    rep = validar_dataset(tmp_path, clases.NUM_CLASES)
    assert rep.ok, rep.errores
    assert rep.advertencias == []


def test_casi_duplicado_genera_advertencia(tmp_path):
    """La misma foto con un poco de ruido y otro nombre en train y test."""
    _dataset_vacio(tmp_path)
    _foto(tmp_path / "images" / "train" / "a.jpg", 7)
    _foto(tmp_path / "images" / "val" / "b.jpg", 8)
    _foto(tmp_path / "images" / "test" / "otra_cosa.jpg", 7, ruido=20)
    for split, n in (("train", "a"), ("val", "b"), ("test", "otra_cosa")):
        (tmp_path / "labels" / split / f"{n}.txt").write_text("")

    rep = validar_dataset(tmp_path, clases.NUM_CLASES)
    assert rep.ok  # no es error: puede ser coincidencia, se revisa a mano
    assert len(rep.advertencias) == 1
    assert "train/a.jpg ~ test/otra_cosa.jpg" in rep.advertencias[0]


def test_manifiesto_detecta_cambio_en_test(tmp_path):
    export = _crear_export(tmp_path / "pothole", ["pothole"], 20, "0 0.5 0.5 0.2 0.2")
    destino, manifiesto = tmp_path / "urbia", tmp_path / "m.csv"
    preparar({"pothole": (export, ["pothole"], clases.MAPEO_FUENTES["pothole"])},
             destino, clases.IDS, manifiesto)

    una_de_test = next((destino / "images" / "test").iterdir())
    una_de_test.write_bytes(b"modificada")

    rep = validar_dataset(destino, clases.NUM_CLASES, manifiesto)
    assert any("sha256 distinto" in e for e in rep.errores)

# --------------------------------------------------- respetar split del export

def _crear_export_roboflow(raiz: Path) -> Path:
    """train/ con 3 variantes aumentadas por foto; valid/ y test/ sin aumentar."""
    def guardar(split, nombre):
        (raiz / split / "images").mkdir(parents=True, exist_ok=True)
        (raiz / split / "labels").mkdir(parents=True, exist_ok=True)
        (raiz / split / "images" / f"{nombre}.jpg").write_bytes(nombre.encode())
        (raiz / split / "labels" / f"{nombre}.txt").write_text("0 0.5 0.5 0.2 0.2\n")

    for i in range(10):
        for h in ("aa1", "bb2", "cc3"):
            guardar("train", f"t{i}_jpg.rf.{h}")
    for i in range(4):
        guardar("valid", f"v{i}_jpg.rf.dd{i}")
    for i in range(2):
        guardar("test", f"p{i}_jpg.rf.ee{i}")
    (raiz / "data.yaml").write_text(yaml.safe_dump({"names": ["pothole"]}))
    return raiz


def test_split_de_origen():
    assert split_de_origen(Path("x/train/images/a.jpg")) == "train"
    assert split_de_origen(Path("x/valid/images/a.jpg")) == "val"
    assert split_de_origen(Path("x/TEST/images/a.jpg")) == "test"
    assert split_de_origen(Path("x/otra/images/a.jpg")) is None


def test_preparar_respetando_split_original(tmp_path):
    export = _crear_export_roboflow(tmp_path / "pothole")
    destino, manifiesto = tmp_path / "urbia", tmp_path / "m.csv"
    resumen = preparar({"pothole": (export, ["pothole"], clases.MAPEO_FUENTES["pothole"])},
                       destino, clases.IDS, manifiesto, split_original=True)

    assert dict(resumen.por_split) == {"train": 30, "val": 4, "test": 2}
    test = sorted(p.name for p in (destino / "images" / "test").iterdir())
    assert test == ["pothole__p0_jpg.rf.ee0.jpg", "pothole__p1_jpg.rf.ee1.jpg"]
    rep = validar_dataset(destino, clases.NUM_CLASES, manifiesto)
    assert rep.ok, rep.errores


def test_split_original_falla_sin_carpetas(tmp_path):
    raiz = tmp_path / "plano"
    (raiz / "images").mkdir(parents=True)
    (raiz / "labels").mkdir()
    (raiz / "images" / "a.jpg").write_bytes(b"a")
    (raiz / "labels" / "a.txt").write_text("")
    with pytest.raises(ValueError, match="no están en carpetas"):
        preparar({"pothole": (raiz, ["pothole"], clases.MAPEO_FUENTES["pothole"])},
                 tmp_path / "urbia", clases.IDS, tmp_path / "m.csv", split_original=True)

def test_preparar_quita_duplicados_de_val_y_test(tmp_path):
    """
    Caso real del dataset de basura: el autor subió la misma foto dos veces
    (o la misma foto de banco con otro nombre) y quedó en train y en val/test.
    """
    export = tmp_path / "garbage"

    def guardar(split, nombre, semilla, ruido=0):
        _foto(export / split / "images" / f"{nombre}.jpg", semilla, ruido)
        (export / split / "labels").mkdir(parents=True, exist_ok=True)
        (export / split / "labels" / f"{nombre}.txt").write_text("0 0.5 0.5 0.3 0.3\n")

    for i in range(4):
        guardar("train", f"t{i}_jpg.rf.a{i}", semilla=i)
    guardar("valid", "basura-slp_jpg.rf.b1", semilla=0)              # idéntica a t0
    guardar("valid", "v1_jpg.rf.b2", semilla=50)                      # foto nueva
    guardar("test", "otra-web_jpg.rf.c1", semilla=1, ruido=20)        # casi idéntica a t1
    guardar("test", "p1_jpg.rf.c2", semilla=60)                       # foto nueva
    (export / "data.yaml").write_text(yaml.safe_dump({"names": ["garbage"]}))

    destino, manifiesto = tmp_path / "urbia", tmp_path / "m.csv"
    registro = tmp_path / "duplicados.csv"
    resumen = preparar({"garbage": (export, ["garbage"], clases.MAPEO_FUENTES["garbage"])},
                       destino, clases.IDS, manifiesto, split_original=True,
                       registro_duplicados=registro)

    assert dict(resumen.por_split) == {"train": 4, "val": 1, "test": 1}
    assert resumen.cajas_por_clase[1] == 6
    assert sorted((d.split, d.tipo) for d in resumen.duplicados_quitados) == [
        ("test", "casi idéntico"), ("val", "idéntico")]
    assert len(registro.read_text(encoding="utf-8").splitlines()) == 3   # encabezado + 2

    rep = validar_dataset(destino, clases.NUM_CLASES, manifiesto)
    assert rep.ok, rep.errores
    assert rep.advertencias == []