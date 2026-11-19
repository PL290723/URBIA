"""
Pruebas del servidor de inferencia (HU 05.5).

Cubren las subtareas de la historia:
  1. Servidor gRPC en Python: arranque, health, apagado ordenado.
  2. Validar identificador/ruta ANTES de procesar.
  3. Ejecutar inferencia y normalizar resultados.
  4. Devolver resultados estructurados (de punta a punta con el modelo falso).
  5. Timeout, manejo de excepciones y logging por petición.
  6. El servicio no guarda nada ni toca el estado del reporte.

No necesitan los pesos reales ni la GPU: usan un YOLO falso.
"""

import hashlib
import io
import json
import logging
import re
import sys
import threading
import time
import urllib.error
import urllib.request
from pathlib import Path
from types import SimpleNamespace

import grpc
import pytest
from PIL import Image

from urbia.inferencia.v1 import inferencia_pb2 as pb
from urbia.inferencia.v1 import inferencia_pb2_grpc as pb_grpc
from urbia_vision import contrato_grpc, salud
from urbia_vision.configuracion import Configuracion
from urbia_vision.configuracion_servidor import ConfigServidor
from urbia_vision.errores import FalloInferencia, ModeloNoDisponible
from urbia_vision.modelo import Caja, DetectorUrbia, InfoModelo
from urbia_vision.normalizacion import normalizar
from urbia_vision.registro import InterceptorRegistro, MetricasGrpc, limpiar
from urbia_vision.servicio import ResultadoInferencia, ServicioInferencia
from urbia_vision.servidor import ServidorInferencia

NOMBRE = "abcdefghijklmnopqrstuvwxyz"
RUTA = f"originales/2026/11/{NOMBRE}.jpg"
RAIZ = Path(__file__).resolve().parents[1]


# ----------------------------------------------------------------- utilidades

def _resultado(resultado="detectado", **extra):
    base = dict(
        resultado=resultado, clase_detectada="basura", confianza=0.9123,
        modelo="urbia-yolo26s", version_modelo="1.0.0", umbral_confianza=0.25,
        cajas=[Caja("basura", 0.9123, 10, 20, 110, 220), Caja("bache", 0.41, 0, 0, 50, 50)],
        ancho=2560, alto=1920, milisegundos=180.54,
    )
    if resultado == "no_detectado":
        base.update(clase_detectada=None, confianza=None, cajas=[])
    base.update(extra)
    return ResultadoInferencia(**base)


class ServicioFalso:
    def __init__(self):
        self.respuesta = _resultado()
        self.rutas = []
        self.espera = 0.0
        self.excepcion = None
        self.detector = SimpleNamespace(info=SimpleNamespace(
            nombre="urbia-yolo26s", version="1.0.0", umbral_confianza=0.25,
            imgsz=640, dispositivo="cpu", sha256="a" * 64))

    def inferir_archivo(self, ruta):
        self.rutas.append(ruta)
        time.sleep(self.espera)
        if self.excepcion:
            raise self.excepcion
        return self.respuesta

    def estado(self):
        return {"listo": True, "peticiones": len(self.rutas), "errores": 0}


def _solicitud(**extra):
    datos = dict(id_solicitud="req-123", id_reporte=7, id_archivo=42, ruta_archivo=RUTA)
    datos.update(extra)
    return pb.DetectarRequest(**datos)


def _error(llamada):
    with pytest.raises(grpc.RpcError) as info:
        llamada()
    e = info.value
    return e.code(), dict(e.trailing_metadata() or ()).get(contrato_grpc.TRAILER_CODIGO), e.details()


def _config(**extra):
    datos = dict(direccion_grpc="127.0.0.1:0", direccion_salud="127.0.0.1:0",
                 hilos=4, timeout_inferencia=2.0, gracia_apagado=2.0)
    datos.update(extra)
    return ConfigServidor(**datos)


def _get(url):
    try:
        with urllib.request.urlopen(url, timeout=5) as r:
            return r.status, json.loads(r.read() or b"{}")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"{}")


