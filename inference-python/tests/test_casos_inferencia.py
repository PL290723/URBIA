"""
Pruebas del servicio de inferencia por caso de uso (HU 09.3).

Cada sección corresponde a una subtarea de la historia y prueba la cadena
COMPLETA tal como la usa Go:

    gRPC Detectar(ruta) -> LectorImagenes (STORAGE_DIR) -> preprocesamiento
        -> DetectorUrbia (cargado con DetectorUrbia.cargar) -> regla de decisión

Lo único simulado es YOLO: un modelo falso al que cada prueba le dice qué
cajas devolver. Así se prueban casos que con el modelo real no se pueden
forzar (una confianza de exactamente 0.25, un modelo que "ve" de más, etc.)
sin depender de los pesos ni de la GPU.

    1. Imagen válida                 4. Ausencia de detección
    2. Archivo corrupto / no imagen  5. Confianza por debajo del umbral
    3. Detección concluyente         6. Modelo o pesos ausentes
    7. Resultados de evaluación registrados (config/modelo.json y docs)
"""

from __future__ import annotations

import io
import json
import re
import urllib.request
from pathlib import Path
from types import SimpleNamespace

import grpc
import pytest
from PIL import Image

from urbia.inferencia.v1 import inferencia_pb2 as pb
from urbia.inferencia.v1 import inferencia_pb2_grpc as pb_grpc
from urbia_vision import contrato_grpc
from urbia_vision import modelo as modulo_modelo
from urbia_vision.configuracion import Configuracion
from urbia_vision.configuracion_servidor import ConfigServidor
from urbia_vision.errores import ModeloInvalido, ModeloNoDisponible
from urbia_vision.metadatos import sha256_archivo
from urbia_vision.modelo import DetectorUrbia
from urbia_vision.servicio import ServicioInferencia
from urbia_vision.servidor import ServidorInferencia

RAIZ = Path(__file__).resolve().parents[1]
UMBRAL = 0.25


# =============================================================== utilidades

class _Lista(list):
    def tolist(self):
        return list(self)


class YoloControlable:
    """
    YOLO falso. `cajas` son (x1, y1, x2, y2, id_clase, confianza) en píxeles de
    la imagen que recibe el modelo. Como Ultralytics, descarta las cajas con
    confianza < conf, salvo que `ignora_conf=True` (simula un modelo que
    devolviera de más, para probar que la regla de decisión también filtra).
    """

    def __init__(self, nombres=None):
        self.names = nombres or {0: "bache", 1: "basura"}
        self.cajas: list[tuple] = []
        self.ignora_conf = False
        self.recibidas: list[tuple] = []   # forma (alto, ancho, canales) de cada imagen

    def predict(self, source, conf, imgsz, device, verbose):
        self.recibidas.append(source.shape)
        cajas = self.cajas if self.ignora_conf else [c for c in self.cajas if c[5] >= conf]
        boxes = SimpleNamespace(xyxy=_Lista([list(c[:4]) for c in cajas]),
                                cls=_Lista([c[4] for c in cajas]),
                                conf=_Lista([c[5] for c in cajas]))
        return [SimpleNamespace(boxes=boxes)]


def nombre_archivo(letra: str) -> str:
    """Nombre como los que genera Go: 26 caracteres base32 en minúsculas."""
    return letra * 26


def imagen(formato: str, tamano=(1600, 1200), modo="RGB", color=(120, 120, 120)) -> bytes:
    buf = io.BytesIO()
    Image.new(modo, tamano, color).save(buf, format=formato)
    return buf.getvalue()


class Almacen:
    """STORAGE_DIR temporal con la misma estructura que usa Go."""

    def __init__(self, raiz: Path):
        self.raiz = raiz
        self.carpeta = raiz / "originales" / "2026" / "11"
        self.carpeta.mkdir(parents=True)

    def guardar(self, letra: str, ext: str, datos: bytes) -> str:
        nombre = f"{nombre_archivo(letra)}.{ext}"
        (self.carpeta / nombre).write_bytes(datos)
        return f"originales/2026/11/{nombre}"


