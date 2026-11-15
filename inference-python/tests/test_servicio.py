"""
Pruebas de carga única, inferencia y manejo de errores (HU 05.3).

Usan un YOLO falso para no depender de los pesos reales ni de la GPU.
La última prueba usa el modelo REAL si sus pesos están en models/.
"""

import io
import json
import threading
from pathlib import Path

import pytest
from PIL import Image

from urbia_vision import modelo as modulo_modelo
from urbia_vision import servicio as modulo_servicio
from urbia_vision.configuracion import RAIZ_SERVICIO, Configuracion
from urbia_vision.errores import ModeloInvalido, ModeloNoDisponible
from urbia_vision.metadatos import sha256_archivo
from urbia_vision.modelo import DetectorUrbia

NOMBRE = "abcdefghijklmnopqrstuvwxyz"


# ------------------------------------------------------------- YOLO falso

class _Lista(list):
    def tolist(self):
        return list(self)


class _Cajas:
    def __init__(self, cajas):
        self.xyxy = _Lista([c[:4] for c in cajas])
        self.cls = _Lista([c[4] for c in cajas])
        self.conf = _Lista([c[5] for c in cajas])


class _Resultado:
    def __init__(self, cajas):
        self.boxes = _Cajas(cajas)


class YoloFalso:
    instancias = 0

    def __init__(self, ruta, nombres=None, cajas=None, falla=False):
        YoloFalso.instancias += 1
        self.names = nombres or {0: "bache", 1: "basura"}
        # (x1, y1, x2, y2, clase, confianza) en coordenadas de la imagen preparada
        self.cajas = cajas if cajas is not None else [(10, 20, 110, 220, 1, 0.9), (0, 0, 50, 50, 0, 0.4)]
        self.falla = falla
        self.llamadas = []

    def predict(self, source, conf, imgsz, device, verbose):
        self.llamadas.append(source.shape)
        if self.falla and source.shape[0] != imgsz:  # el calentamiento sí funciona
            raise RuntimeError("CUDA out of memory")
        return [_Resultado([c for c in self.cajas if c[5] >= conf])]


@pytest.fixture
def entorno(tmp_path, monkeypatch):
    """modelo.json + pesos falsos + STORAGE_DIR con una foto."""
    pesos = tmp_path / "models" / "urbia-yolo26s-v1.0.0.pt"
    pesos.parent.mkdir()
    pesos.write_bytes(b"pesos de prueba")
    meta = {
        "nombre": "urbia-yolo26s", "version": "1.0.0", "archivo": str(pesos),
        "sha256": sha256_archivo(pesos), "clases": {"0": "bache", "1": "basura"},
        "imgsz": 640, "umbral_confianza": 0.25, "modelo_base": "yolo26s.pt",
    }
    config_json = tmp_path / "modelo.json"
    config_json.write_text(json.dumps(meta), encoding="utf-8")

    storage = tmp_path / "storage"
    (storage / "originales" / "2026" / "11").mkdir(parents=True)
    foto = io.BytesIO()
    Image.new("RGB", (2560, 1920), (90, 90, 90)).save(foto, format="JPEG")
    (storage / "originales" / "2026" / "11" / f"{NOMBRE}.jpg").write_bytes(foto.getvalue())

    YoloFalso.instancias = 0
    monkeypatch.setattr(modulo_modelo, "_crear_yolo", lambda ruta: YoloFalso(ruta))
    modulo_servicio._reiniciar_para_pruebas()
    yield Configuracion(modelo_config=config_json, storage_dir=storage, device="cpu"), meta, pesos
    modulo_servicio._reiniciar_para_pruebas()


RUTA = f"originales/2026/11/{NOMBRE}.jpg"


# ------------------------------------------------------------------ carga