@pytest.fixture
def servidor():
    """ServidorInferencia completo (health + gRPC + interceptor) con servicio falso."""
    servicio = ServicioFalso()
    s = ServidorInferencia(_config(), obtener=lambda: servicio)
    s.iniciar()
    canal = grpc.insecure_channel(f"127.0.0.1:{s.puerto_grpc}")
    yield s, servicio, pb_grpc.InferenciaServiceStub(canal), f"http://127.0.0.1:{s.salud.puerto}"
    canal.close()
    s.apagar()


# ========================================================= 0. configuración

def test_config_por_defecto(monkeypatch):
    for v in ("URBIA_GRPC_DIRECCION", "URBIA_SALUD_DIRECCION", "URBIA_GRPC_HILOS",
              "URBIA_TIMEOUT_INFERENCIA", "URBIA_APAGADO_GRACIA", "URBIA_LOG_NIVEL"):
        monkeypatch.delenv(v, raising=False)
    c = ConfigServidor.desde_entorno()
    assert (c.direccion_grpc, c.direccion_salud, c.hilos) == ("127.0.0.1:50051", "127.0.0.1:8000", 4)
    assert c.timeout_inferencia < 10   # menor que INFERENCE_GRPC_TIMEOUT de Go


def test_config_desde_entorno(monkeypatch):
    monkeypatch.setenv("URBIA_GRPC_DIRECCION", "0.0.0.0:50051")
    monkeypatch.setenv("URBIA_SALUD_DIRECCION", "0.0.0.0:8000")
    monkeypatch.setenv("URBIA_GRPC_HILOS", "2")
    monkeypatch.setenv("URBIA_TIMEOUT_INFERENCIA", "5.5")
    monkeypatch.setenv("URBIA_LOG_NIVEL", "debug")
    c = ConfigServidor.desde_entorno()
    assert (c.direccion_grpc, c.hilos, c.timeout_inferencia, c.nivel_log) == ("0.0.0.0:50051", 2, 5.5, "DEBUG")


@pytest.mark.parametrize("variable, valor", [
    ("URBIA_GRPC_DIRECCION", "50051"),
    ("URBIA_SALUD_DIRECCION", "localhost:99999"),
    ("URBIA_GRPC_HILOS", "0"),
    ("URBIA_TIMEOUT_INFERENCIA", "abc"),
    ("URBIA_LOG_NIVEL", "MUCHO"),
])
def test_config_invalida(monkeypatch, variable, valor):
    monkeypatch.setenv(variable, valor)
    with pytest.raises(ValueError, match=variable):
        ConfigServidor.desde_entorno()


# ===================================================== 1. servidor y health

def test_health_listo_con_modelo(servidor):
    _, _, _, url = servidor
    codigo, cuerpo = _get(url + "/health")
    assert codigo == 200
    assert cuerpo["estado"] == "listo"
    assert cuerpo["version_contrato"] == contrato_grpc.VERSION_CONTRATO
    assert cuerpo["modelo"] == {"nombre": "urbia-yolo26s", "version": "1.0.0",
                                "umbral_confianza": 0.25, "dispositivo": "cpu"}
    assert "sha256" not in json.dumps(cuerpo)   # no se expone más de lo necesario


def test_health_rutas_y_metodos(servidor):
    _, _, _, url = servidor
    assert _get(url + "/otra")[0] == 404
    peticion = urllib.request.Request(url + "/health", method="POST", data=b"")
    with pytest.raises(urllib.error.HTTPError) as e:
        urllib.request.urlopen(peticion, timeout=5)
    assert e.value.code == 405


def test_health_cuenta_peticiones_grpc(servidor):
    _, _, stub, url = servidor
    stub.Detectar(_solicitud(), timeout=5)
    _error(lambda: stub.Detectar(_solicitud(id_reporte=0), timeout=5))
    grpc_ = _get(url + "/health")[1]["grpc"]
    assert (grpc_["peticiones"], grpc_["errores"]) == (2, 1)
    assert grpc_["por_codigo"] == {"OK": 1, "INVALID_ARGUMENT": 1}