def escribir_modelo(carpeta: Path, **cambios) -> tuple[Path, Path]:
    """modelo.json + pesos falsos con su SHA-256 correcto."""
    pesos = carpeta / "models" / "urbia-yolo26s-v1.0.0.pt"
    pesos.parent.mkdir(parents=True, exist_ok=True)
    pesos.write_bytes(b"pesos de prueba HU 09.3")
    meta = {
        "nombre": "urbia-yolo26s", "version": "1.0.0", "archivo": str(pesos),
        "sha256": sha256_archivo(pesos), "clases": {"0": "bache", "1": "basura"},
        "imgsz": 640, "umbral_confianza": UMBRAL, "modelo_base": "yolo26s.pt",
    }
    meta.update(cambios)
    ruta = carpeta / "modelo.json"
    ruta.write_text(json.dumps(meta), encoding="utf-8")
    return ruta, pesos


def solicitud(ruta: str, **extra) -> pb.DetectarRequest:
    return pb.DetectarRequest(id_solicitud="hu093", id_reporte=1, id_archivo=1, ruta_archivo=ruta, **extra)


def error_grpc(llamada) -> tuple[grpc.StatusCode, str | None]:
    with pytest.raises(grpc.RpcError) as info:
        llamada()
    e = info.value
    return e.code(), dict(e.trailing_metadata() or ()).get(contrato_grpc.TRAILER_CODIGO)


@pytest.fixture
def yolo(monkeypatch):
    falso = YoloControlable()
    monkeypatch.setattr(modulo_modelo, "_crear_yolo", lambda ruta: falso)
    return falso


@pytest.fixture
def entorno(tmp_path, yolo):
    """
    Servidor gRPC + health REALES, con el detector cargado por
    DetectorUrbia.cargar (valida modelo.json, SHA-256 y clases) y YOLO falso.
    """
    modelo_json, _ = escribir_modelo(tmp_path)
    almacen = Almacen(tmp_path / "storage")
    config = Configuracion(modelo_config=modelo_json, storage_dir=almacen.raiz, device="cpu")
    servicio = ServicioInferencia(config, DetectorUrbia.cargar(modelo_json, "cpu"))
    yolo.recibidas.clear()  # olvida la inferencia de calentamiento

    servidor = ServidorInferencia(
        ConfigServidor(direccion_grpc="127.0.0.1:0", direccion_salud="127.0.0.1:0",
                       hilos=2, timeout_inferencia=5.0, gracia_apagado=1.0),
        obtener=lambda: servicio,
    )
    servidor.iniciar()
    canal = grpc.insecure_channel(f"127.0.0.1:{servidor.puerto_grpc}")
    stub = pb_grpc.InferenciaServiceStub(canal)
    yield SimpleNamespace(stub=stub, almacen=almacen, servicio=servicio, yolo=yolo,
                          salud=f"http://127.0.0.1:{servidor.salud.puerto}/health")
    canal.close()
    servidor.apagar()


def detectar(e, ruta: str) -> pb.DetectarResponse:
    return e.stub.Detectar(solicitud(ruta), timeout=5)


def salud(e) -> dict:
    with urllib.request.urlopen(e.salud, timeout=5) as r:
        return json.loads(r.read())


# ======================================================== 1. imagen válida

@pytest.mark.parametrize("formato, ext, modo", [
    ("JPEG", "jpg", "RGB"),
    ("PNG", "png", "RGB"),
    ("PNG", "png", "RGBA"),   # con transparencia
    ("WEBP", "webp", "RGB"),
])
def test_imagen_valida_en_cada_formato_permitido(entorno, formato, ext, modo):
    entorno.yolo.cajas = [(100, 100, 300, 300, 0, 0.8)]
    ruta = entorno.almacen.guardar("a", ext, imagen(formato, (1600, 1200), modo))

    r = detectar(entorno, ruta)

    assert r.resultado == pb.RESULTADO_DETECTADO
    assert r.id_solicitud == "hu093"
    assert (r.ancho, r.alto) == (1600, 1200)
    assert (r.modelo.nombre, r.modelo.version) == ("urbia-yolo26s", "1.0.0")
    assert r.modelo.umbral_confianza == pytest.approx(UMBRAL)
    assert r.milisegundos >= 0
    # Al modelo le llega RGB->BGR de 3 canales, reducido a 1280 px por el lado mayor
    assert entorno.yolo.recibidas[-1] == (960, 1280, 3)


