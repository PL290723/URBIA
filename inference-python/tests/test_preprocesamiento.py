import io

import pytest
from PIL import Image

from urbia_vision.errores import ImagenInvalida
from urbia_vision.preprocesamiento import LimitesImagen, formato_por_firma, preparar


def _imagen(formato="JPEG", tamano=(400, 300), modo="RGB", color=(200, 10, 10), exif=None):
    im = Image.new(modo, tamano, color)
    buf = io.BytesIO()
    extra = {"exif": exif} if exif is not None else {}
    im.save(buf, format=formato, **extra)
    return buf.getvalue()


@pytest.mark.parametrize("formato", ["JPEG", "PNG", "WEBP"])
def test_formatos_permitidos(formato):
    p = preparar(_imagen(formato))
    assert p.formato == formato
    assert p.arreglo.shape == (300, 400, 3)
    assert (p.ancho_original, p.alto_original, p.escala) == (400, 300, 1.0)


def test_salida_en_bgr():
    p = preparar(_imagen("PNG", color=(255, 0, 0)))   # rojo puro en RGB
    b, g, r = p.arreglo[0, 0]
    assert (int(b), int(g), int(r)) == (0, 0, 255)


def test_png_transparente_se_aplana_en_blanco():
    p = preparar(_imagen("PNG", modo="RGBA", color=(0, 0, 0, 0)))
    assert p.arreglo[0, 0].tolist() == [255, 255, 255]


def test_reduce_fotos_grandes_conservando_proporcion():
    p = preparar(_imagen(tamano=(4000, 3000)))
    assert p.arreglo.shape == (960, 1280, 3)
    assert p.escala == pytest.approx(0.32)
    assert (p.ancho_original, p.alto_original) == (4000, 3000)


def test_aplica_orientacion_exif():
    exif = Image.Exif()
    exif[0x0112] = 6  # girar 90° (foto de celular en vertical)
    p = preparar(_imagen(tamano=(400, 300), exif=exif.tobytes()))
    assert (p.ancho_original, p.alto_original) == (300, 400)
    assert p.arreglo.shape == (400, 300, 3)


@pytest.mark.parametrize("datos, fragmento", [
    (b"esto no es una imagen", "Formato no permitido"),
    (b"GIF89a" + b"\x00" * 50, "Formato no permitido"),
    (b"\xff\xd8\xff\xe0" + b"\x00" * 64, "dañada"),
])
def test_rechaza_contenido_invalido(datos, fragmento):
    with pytest.raises(ImagenInvalida, match=fragmento):
        preparar(datos)


def test_rechaza_jpeg_truncado():
    datos = _imagen(tamano=(800, 600))
    with pytest.raises(ImagenInvalida):
        preparar(datos[: len(datos) // 2])


def test_rechaza_firma_que_no_coincide():
    png_con_cabecera_jpeg = b"\xff\xd8\xff" + _imagen("PNG")[3:]
    with pytest.raises(ImagenInvalida):
        preparar(png_con_cabecera_jpeg)


@pytest.mark.parametrize("tamano", [(100, 300), (300, 150), (8001, 300)])
def test_rechaza_dimensiones_fuera_de_rango(tamano):
    with pytest.raises(ImagenInvalida, match="px"):
        preparar(_imagen("PNG", tamano=tamano))


def test_rechaza_demasiados_megapixeles():
    with pytest.raises(ImagenInvalida, match="MP"):
        preparar(_imagen("PNG", tamano=(1000, 1000)), LimitesImagen(max_pixeles=500_000))


def test_formato_por_firma():
    assert formato_por_firma(_imagen("JPEG")) == "JPEG"
    assert formato_por_firma(_imagen("PNG")) == "PNG"
    assert formato_por_firma(_imagen("WEBP")) == "WEBP"
    assert formato_por_firma(b"RIFF1234WAVE") is None


def test_no_usa_el_image_open_de_ultralytics(monkeypatch):
    """
    Ultralytics reemplaza PIL.Image.open y, ante un archivo que Pillow no
    reconoce, intenta instalar pi-heif con pip. El servicio no debe pasar por ahí.
    """
    pytest.importorskip("ultralytics")
    import ultralytics.utils.checks as checks
    import ultralytics.utils.patches  # noqa: F401  (aplica el parche a Image.open)

    intentos_de_instalar = []
    monkeypatch.setattr(checks, "check_requirements", lambda *a, **k: intentos_de_instalar.append(a))
    monkeypatch.setattr(ultralytics.utils.patches, "_pil_plugins_registered", False, raising=False)
    with pytest.raises(ImagenInvalida):
        preparar(b"\xff\xd8\xff\xe0" + b"\x00" * 64)
    assert intentos_de_instalar == [], "el preprocesamiento usó el Image.open de Ultralytics"
    assert preparar(_imagen("JPEG")).formato == "JPEG"


def test_blindar_ultralytics(monkeypatch):
    from urbia_vision.configuracion import blindar_ultralytics

    monkeypatch.delenv("YOLO_AUTOINSTALL", raising=False)
    monkeypatch.setenv("YOLO_OFFLINE", "False")  # un valor explícito se respeta
    blindar_ultralytics()
    import os
    assert os.environ["YOLO_AUTOINSTALL"] == "False"
    assert os.environ["YOLO_OFFLINE"] == "False"