def test_health_503_mientras_carga_el_modelo():
    cargando, liberar = threading.Event(), threading.Event()

    def obtener_lento():
        cargando.set()
        liberar.wait(5)
        return ServicioFalso()

    s = ServidorInferencia(_config(), obtener=obtener_lento)
    hilo = threading.Thread(target=s.iniciar)
    hilo.start()
    try:
        assert cargando.wait(5)
        codigo, cuerpo = _get(f"http://127.0.0.1:{s.salud.puerto}/health")
        assert (codigo, cuerpo["estado"]) == (503, "iniciando")
    finally:
        liberar.set()
        hilo.join(5)
        s.apagar()


def test_si_el_modelo_no_carga_no_queda_nada_escuchando():
    def obtener_falla():
        raise ModeloNoDisponible("faltan pesos")

    s = ServidorInferencia(_config(), obtener=obtener_falla)
    with pytest.raises(ModeloNoDisponible):
        s.iniciar()
    puerto = s.salud.puerto
    with pytest.raises(urllib.error.URLError):
        urllib.request.urlopen(f"http://127.0.0.1:{puerto}/health", timeout=2)


def test_apagado_ordenado_termina_lo_que_esta_en_curso():
    servicio = ServicioFalso()
    servicio.espera = 0.5
    s = ServidorInferencia(_config(gracia_apagado=3), obtener=lambda: servicio)
    s.iniciar()
    url = f"http://127.0.0.1:{s.salud.puerto}/health"
    canal = grpc.insecure_channel(f"127.0.0.1:{s.puerto_grpc}")
    stub = pb_grpc.InferenciaServiceStub(canal)

    resultado = {}
    en_curso = threading.Thread(target=lambda: resultado.setdefault("r", stub.Detectar(_solicitud(), timeout=5)))
    en_curso.start()
    while not servicio.rutas:          # espera a que la petición esté adentro
        time.sleep(0.01)

    s.solicitar_apagado("prueba")
    assert _get(url)[0] == 503          # Go deja de considerarlo disponible de inmediato
    s.apagar()
    en_curso.join(5)
    assert resultado["r"].resultado == pb.RESULTADO_DETECTADO   # la petición en curso terminó bien

    # En Windows una conexión rechazada tarda ~2 s (reintentos de TCP): timeout amplio
    codigo, _, _ = _error(lambda: stub.Detectar(_solicitud(), timeout=10))
    assert codigo == grpc.StatusCode.UNAVAILABLE                # las nuevas se rechazan
    canal.close()


def test_apagar_dos_veces_no_falla(servidor):
    s, _, _, _ = servidor
    s.apagar()
    s.apagar()


# ============================================= 2. validar id y ruta antes

@pytest.mark.parametrize("ruta", [
    "../../etc/passwd",
    "/etc/passwd",
    "C:\\Windows\\win.ini",
    "originales\\2026\\11\\abcdefghijklmnopqrstuvwxyz.jpg",
    "originales/2026/11/../../../secreto.jpg",
    "originales/2026/11/ABCDEFGHIJKLMNOPQRSTUVWXYZ.jpg",
    "originales/2026/11/abcdefghijklmnopqrstuvwxyz.gif",
    "otra/2026/11/abcdefghijklmnopqrstuvwxyz.jpg",
    "originales/2026/11/" + "a" * 300 + ".jpg",
])
def test_ruta_invalida_no_llega_al_servicio(servidor, ruta):
    _, servicio, stub, _ = servidor
    codigo, urbia, _ = _error(lambda: stub.Detectar(_solicitud(ruta_archivo=ruta), timeout=5))
    assert (codigo, urbia) == (grpc.StatusCode.INVALID_ARGUMENT, "RUTA_INVALIDA")
    assert servicio.rutas == []    # ni se lee el disco ni se usa el modelo


@pytest.mark.parametrize("id_solicitud", ["x" * 129, "req\n2026-11-15 INFO falso", "req\x00"])
def test_id_solicitud_invalido(servidor, id_solicitud):
    _, servicio, stub, _ = servidor
    codigo, urbia, mensaje = _error(lambda: stub.Detectar(_solicitud(id_solicitud=id_solicitud), timeout=5))
    assert (codigo, urbia) == (grpc.StatusCode.INVALID_ARGUMENT, "SOLICITUD_INVALIDA")
    assert "id_solicitud" in mensaje
    assert servicio.rutas == []