def test_imagen_valida_cajas_en_pixeles_de_la_foto_original(entorno):
    entorno.yolo.cajas = [(100, 50, 400, 300, 1, 0.9)]
    ruta = entorno.almacen.guardar("b", "jpg", imagen("JPEG", (2560, 1920)))

    caja = detectar(entorno, ruta).cajas[0]

    # La foto se redujo a la mitad (2560 -> 1280); las cajas regresan multiplicadas por 2
    assert (caja.x1, caja.y1, caja.x2, caja.y2) == (200, 100, 800, 600)


def test_imagen_valida_pequena_no_se_agranda(entorno):
    entorno.yolo.cajas = []
    ruta = entorno.almacen.guardar("c", "png", imagen("PNG", (640, 480)))
    r = detectar(entorno, ruta)
    assert r.resultado == pb.RESULTADO_NO_DETECTADO
    assert entorno.yolo.recibidas[-1] == (480, 640, 3)


# =========================================== 2. archivo corrupto / no imagen

def _jpeg_truncado() -> bytes:
    completo = imagen("JPEG", (1600, 1200))
    return completo[: len(completo) // 2]


CORRUPTOS = {
    "texto con extensión jpg": ("d", "jpg", b"esto no es una imagen, es texto plano\n"),
    "HTML disfrazado de png": ("e", "png", b"<!DOCTYPE html><script>alert(1)</script>"),
    "GIF real con extensión png": ("f", "png", imagen("GIF", (400, 400), "P")),
    "JPEG truncado": ("g", "jpg", _jpeg_truncado()),
    "cabecera PNG sin datos": ("h", "png", b"\x89PNG\r\n\x1a\n" + b"\x00" * 64),
    "JPEG de 100x100 px (muy chico)": ("i", "jpg", imagen("JPEG", (100, 100))),
}


@pytest.mark.parametrize("caso", list(CORRUPTOS))
def test_archivo_corrupto_o_no_imagen_es_imagen_invalida(entorno, caso):
    letra, ext, datos = CORRUPTOS[caso]
    ruta = entorno.almacen.guardar(letra, ext, datos)

    codigo, urbia = error_grpc(lambda: detectar(entorno, ruta))

    assert (codigo, urbia) == (grpc.StatusCode.INVALID_ARGUMENT, "IMAGEN_INVALIDA")
    assert entorno.yolo.recibidas == [], "un archivo inválido nunca debe llegar al modelo"


def test_tras_archivos_corruptos_el_servicio_sigue_atendiendo(entorno):
    entorno.yolo.cajas = [(0, 0, 100, 100, 1, 0.7)]
    for letra, ext, datos in CORRUPTOS.values():
        ruta = entorno.almacen.guardar(letra, ext, datos)
        error_grpc(lambda: detectar(entorno, ruta))

    valida = entorno.almacen.guardar("k", "jpg", imagen("JPEG"))
    assert detectar(entorno, valida).resultado == pb.RESULTADO_DETECTADO

    inferencias = salud(entorno)["inferencias"]
    assert inferencias == {"peticiones": len(CORRUPTOS) + 1, "errores": len(CORRUPTOS)}


def test_archivo_vacio_es_imagen_no_encontrada(entorno):
    # Go escribe la foto en un .tmp y la renombra al terminar: un archivo vacío
    # significa que la escritura no se completó, igual que si no existiera.
    ruta = entorno.almacen.guardar("j", "jpg", b"")
    assert error_grpc(lambda: detectar(entorno, ruta)) == (grpc.StatusCode.NOT_FOUND, "IMAGEN_NO_ENCONTRADA")
    assert entorno.yolo.recibidas == []


def test_archivo_inexistente_y_ruta_maliciosa(entorno):
    falta = f"originales/2026/11/{nombre_archivo('z')}.jpg"
    assert error_grpc(lambda: detectar(entorno, falta)) == (grpc.StatusCode.NOT_FOUND, "IMAGEN_NO_ENCONTRADA")
    for ruta in ("../../etc/passwd", "originales/2026/11/../../../modelo.json", "C:\\Windows\\win.ini"):
        assert error_grpc(lambda: detectar(entorno, ruta))[0] == grpc.StatusCode.INVALID_ARGUMENT


# ===================================================== 3. detección concluyente

def test_deteccion_concluyente_de_bache(entorno):
    entorno.yolo.cajas = [(10, 10, 200, 200, 0, 0.8734)]
    r = detectar(entorno, entorno.almacen.guardar("l", "jpg", imagen("JPEG", (1280, 960))))

    assert r.resultado == pb.RESULTADO_DETECTADO
    assert r.clase_detectada == "bache"
    assert r.confianza == pytest.approx(0.8734, abs=1e-4)
    assert [c.clase for c in r.cajas] == ["bache"]


def test_con_varias_clases_gana_la_caja_mas_confiable(entorno):
    entorno.yolo.cajas = [
        (0, 0, 100, 100, 0, 0.55),    # bache
        (200, 200, 400, 400, 1, 0.91),  # basura  <- la más confiable
        (500, 500, 600, 600, 0, 0.60),  # bache
    ]
    r = detectar(entorno, entorno.almacen.guardar("m", "jpg", imagen("JPEG", (1280, 960))))

    assert (r.clase_detectada, round(r.confianza, 2)) == ("basura", 0.91)
    assert [round(c.confianza, 2) for c in r.cajas] == [0.91, 0.60, 0.55]   # ordenadas


def test_confianza_con_4_decimales_como_la_bd(entorno):
    entorno.yolo.cajas = [(0, 0, 100, 100, 1, 0.876543219)]
    r = detectar(entorno, entorno.almacen.guardar("n", "jpg", imagen("JPEG", (1280, 960))))
    assert r.confianza == pytest.approx(0.8765, abs=1e-6)   # deteccion.confianza NUMERIC(5,4)


def test_omitir_cajas_y_maximo_de_cajas(entorno):
    entorno.yolo.cajas = [(0, 0, 50, 50, 0, 0.3 + i / 100) for i in range(10)]
    ruta = entorno.almacen.guardar("o", "jpg", imagen("JPEG", (1280, 960)))

    r = entorno.stub.Detectar(solicitud(ruta, opciones=pb.OpcionesDeteccion(max_cajas=3)), timeout=5)
    assert len(r.cajas) == 3 and r.cajas[0].confianza == pytest.approx(0.39, abs=1e-4)

    r = entorno.stub.Detectar(solicitud(ruta, opciones=pb.OpcionesDeteccion(omitir_cajas=True)), timeout=5)
    assert r.resultado == pb.RESULTADO_DETECTADO and len(r.cajas) == 0


# ===================================================== 4. ausencia de detección

def test_sin_cajas_es_no_detectado(entorno):
    entorno.yolo.cajas = []
    r = detectar(entorno, entorno.almacen.guardar("p", "jpg", imagen("JPEG")))

    assert r.resultado == pb.RESULTADO_NO_DETECTADO
    assert r.clase_detectada == ""          # proto3: vacío = NULL en la BD
    assert r.confianza == 0.0
    assert len(r.cajas) == 0
    # Aun sin detección se informa con qué modelo se decidió (deteccion.modelo)
    assert (r.modelo.nombre, r.modelo.version) == ("urbia-yolo26s", "1.0.0")


def test_no_detectado_no_cuenta_como_error(entorno):
    entorno.yolo.cajas = []
    for letra in "qrs":
        detectar(entorno, entorno.almacen.guardar(letra, "jpg", imagen("JPEG")))
    assert salud(entorno)["inferencias"] == {"peticiones": 3, "errores": 0}


# ============================================ 5. confianza por debajo del umbral

@pytest.mark.parametrize("confianzas, esperado", [
    ([0.24], pb.RESULTADO_NO_DETECTADO),
    ([0.10, 0.20, 0.2499], pb.RESULTADO_NO_DETECTADO),
    ([0.25], pb.RESULTADO_DETECTADO),       # el umbral es inclusivo
    ([0.2501], pb.RESULTADO_DETECTADO),
])
def test_umbral_de_confianza(entorno, confianzas, esperado):
    entorno.yolo.cajas = [(0, 0, 100, 100, 0, c) for c in confianzas]
    r = detectar(entorno, entorno.almacen.guardar("t", "jpg", imagen("JPEG")))
    assert r.resultado == esperado


def test_bajo_el_umbral_no_se_sugiere_aunque_el_modelo_lo_devuelva(entorno):
    # Defensa en profundidad: aunque YOLO ignorara `conf`, la regla de decisión filtra.
    entorno.yolo.ignora_conf = True
    entorno.yolo.cajas = [(0, 0, 100, 100, 1, 0.24), (0, 0, 80, 80, 0, 0.05)]
    r = detectar(entorno, entorno.almacen.guardar("u", "jpg", imagen("JPEG")))
    assert r.resultado == pb.RESULTADO_NO_DETECTADO
    assert r.clase_detectada == ""


def test_solo_cuentan_las_cajas_sobre_el_umbral(entorno):
    entorno.yolo.ignora_conf = True
    entorno.yolo.cajas = [(0, 0, 100, 100, 1, 0.249), (0, 0, 80, 80, 0, 0.31)]
    r = detectar(entorno, entorno.almacen.guardar("v", "jpg", imagen("JPEG")))
    # La basura de 0.249 no gana aunque sea "la más confiable" de las que no pasan
    assert (r.clase_detectada, round(r.confianza, 2)) == ("bache", 0.31)


def test_el_umbral_viene_de_modelo_json(tmp_path, yolo):
    modelo_json, _ = escribir_modelo(tmp_path, umbral_confianza=0.6)
    detector = DetectorUrbia.cargar(modelo_json, "cpu")
    yolo.ignora_conf = True
    yolo.cajas = [(0, 0, 100, 100, 0, 0.55)]

    almacen = Almacen(tmp_path / "storage")
    ruta = almacen.guardar("w", "jpg", imagen("JPEG"))
    servicio = ServicioInferencia(Configuracion(modelo_config=modelo_json, storage_dir=almacen.raiz,
                                                device="cpu"), detector)
    r = servicio.inferir_archivo(ruta)
    assert (r.resultado, r.umbral_confianza) == ("no_detectado", 0.6)


# ============================================== 6. modelo o pesos ausentes

def test_faltan_los_pesos(tmp_path, yolo):
    modelo_json, pesos = escribir_modelo(tmp_path)
    pesos.unlink()
    with pytest.raises(ModeloNoDisponible, match="descargar_modelo"):
        DetectorUrbia.cargar(modelo_json, "cpu")


def test_falta_modelo_json(tmp_path, yolo):
    with pytest.raises(ModeloNoDisponible, match="publicar_modelo"):
        DetectorUrbia.cargar(tmp_path / "no-existe.json", "cpu")


@pytest.mark.parametrize("descripcion, preparar, mensaje", [
    ("pesos cambiados (SHA-256 distinto)",
     lambda pesos, js: pesos.write_bytes(b"otros pesos"), "SHA-256"),
    ("modelo.json no es JSON",
     lambda pesos, js: js.write_text("{ esto no es json", encoding="utf-8"), "JSON"),
    ("modelo.json sin campos obligatorios",
     lambda pesos, js: js.write_text(json.dumps({"nombre": "x"}), encoding="utf-8"), "falta el campo"),
    ("umbral fuera de rango",
     lambda pesos, js: js.write_text(json.dumps({**json.loads(js.read_text()), "umbral_confianza": 1.5}),
                                     encoding="utf-8"), "umbral_confianza"),
])
def test_modelo_invalido_no_arranca(tmp_path, yolo, descripcion, preparar, mensaje):
    modelo_json, pesos = escribir_modelo(tmp_path)
    preparar(pesos, modelo_json)
    with pytest.raises(ModeloInvalido, match=mensaje):
        DetectorUrbia.cargar(modelo_json, "cpu")


def test_modelo_con_otras_clases_no_arranca(tmp_path, monkeypatch):
    otro = YoloControlable(nombres={0: "persona", 1: "auto"})
    monkeypatch.setattr(modulo_modelo, "_crear_yolo", lambda ruta: otro)
    modelo_json, _ = escribir_modelo(tmp_path)
    with pytest.raises(ModeloInvalido, match="clases"):
        DetectorUrbia.cargar(modelo_json, "cpu")


def test_pesos_corruptos_que_ultralytics_no_puede_abrir(tmp_path, monkeypatch):
    def falla(ruta):
        raise RuntimeError("PytorchStreamReader failed reading zip archive")
    monkeypatch.setattr(modulo_modelo, "_crear_yolo", falla)
    modelo_json, _ = escribir_modelo(tmp_path)
    with pytest.raises(ModeloInvalido, match="Ultralytics no pudo cargar"):
        DetectorUrbia.cargar(modelo_json, "cpu")


def test_sin_modelo_el_servidor_no_queda_escuchando(tmp_path, yolo):
    modelo_json, pesos = escribir_modelo(tmp_path)
    pesos.unlink()

    def obtener():
        return ServicioInferencia(Configuracion(modelo_config=modelo_json, storage_dir=tmp_path,
                                                device="cpu"),
                                  DetectorUrbia.cargar(modelo_json, "cpu"))

    servidor = ServidorInferencia(ConfigServidor(direccion_grpc="127.0.0.1:0",
                                                 direccion_salud="127.0.0.1:0"), obtener=obtener)
    with pytest.raises(ModeloNoDisponible):
        servidor.iniciar()
    assert servidor.puerto_grpc is None   # Go verá "IA no disponible" y guardará el reporte igual


def test_al_restaurar_los_pesos_vuelve_a_cargar(tmp_path, yolo):
    modelo_json, pesos = escribir_modelo(tmp_path)
    contenido = pesos.read_bytes()
    pesos.unlink()
    with pytest.raises(ModeloNoDisponible):
        DetectorUrbia.cargar(modelo_json, "cpu")
    pesos.write_bytes(contenido)
    assert DetectorUrbia.cargar(modelo_json, "cpu").info.version == "1.0.0"


# ================================= 7. resultados de evaluación registrados

MODELO_PUBLICADO = RAIZ / "config" / "modelo.json"
DOC_MODELO = RAIZ.parent / "docs" / "vision" / "modelo.md"
DOC_PRUEBAS = RAIZ.parent / "docs" / "vision" / "pruebas-inferencia.md"


@pytest.fixture(scope="module")
def publicado() -> dict:
    return json.loads(MODELO_PUBLICADO.read_text(encoding="utf-8"))


def test_modelo_publicado_registra_su_evaluacion(publicado):
    for campo in ("metricas_test", "umbral_confianza", "criterio_umbral", "dataset", "entrenamiento"):
        assert campo in publicado, f"modelo.json debe registrar '{campo}'"
    assert publicado["dataset"]["split_evaluacion"] == "test"
    assert publicado["dataset"]["imagenes_evaluacion"] > 0
    assert set(publicado["metricas_test"]["al_umbral"]) == {"bache", "basura"}


def test_metricas_al_umbral_son_consistentes(publicado):
    """Precisión, recall y F1 registrados salen de los TP/FP/FN registrados."""
    al_umbral = publicado["metricas_test"]["al_umbral"]
    for clase, m in al_umbral.items():
        precision = m["tp"] / (m["tp"] + m["fp"])
        recall = m["tp"] / (m["tp"] + m["fn"])
        f1 = 2 * precision * recall / (precision + recall)
        assert m["precision"] == pytest.approx(precision, abs=1e-4), clase
        assert m["recall"] == pytest.approx(recall, abs=1e-4), clase
        assert m["f1"] == pytest.approx(f1, abs=1e-4), clase
    f1_macro = sum(m["f1"] for m in al_umbral.values()) / len(al_umbral)
    assert publicado["metricas_test"]["f1_macro"] == pytest.approx(f1_macro, abs=1e-3)


def test_metricas_en_rango(publicado):
    def recorrer(d):
        for k, v in d.items():
            if isinstance(v, dict):
                yield from recorrer(v)
            elif k in ("precision", "recall", "mAP50", "mAP50_95", "f1"):
                yield k, v
    valores = list(recorrer(publicado["metricas_test"]))
    assert valores and all(0.0 <= v <= 1.0 for _, v in valores)
    assert 0.0 <= publicado["metricas_test"]["exactitud_por_foto"] <= 1.0
    assert 0.0 < publicado["umbral_confianza"] < 1.0


def test_documentacion_del_modelo_coincide_con_modelo_json(publicado):
    doc = DOC_MODELO.read_text(encoding="utf-8")
    assert publicado["sha256"] in doc, "modelo.md debe mostrar el SHA-256 real de los pesos"
    assert f"{publicado['umbral_confianza']:.2f}" in doc
    assert f"{publicado['metricas_test']['f1_macro']:.3f}" in doc
    assert f"{publicado['metricas_test']['exactitud_por_foto'] * 100:.1f} %" in doc


def test_registro_de_pruebas_de_inferencia(publicado):
    doc = DOC_PRUEBAS.read_text(encoding="utf-8")
    assert f"{publicado['nombre']} v{publicado['version']}" in doc
    for clase, m in publicado["metricas_test"]["al_umbral"].items():
        fila = re.search(rf"\|\s*{clase}\s*\|\s*{m['tp']}\s*\|\s*{m['fp']}\s*\|\s*{m['fn']}\s*\|", doc)
        assert fila, f"pruebas-inferencia.md debe registrar TP/FP/FN de {clase}"