def test_el_modelo_se_carga_una_sola_vez(entorno):
    config, _, _ = entorno
    s1 = modulo_servicio.obtener_servicio(config)
    resultados = [modulo_servicio.obtener_servicio(config).inferir_archivo(RUTA) for _ in range(5)]
    assert modulo_servicio.obtener_servicio(config) is s1
    assert YoloFalso.instancias == 1
    assert all(r.resultado == "detectado" for r in resultados)
    assert s1.estado()["peticiones"] == 5


def test_carga_unica_con_varios_hilos(entorno):
    config, _, _ = entorno
    servicios = []
    hilos = [threading.Thread(target=lambda: servicios.append(modulo_servicio.obtener_servicio(config)))
             for _ in range(8)]
    for h in hilos:
        h.start()
    for h in hilos:
        h.join()
    assert YoloFalso.instancias == 1
    assert all(s is servicios[0] for s in servicios)


def test_calentamiento_al_cargar(entorno):
    config, _, _ = entorno
    s = modulo_servicio.obtener_servicio(config)
    assert s.detector._yolo.llamadas[0] == (640, 640, 3)


def test_info_del_modelo(entorno):
    config, meta, _ = entorno
    info = modulo_servicio.obtener_servicio(config).detector.info
    assert (info.nombre, info.version, info.umbral_confianza, info.imgsz, info.dispositivo) == \
        ("urbia-yolo26s", "1.0.0", 0.25, 640, "cpu")
    assert info.sha256 == meta["sha256"]


def test_faltan_pesos(entorno):
    config, _, pesos = entorno
    pesos.unlink()
    with pytest.raises(ModeloNoDisponible, match="descargar_modelo"):
        modulo_servicio.obtener_servicio(config)


def test_falta_modelo_json(entorno, tmp_path):
    config, _, _ = entorno
    config = Configuracion(modelo_config=tmp_path / "no.json", storage_dir=config.storage_dir, device="cpu")
    with pytest.raises(ModeloNoDisponible):
        modulo_servicio.obtener_servicio(config)


def test_sha256_distinto(entorno):
    config, _, pesos = entorno
    pesos.write_bytes(b"otros pesos")
    with pytest.raises(ModeloInvalido, match="SHA-256"):
        modulo_servicio.obtener_servicio(config)


def test_clases_distintas(entorno, monkeypatch):
    config, _, _ = entorno
    monkeypatch.setattr(modulo_modelo, "_crear_yolo",
                        lambda r: YoloFalso(r, nombres={0: "basura", 1: "bache"}))
    with pytest.raises(ModeloInvalido, match="clases"):
        modulo_servicio.obtener_servicio(config)


def test_modelo_json_invalido(entorno):
    config, meta, _ = entorno
    meta["version"] = "v1"
    config.modelo_config.write_text(json.dumps(meta), encoding="utf-8")
    with pytest.raises(ModeloInvalido, match="X.Y.Z"):
        modulo_servicio.obtener_servicio(config)


def test_si_falla_la_carga_se_puede_reintentar(entorno):
    config, _, pesos = entorno
    contenido = pesos.read_bytes()
    pesos.unlink()
    with pytest.raises(ModeloNoDisponible):
        modulo_servicio.obtener_servicio(config)
    pesos.write_bytes(contenido)
    assert modulo_servicio.obtener_servicio(config).detector.info.version == "1.0.0"


# ------------------------------------------------------------- inferencia

def test_resultado_estructurado_y_cajas_en_pixeles_originales(entorno):
    config, _, _ = entorno
    r = modulo_servicio.obtener_servicio(config).inferir_archivo(RUTA)
    assert (r.resultado, r.clase_detectada, r.confianza) == ("detectado", "basura", 0.9)
    assert (r.modelo, r.version_modelo, r.umbral_confianza) == ("urbia-yolo26s", "1.0.0", 0.25)
    assert (r.ancho, r.alto) == (2560, 1920)
    # La foto de 2560 px se redujo a 1280 (escala 0.5): las cajas vuelven x2
    assert [c.clase for c in r.cajas] == ["basura", "bache"]
    assert (r.cajas[0].x1, r.cajas[0].y1, r.cajas[0].x2, r.cajas[0].y2) == (20.0, 40.0, 220.0, 440.0)
    assert r.codigo_error is None and r.milisegundos > 0