def test_id_solicitud_de_go_es_valido(servidor):
    # X-Request-ID que genera el middleware de Go: 16 caracteres hexadecimales
    _, _, stub, _ = servidor
    assert stub.Detectar(_solicitud(id_solicitud="9f86d081884c7d65"), timeout=5).id_solicitud == "9f86d081884c7d65"


# ================================================ 3. normalizar resultados

def test_normalizar_recorta_redondea_y_ordena():
    r = normalizar(_resultado(confianza=0.912345, cajas=[
        Caja("bache", 0.41, -5, -3, 50.06, 50),            # se recorta a 0
        Caja("basura", 0.912345, 2500, 20, 2600, 220.04),   # x2 se recorta al ancho
        Caja("bache", 0.10, 0, 0, 10, 10),                 # bajo el umbral: fuera
        Caja("basura", 0.80, 100, 100, 100.4, 300),        # ancho < 1 px: fuera
    ]))
    assert r.confianza == 0.9123
    assert [(c.clase, c.confianza) for c in r.cajas] == [("basura", 0.9123), ("bache", 0.41)]
    assert (r.cajas[0].x2, r.cajas[0].y2) == (2560, 220.0)
    assert (r.cajas[1].x1, r.cajas[1].y1, r.cajas[1].x2) == (0, 0, 50.1)
    assert r.milisegundos == 180.5


def test_normalizar_no_detectado():
    r = normalizar(_resultado("no_detectado"))
    assert (r.resultado, r.clase_detectada, r.confianza, r.cajas) == ("no_detectado", None, None, [])


@pytest.mark.parametrize("cambios", [
    {"resultado": "quizas"},
    {"clase_detectada": "semaforo"},
    {"confianza": None},
    {"confianza": 0.1},                                         # detectado bajo el umbral
    {"confianza": float("nan")},
    {"confianza": 1.5},
    {"ancho": None},
    {"cajas": [Caja("perro", 0.9, 0, 0, 10, 10)]},
    {"cajas": [Caja("bache", 0.9, 0, 0, float("inf"), 10)]},
])
def test_normalizar_rechaza_resultados_inconsistentes(cambios):
    with pytest.raises(FalloInferencia):
        normalizar(_resultado(**cambios))


def test_no_detectado_con_cajas_es_inconsistente():
    with pytest.raises(FalloInferencia):
        normalizar(_resultado("no_detectado", cajas=[Caja("bache", 0.9, 0, 0, 10, 10)]))


def test_resultado_inconsistente_se_responde_como_fallo(servidor):
    _, servicio, stub, _ = servidor
    servicio.respuesta = _resultado(clase_detectada="semaforo")
    codigo, urbia, _ = _error(lambda: stub.Detectar(_solicitud(), timeout=5))
    assert (codigo, urbia) == (grpc.StatusCode.INTERNAL, "FALLO_INFERENCIA")


# =========================================== 4. de punta a punta (modelo falso)

class _Lista(list):
    def tolist(self):
        return list(self)


class YoloFalso:
    names = {0: "bache", 1: "basura"}

    def predict(self, source, conf, imgsz, device, verbose):
        cajas = [(10, 20, 110, 220, 1, 0.9), (0, 0, 50, 50, 0, 0.4)]
        cajas = [c for c in cajas if c[5] >= conf]
        boxes = SimpleNamespace(xyxy=_Lista([c[:4] for c in cajas]), cls=_Lista([c[4] for c in cajas]),
                                conf=_Lista([c[5] for c in cajas]))
        return [SimpleNamespace(boxes=boxes)]


@pytest.fixture
def storage(tmp_path):
    raiz = tmp_path / "storage"
    carpeta = raiz / "originales" / "2026" / "11"
    carpeta.mkdir(parents=True)
    foto = io.BytesIO()
    Image.new("RGB", (2560, 1920), (90, 90, 90)).save(foto, format="JPEG")
    (carpeta / f"{NOMBRE}.jpg").write_bytes(foto.getvalue())
    (carpeta / f"{'b' * 26}.jpg").write_bytes(b"esto no es una imagen")
    return raiz


