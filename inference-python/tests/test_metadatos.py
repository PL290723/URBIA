from urbia_vision.clases import NOMBRES
from urbia_vision.hardware import InfoHardware, recomendar
from urbia_vision.metadatos import cargar, guardar, sha256_archivo, validar_metadatos


def meta_valida() -> dict:
    return {
        "nombre": "urbia-yolo26s",
        "version": "1.0.0",
        "archivo": "models/urbia-yolo26s-v1.0.0.pt",
        "sha256": "a" * 64,
        "clases": {"0": "bache", "1": "basura"},
        "imgsz": 640,
        "umbral_confianza": 0.35,
        "modelo_base": "yolo26s.pt",
    }


def test_metadatos_validos():
    assert validar_metadatos(meta_valida(), NOMBRES) == []


def test_detecta_campos_y_valores_invalidos():
    m = meta_valida()
    m.update(version="v1", umbral_confianza=1.2, sha256="xyz",
             clases={"0": "basura", "1": "bache"}, nombre="x" * 61)
    errores = "\n".join(validar_metadatos(m, NOMBRES))
    for fragmento in ("X.Y.Z", "umbral_confianza", "sha256", "no coinciden", "60 caracteres"):
        assert fragmento in errores


def test_falta_campo():
    m = meta_valida()
    del m["umbral_confianza"]
    assert validar_metadatos(m, NOMBRES) == ["falta el campo 'umbral_confianza'"]


def test_guardar_y_cargar(tmp_path):
    ruta = tmp_path / "config" / "modelo.json"
    guardar(meta_valida(), ruta)
    assert cargar(ruta) == meta_valida()
    assert len(sha256_archivo(ruta)) == 64


def _hw(cuda: bool, vram: float | None) -> InfoHardware:
    return InfoHardware("Windows 11", "3.11", "2.x", "8.4", cuda, "GPU" if cuda else None, vram, 8)


def test_recomendacion_segun_hardware():
    grande = recomendar(_hw(True, 8.0))
    assert (grande.modelo_base, grande.batch) == ("yolo26s.pt", -1)
    rtx_4gb = recomendar(_hw(True, 4.0))          # p. ej. RTX 3050 Ti Laptop
    assert (rtx_4gb.modelo_base, rtx_4gb.device, rtx_4gb.batch) == ("yolo26s.pt", "0", 8)
    chica = recomendar(_hw(True, 2.0))
    assert (chica.modelo_base, chica.batch) == ("yolo26n.pt", 8)
    cpu = recomendar(_hw(False, None))
    assert (cpu.modelo_base, cpu.device, cpu.batch) == ("yolo26n.pt", "cpu", 8)