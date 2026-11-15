import os

import pytest

from urbia_vision.almacen import LectorImagenes
from urbia_vision.errores import ImagenInvalida, ImagenNoEncontrada, RutaInvalida

NOMBRE = "abcdefghijklmnopqrstuvwxyz"  # 26 caracteres base32 válidos
RUTA = f"originales/2026/11/{NOMBRE}.jpg"


@pytest.fixture
def storage(tmp_path):
    raiz = tmp_path / "storage"
    (raiz / "originales" / "2026" / "11").mkdir(parents=True)
    (raiz / RUTA).write_bytes(b"\xff\xd8\xff contenido")
    return raiz


def test_lee_ruta_valida(storage):
    assert LectorImagenes(storage, 1024).leer(RUTA).startswith(b"\xff\xd8\xff")


@pytest.mark.parametrize("ruta", [
    "../../etc/passwd",
    "/etc/passwd",
    "C:/Windows/win.ini",
    "originales\\2026\\11\\" + NOMBRE + ".jpg",
    "originales/2026/11/../11/" + NOMBRE + ".jpg",
    "originales/2026/11/" + NOMBRE.upper() + ".jpg",
    "originales/2026/11/" + NOMBRE + ".gif",
    "originales/2026/11/corto.jpg",
    "otros/2026/11/" + NOMBRE + ".jpg",
    "",
])
def test_rechaza_rutas_fuera_del_patron(storage, ruta):
    with pytest.raises(RutaInvalida):
        LectorImagenes(storage, 1024).leer(ruta)


def test_ruta_no_texto(storage):
    with pytest.raises(RutaInvalida):
        LectorImagenes(storage, 1024).leer(None)


def test_imagen_inexistente(storage):
    with pytest.raises(ImagenNoEncontrada):
        LectorImagenes(storage, 1024).leer("originales/2026/11/" + "b" * 26 + ".jpg")


def test_directorio_en_lugar_de_archivo(storage):
    (storage / "procesadas" / "2026" / "11" / ("c" * 26 + ".jpg")).mkdir(parents=True)
    with pytest.raises(ImagenNoEncontrada):
        LectorImagenes(storage, 1024).leer("procesadas/2026/11/" + "c" * 26 + ".jpg")


def test_archivo_demasiado_grande(storage):
    with pytest.raises(ImagenInvalida):
        LectorImagenes(storage, 5).leer(RUTA)


def test_archivo_vacio(storage):
    (storage / RUTA).write_bytes(b"")
    with pytest.raises(ImagenNoEncontrada):
        LectorImagenes(storage, 1024).leer(RUTA)


def test_symlink_que_sale_del_storage(storage, tmp_path):
    secreto = tmp_path / "secreto.jpg"
    secreto.write_bytes(b"\xff\xd8\xff secreto")
    enlace = storage / "originales" / "2026" / "11" / ("d" * 26 + ".jpg")
    try:
        os.symlink(secreto, enlace)
    except (OSError, NotImplementedError):
        pytest.skip("este sistema no permite crear symlinks sin privilegios")
    with pytest.raises(RutaInvalida):
        LectorImagenes(storage, 1024).leer("originales/2026/11/" + "d" * 26 + ".jpg")