def test_sin_cajas_es_no_detectado(entorno, monkeypatch):
    config, _, _ = entorno
    monkeypatch.setattr(modulo_modelo, "_crear_yolo", lambda r: YoloFalso(r, cajas=[(0, 0, 9, 9, 0, 0.1)]))
    r = modulo_servicio.obtener_servicio(config).inferir_archivo(RUTA)
    assert (r.resultado, r.clase_detectada, r.confianza, r.cajas) == ("no_detectado", None, None, [])


@pytest.mark.parametrize("ruta, codigo", [
    ("../../etc/passwd", "RUTA_INVALIDA"),
    ("originales/2026/11/" + "z" * 26 + ".jpg", "IMAGEN_NO_ENCONTRADA"),
])
def test_errores_de_ruta_no_tumban_el_servicio(entorno, ruta, codigo):
    config, _, _ = entorno
    s = modulo_servicio.obtener_servicio(config)
    malo = s.inferir_archivo(ruta)
    assert (malo.resultado, malo.codigo_error) == ("error", codigo)
    assert malo.modelo == "urbia-yolo26s"
    assert s.inferir_archivo(RUTA).resultado == "detectado"   # sigue funcionando
    assert s.estado() | {"modelo": None} == {"listo": True, "modelo": None, "peticiones": 2, "errores": 1}


def test_imagen_invalida_no_tumba_el_servicio(entorno):
    config, _, _ = entorno
    s = modulo_servicio.obtener_servicio(config)
    r = s.inferir_bytes(b"no es imagen")
    assert (r.resultado, r.codigo_error) == ("error", "IMAGEN_INVALIDA")
    assert s.inferir_archivo(RUTA).resultado == "detectado"


def test_fallo_del_modelo_no_tumba_el_servicio(entorno, monkeypatch):
    config, _, _ = entorno
    monkeypatch.setattr(modulo_modelo, "_crear_yolo", lambda r: YoloFalso(r, falla=True))
    s = modulo_servicio.obtener_servicio(config)
    r = s.inferir_archivo(RUTA)
    assert (r.resultado, r.codigo_error) == ("error", "FALLO_INFERENCIA")
    assert (r.ancho, r.alto) == (2560, 1920)
    assert s.estado()["errores"] == 1


def test_resultado_serializable(entorno):
    config, _, _ = entorno
    d = modulo_servicio.obtener_servicio(config).inferir_archivo(RUTA).como_dict()
    assert json.loads(json.dumps(d))["cajas"][0]["clase"] == "basura"


# ---------------------------------------------------------- modelo real

MODELO_REAL = RAIZ_SERVICIO / "config" / "modelo.json"


def _pesos_reales() -> Path | None:
    if not MODELO_REAL.is_file():
        return None
    p = RAIZ_SERVICIO / json.loads(MODELO_REAL.read_text(encoding="utf-8"))["archivo"]
    return p if p.is_file() else None


@pytest.mark.skipif(_pesos_reales() is None, reason="pesos reales no descargados (scripts.descargar_modelo)")
def test_modelo_real_carga_una_vez_y_procesa_varias_imagenes():
    detector = DetectorUrbia.cargar(MODELO_REAL, "auto")
    config = Configuracion(modelo_config=MODELO_REAL, storage_dir=RAIZ_SERVICIO, device="auto")
    s = modulo_servicio.ServicioInferencia(config, detector)
    for color, tamano in (((80, 80, 80), (640, 480)), ((200, 200, 200), (1920, 1080)), ((30, 60, 90), (500, 500))):
        buf = io.BytesIO()
        Image.new("RGB", tamano, color).save(buf, format="JPEG")
        r = s.inferir_bytes(buf.getvalue())
        assert r.resultado in ("detectado", "no_detectado")
        assert (r.ancho, r.alto) == tamano
    assert s.estado()["peticiones"] == 3 and s.estado()["errores"] == 0