@pytest.fixture
def servidor_real(storage):
    """ServicioInferencia REAL (lector, preprocesamiento, decisión) con YOLO falso."""
    info = InfoModelo(nombre="urbia-yolo26s", version="1.0.0", sha256="a" * 64, umbral_confianza=0.25,
                      imgsz=640, clases={0: "bache", 1: "basura"}, dispositivo="cpu", segundos_carga=0.1)
    config = Configuracion(modelo_config=Path("modelo.json"), storage_dir=storage, device="cpu")
    servicio = ServicioInferencia(config, DetectorUrbia(YoloFalso(), info))
    s = ServidorInferencia(_config(), obtener=lambda: servicio)
    s.iniciar()
    canal = grpc.insecure_channel(f"127.0.0.1:{s.puerto_grpc}")
    yield s, pb_grpc.InferenciaServiceStub(canal)
    canal.close()
    s.apagar()


def test_de_punta_a_punta_respuesta_estructurada(servidor_real):
    _, stub = servidor_real
    r = stub.Detectar(_solicitud(), timeout=5)
    assert r.resultado == pb.RESULTADO_DETECTADO
    assert (r.clase_detectada, round(r.confianza, 4)) == ("basura", 0.9)
    assert [c.clase for c in r.cajas] == ["basura", "bache"]
    assert (r.ancho, r.alto) == (2560, 1920)
    assert (r.modelo.nombre, r.modelo.version) == ("urbia-yolo26s", "1.0.0")
    # Las cajas regresan en píxeles de la foto original (1280 px -> 2560 px = x2)
    assert (r.cajas[0].x1, r.cajas[0].y1, r.cajas[0].x2, r.cajas[0].y2) == (20, 40, 220, 440)


@pytest.mark.parametrize("ruta, codigo, urbia", [
    (f"originales/2026/11/{'c' * 26}.jpg", grpc.StatusCode.NOT_FOUND, "IMAGEN_NO_ENCONTRADA"),
    (f"originales/2026/11/{'b' * 26}.jpg", grpc.StatusCode.INVALID_ARGUMENT, "IMAGEN_INVALIDA"),
])
def test_de_punta_a_punta_errores_de_la_foto(servidor_real, ruta, codigo, urbia):
    _, stub = servidor_real
    assert _error(lambda: stub.Detectar(_solicitud(ruta_archivo=ruta), timeout=5))[:2] == (codigo, urbia)
    assert stub.Detectar(_solicitud(), timeout=5).resultado == pb.RESULTADO_DETECTADO  # sigue vivo


# ============================================ 5. timeout, excepciones, logs

def test_timeout_de_inferencia_del_servidor(servidor):
    s, servicio, stub, _ = servidor
    servicio.espera = 3.0   # > timeout_inferencia (2 s); el cliente da 10 s
    inicio = time.monotonic()
    codigo, urbia, mensaje = _error(lambda: stub.Detectar(_solicitud(), timeout=10))
    assert (codigo, urbia) == (grpc.StatusCode.DEADLINE_EXCEEDED, "TIEMPO_AGOTADO")
    assert "2.0 s" in mensaje
    assert time.monotonic() - inicio < 2.9   # no esperó a que terminara la inferencia


def test_el_deadline_del_cliente_manda_si_es_menor(servidor):
    _, servicio, stub, _ = servidor
    servicio.espera = 1.0
    codigo, _, _ = _error(lambda: stub.Detectar(_solicitud(), timeout=0.3))
    assert codigo == grpc.StatusCode.DEADLINE_EXCEEDED


def test_excepcion_inesperada_es_fallo_y_el_servidor_sigue(servidor):
    _, servicio, stub, _ = servidor
    servicio.excepcion = RuntimeError("bug")
    codigo, urbia, mensaje = _error(lambda: stub.Detectar(_solicitud(), timeout=5))
    assert (codigo, urbia) == (grpc.StatusCode.INTERNAL, "FALLO_INFERENCIA")
    assert "bug" not in mensaje                 # no se filtran detalles internos
    servicio.excepcion = None
    assert stub.Detectar(_solicitud(), timeout=5).resultado == pb.RESULTADO_DETECTADO


