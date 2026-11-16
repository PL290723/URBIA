"""
Pruebas del contrato gRPC urbia.inferencia.v1, lado Python (HU 05.4).

Levantan un servidor gRPC real en un puerto libre y lo llaman con el cliente
generado, igual que lo hará Go. El servicio de inferencia es falso.
"""

import threading
import time
from types import SimpleNamespace

import grpc
import pytest

from urbia.inferencia.v1 import inferencia_pb2 as pb
from urbia.inferencia.v1 import inferencia_pb2_grpc as pb_grpc
from urbia_vision import contrato_grpc
from urbia_vision.modelo import Caja
from urbia_vision.servicio import ResultadoInferencia

RUTA = "originales/2026/11/abcdefghijklmnopqrstuvwxyz.jpg"


def _resultado(resultado="detectado", **extra):
    base = dict(
        resultado=resultado, clase_detectada="basura", confianza=0.9123,
        modelo="urbia-yolo26s", version_modelo="1.0.0", umbral_confianza=0.25,
        cajas=[Caja("basura", 0.9123, 10, 20, 110, 220), Caja("bache", 0.41, 0, 0, 50, 50)],
        ancho=2560, alto=1920, milisegundos=180.5,
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
        self.detector = SimpleNamespace(info=SimpleNamespace(
            nombre="urbia-yolo26s", version="1.0.0", umbral_confianza=0.25,
            imgsz=640, dispositivo="cpu", sha256="a" * 64))

    def inferir_archivo(self, ruta):
        self.rutas.append(ruta)
        time.sleep(self.espera)
        return self.respuesta

    def estado(self):
        return {"listo": True, "peticiones": len(self.rutas), "errores": 0}


@pytest.fixture
def conexion():
    servicio = ServicioFalso()
    servidor, puerto = contrato_grpc.crear_servidor(servicio, "127.0.0.1:0")
    servidor.start()
    canal = grpc.insecure_channel(f"127.0.0.1:{puerto}")
    yield servicio, pb_grpc.InferenciaServiceStub(canal)
    canal.close()
    servidor.stop(grace=0)


def _solicitud(**extra):
    datos = dict(id_solicitud="req-123", id_reporte=7, id_archivo=42, ruta_archivo=RUTA)
    datos.update(extra)
    return pb.DetectarRequest(**datos)


def _error(llamada):
    with pytest.raises(grpc.RpcError) as info:
        llamada()
    e = info.value
    return e.code(), dict(e.trailing_metadata() or ()).get(contrato_grpc.TRAILER_CODIGO), e.details()


# -------------------------------------------------------------- respuestas

def test_detectado(conexion):
    servicio, stub = conexion
    r = stub.Detectar(_solicitud(), timeout=5)
    assert servicio.rutas == [RUTA]
    assert r.id_solicitud == "req-123"
    assert r.resultado == pb.RESULTADO_DETECTADO
    assert (r.clase_detectada, round(r.confianza, 4)) == ("basura", 0.9123)
    assert [c.clase for c in r.cajas] == ["basura", "bache"]
    assert (r.cajas[0].x1, r.cajas[0].y2) == (10, 220)
    assert (r.modelo.nombre, r.modelo.version, round(r.modelo.umbral_confianza, 2)) == ("urbia-yolo26s", "1.0.0", 0.25)
    assert (r.ancho, r.alto) == (2560, 1920)


def test_no_detectado_deja_campos_vacios(conexion):
    servicio, stub = conexion
    servicio.respuesta = _resultado("no_detectado")
    r = stub.Detectar(_solicitud(), timeout=5)
    assert r.resultado == pb.RESULTADO_NO_DETECTADO
    assert (r.clase_detectada, r.confianza, len(r.cajas)) == ("", 0.0, 0)
    assert r.modelo.version == "1.0.0"


def test_opciones_de_cajas(conexion):
    _, stub = conexion
    assert len(stub.Detectar(_solicitud(opciones=pb.OpcionesDeteccion(omitir_cajas=True)), timeout=5).cajas) == 0
    r = stub.Detectar(_solicitud(opciones=pb.OpcionesDeteccion(max_cajas=1)), timeout=5)
    assert [c.clase for c in r.cajas] == ["basura"]


def test_obtener_info(conexion):
    _, stub = conexion
    stub.Detectar(_solicitud(), timeout=5)
    i = stub.ObtenerInfo(pb.ObtenerInfoRequest(), timeout=5)
    assert i.version_contrato == contrato_grpc.VERSION_CONTRATO
    assert list(i.clases) == ["bache", "basura"]
    assert (i.modelo.nombre, i.imgsz, i.dispositivo, i.peticiones) == ("urbia-yolo26s", 640, "cpu", 1)


# ------------------------------------------------------------------ errores

@pytest.mark.parametrize("cambios, fragmento", [
    ({"id_reporte": 0}, "id_reporte"),
    ({"id_archivo": -1}, "id_archivo"),
    ({"ruta_archivo": ""}, "ruta_archivo"),
    ({"opciones": pb.OpcionesDeteccion(max_cajas=101)}, "max_cajas"),
])
def test_solicitud_invalida(conexion, cambios, fragmento):
    servicio, stub = conexion
    codigo, urbia, mensaje = _error(lambda: stub.Detectar(_solicitud(**cambios), timeout=5))
    assert (codigo, urbia) == (grpc.StatusCode.INVALID_ARGUMENT, "SOLICITUD_INVALIDA")
    assert fragmento in mensaje
    assert servicio.rutas == []   # ni siquiera se intenta inferir


@pytest.mark.parametrize("codigo_urbia, codigo_grpc", [
    ("RUTA_INVALIDA", grpc.StatusCode.INVALID_ARGUMENT),
    ("IMAGEN_INVALIDA", grpc.StatusCode.INVALID_ARGUMENT),
    ("IMAGEN_NO_ENCONTRADA", grpc.StatusCode.NOT_FOUND),
    ("FALLO_INFERENCIA", grpc.StatusCode.INTERNAL),
])
def test_errores_del_servicio(conexion, codigo_urbia, codigo_grpc):
    servicio, stub = conexion
    servicio.respuesta = _resultado("error", clase_detectada=None, confianza=None, cajas=[],
                                    codigo_error=codigo_urbia, mensaje_error="detalle del error")
    codigo, urbia, mensaje = _error(lambda: stub.Detectar(_solicitud(), timeout=5))
    assert (codigo, urbia, mensaje) == (codigo_grpc, codigo_urbia, "detalle del error")


def test_el_servidor_sigue_respondiendo_despues_de_un_error(conexion):
    servicio, stub = conexion
    servicio.respuesta = _resultado("error", codigo_error="IMAGEN_INVALIDA", mensaje_error="x")
    _error(lambda: stub.Detectar(_solicitud(), timeout=5))
    servicio.respuesta = _resultado()
    assert stub.Detectar(_solicitud(), timeout=5).resultado == pb.RESULTADO_DETECTADO


def test_timeout_del_cliente(conexion):
    servicio, stub = conexion
    servicio.espera = 0.5
    codigo, _, _ = _error(lambda: stub.Detectar(_solicitud(), timeout=0.1))
    assert codigo == grpc.StatusCode.DEADLINE_EXCEEDED


def test_sin_tiempo_suficiente_no_infiere(conexion, monkeypatch):
    servicio, stub = conexion
    monkeypatch.setattr(contrato_grpc, "MARGEN_MINIMO_SEGUNDOS", 10.0)
    codigo, urbia, _ = _error(lambda: stub.Detectar(_solicitud(), timeout=5))
    assert (codigo, urbia) == (grpc.StatusCode.DEADLINE_EXCEEDED, "TIEMPO_AGOTADO")
    assert servicio.rutas == []


def test_peticiones_concurrentes(conexion):
    servicio, stub = conexion
    servicio.espera = 0.05
    resultados = []
    hilos = [threading.Thread(target=lambda: resultados.append(stub.Detectar(_solicitud(), timeout=5).resultado))
             for _ in range(8)]
    for h in hilos:
        h.start()
    for h in hilos:
        h.join()
    assert resultados == [pb.RESULTADO_DETECTADO] * 8


def test_servicio_no_disponible():
    # Nadie escucha en el puerto 1. En Windows la conexión rechazada tarda ~2 s
    # (reintentos de TCP), así que el timeout debe ser mayor para recibir UNAVAILABLE.
    canal = grpc.insecure_channel("127.0.0.1:1")
    codigo, _, _ = _error(lambda: pb_grpc.InferenciaServiceStub(canal).Detectar(_solicitud(), timeout=10))
    assert codigo == grpc.StatusCode.UNAVAILABLE
    canal.close()


def test_tabla_de_codigos_completa():
    from urbia_vision import errores
    codigos = {c.codigo for c in vars(errores).values()
               if isinstance(c, type) and issubclass(c, errores.ErrorInferencia) and c is not errores.ErrorInferencia}
    assert codigos <= set(contrato_grpc.CODIGOS_GRPC)
    assert contrato_grpc.codigo_grpc("ALGO_NUEVO") == grpc.StatusCode.UNKNOWN