def test_una_linea_de_log_por_peticion(servidor, caplog):
    _, servicio, stub, _ = servidor
    with caplog.at_level(logging.INFO, logger="urbia.inferencia"):
        stub.Detectar(_solicitud(id_solicitud="req-ok"), timeout=5)
        _error(lambda: stub.Detectar(_solicitud(id_solicitud="req-mal", ruta_archivo="../x"), timeout=5))
    lineas = [r for r in caplog.records if r.getMessage().startswith("grpc metodo=Detectar")]
    assert len(lineas) == 2
    ok, mal = (r.getMessage() for r in lineas)
    assert re.search(r"id_solicitud=req-ok id_reporte=7 id_archivo=42 codigo=OK urbia_codigo=- ms=\d", ok)
    assert "id_solicitud=req-mal" in mal and "codigo=INVALID_ARGUMENT urbia_codigo=RUTA_INVALIDA" in mal
    assert (lineas[0].levelno, lineas[1].levelno) == (logging.INFO, logging.WARNING)


def test_limpiar_evita_inyeccion_en_logs():
    assert limpiar("a\nb c") == "a_b_c"
    assert limpiar("") == "-"
    assert len(limpiar("x" * 200)) == 64


def test_interceptor_sin_servidor_completo():
    """El interceptor también sirve con crear_servidor() directo (por ejemplo, en scripts)."""
    servicio = ServicioFalso()
    metricas = MetricasGrpc()
    srv, puerto = contrato_grpc.crear_servidor(servicio, "127.0.0.1:0",
                                               interceptores=[InterceptorRegistro(metricas)])
    srv.start()
    with grpc.insecure_channel(f"127.0.0.1:{puerto}") as canal:
        pb_grpc.InferenciaServiceStub(canal).ObtenerInfo(pb.ObtenerInfoRequest(), timeout=5)
    srv.stop(0)
    assert metricas.como_dict()["por_codigo"] == {"OK": 1}


# =================================== 6. no guarda nada ni toca el reporte

def _foto_del_storage(raiz: Path) -> dict:
    return {str(p.relative_to(raiz)): (hashlib.sha256(p.read_bytes()).hexdigest(), p.stat().st_mtime_ns)
            for p in sorted(raiz.rglob("*")) if p.is_file()}


def test_no_modifica_el_almacenamiento(servidor_real, storage):
    _, stub = servidor_real
    antes = _foto_del_storage(storage)
    stub.Detectar(_solicitud(), timeout=5)
    for ruta in (f"originales/2026/11/{'b' * 26}.jpg", f"originales/2026/11/{'c' * 26}.jpg", "../x"):
        _error(lambda: stub.Detectar(_solicitud(ruta_archivo=ruta), timeout=5))
    assert _foto_del_storage(storage) == antes   # mismos archivos, mismo contenido, misma fecha


def test_no_usa_base_de_datos():
    import urbia_vision.servidor  # noqa: F401  (todo el servidor ya importado)
    for modulo in ("psycopg", "psycopg2", "asyncpg", "sqlalchemy", "sqlite3"):
        assert modulo not in sys.modules, f"el servicio de inferencia no debe usar {modulo}"


ESCRITURA = re.compile(
    r"open\([^)]*['\"][wax+]|\.write_(text|bytes)\(|\.unlink\(|\.rename\(|\.replace\(\s*[^)]*Path"
    r"|\.mkdir\(|shutil\.|os\.remove|os\.rename"
    r"|INSERT\s+INTO|UPDATE\s+\w+\s+SET|DELETE\s+FROM"
)


@pytest.mark.parametrize("modulo", ["almacen", "contrato_grpc", "normalizacion", "registro",
                                    "salud", "servicio", "servidor", "preprocesamiento"])
def test_el_codigo_del_servicio_no_escribe(modulo):
    codigo = (RAIZ / "urbia_vision" / f"{modulo}.py").read_text(encoding="utf-8")
    assert not ESCRITURA.search(codigo), f"{modulo}.py parece escribir en disco o en la BD"