"""
Lado Python del contrato gRPC `urbia.inferencia.v1` (HU 05.4, completado en 05.5).

Traduce entre los mensajes del .proto y el ServicioInferencia de la HU 05.3:

    DetectarRequest  --validar_solicitud-->  servicio.inferir_archivo(ruta)   (con límite de tiempo)
    ResultadoInferencia  --normalizar-->  --a_respuesta-->  DetectarResponse
    resultado "error" + codigo_error  -->    código gRPC + trailer "urbia-codigo"

HU 05.5 agrega:
- Validación del identificador y de la ruta ANTES de tocar el disco o el modelo.
- Límite de tiempo por inferencia (`timeout_inferencia`), además del deadline de Go.
- Normalización del resultado (normalizacion.py) antes de responder.
- Interceptores (log por petición) en `crear_servidor()`.

El proceso completo (health check, señales, apagado ordenado) está en servidor.py.
Este módulo NUNCA escribe en disco ni en la base de datos: solo lee y responde.
"""

from __future__ import annotations

import logging
from concurrent import futures

import grpc

from urbia.inferencia.v1 import inferencia_pb2 as pb
from urbia.inferencia.v1 import inferencia_pb2_grpc as pb_grpc
from urbia_vision import clases
from urbia_vision.almacen import PATRON_RUTA
from urbia_vision.decision import DETECTADO, NO_DETECTADO
from urbia_vision.errores import ErrorInferencia
from urbia_vision.normalizacion import normalizar

log = logging.getLogger("urbia.inferencia")

VERSION_CONTRATO = "1.0.0"
TRAILER_CODIGO = "urbia-codigo"
MAX_CAJAS_PERMITIDO = 100
MAX_LARGO_ID_SOLICITUD = 128
MAX_LARGO_RUTA = 255   # archivo.ruta es VARCHAR(255)

# Margen mínimo de tiempo para empezar una inferencia. Si a la petición le
# queda menos, se responde DEADLINE_EXCEEDED sin gastar la GPU/CPU.
MARGEN_MINIMO_SEGUNDOS = 0.05

CODIGOS_GRPC: dict[str, grpc.StatusCode] = {
    "SOLICITUD_INVALIDA": grpc.StatusCode.INVALID_ARGUMENT,
    "RUTA_INVALIDA": grpc.StatusCode.INVALID_ARGUMENT,
    "IMAGEN_INVALIDA": grpc.StatusCode.INVALID_ARGUMENT,
    "IMAGEN_NO_ENCONTRADA": grpc.StatusCode.NOT_FOUND,
    "FALLO_INFERENCIA": grpc.StatusCode.INTERNAL,
    "MODELO_NO_DISPONIBLE": grpc.StatusCode.UNAVAILABLE,
    "MODELO_INVALIDO": grpc.StatusCode.UNAVAILABLE,
    "TIEMPO_AGOTADO": grpc.StatusCode.DEADLINE_EXCEEDED,
}

_RESULTADOS = {DETECTADO: pb.RESULTADO_DETECTADO, NO_DETECTADO: pb.RESULTADO_NO_DETECTADO}


def codigo_grpc(codigo_urbia: str) -> grpc.StatusCode:
    return CODIGOS_GRPC.get(codigo_urbia, grpc.StatusCode.UNKNOWN)


def validar_solicitud(req: pb.DetectarRequest) -> tuple[str, str] | None:
    """(codigo_urbia, mensaje) si la solicitud no se debe procesar, o None.

    Se valida todo lo que se puede sin tocar el disco: el lector de 05.3 vuelve
    a revisar la ruta y además que no salga de STORAGE_DIR (symlinks, junctions).
    """
    if len(req.id_solicitud) > MAX_LARGO_ID_SOLICITUD:
        return "SOLICITUD_INVALIDA", f"id_solicitud no puede tener más de {MAX_LARGO_ID_SOLICITUD} caracteres"
    if any(not c.isprintable() for c in req.id_solicitud):
        return "SOLICITUD_INVALIDA", "id_solicitud contiene caracteres de control"
    if req.id_reporte <= 0:
        return "SOLICITUD_INVALIDA", "id_reporte es obligatorio y debe ser mayor que 0"
    if req.id_archivo <= 0:
        return "SOLICITUD_INVALIDA", "id_archivo es obligatorio y debe ser mayor que 0"
    if not req.ruta_archivo:
        return "SOLICITUD_INVALIDA", "ruta_archivo es obligatoria"
    if req.opciones.max_cajas > MAX_CAJAS_PERMITIDO:
        return "SOLICITUD_INVALIDA", f"opciones.max_cajas debe estar entre 0 y {MAX_CAJAS_PERMITIDO}"
    if len(req.ruta_archivo) > MAX_LARGO_RUTA or not PATRON_RUTA.fullmatch(req.ruta_archivo):
        return "RUTA_INVALIDA", "ruta_archivo no tiene el formato de una foto guardada por URBIA"
    return None


def modelo_usado(info) -> pb.ModeloUsado:
    return pb.ModeloUsado(nombre=info.nombre, version=info.version,
                          umbral_confianza=info.umbral_confianza)


def a_respuesta(resultado, req: pb.DetectarRequest) -> pb.DetectarResponse:
    """ResultadoInferencia (05.3, sin error y ya normalizado) -> DetectarResponse."""
    cajas = [] if req.opciones.omitir_cajas else list(resultado.cajas)
    if req.opciones.max_cajas:
        cajas = cajas[: req.opciones.max_cajas]   # ya vienen de mayor a menor confianza
    return pb.DetectarResponse(
        id_solicitud=req.id_solicitud,
        resultado=_RESULTADOS[resultado.resultado],
        clase_detectada=resultado.clase_detectada or "",
        confianza=resultado.confianza or 0.0,
        cajas=[pb.Caja(clase=c.clase, confianza=c.confianza, x1=c.x1, y1=c.y1, x2=c.x2, y2=c.y2)
               for c in cajas],
        modelo=pb.ModeloUsado(nombre=resultado.modelo, version=resultado.version_modelo,
                              umbral_confianza=resultado.umbral_confianza),
        ancho=resultado.ancho or 0,
        alto=resultado.alto or 0,
        milisegundos=resultado.milisegundos,
    )


def _abortar(context: grpc.ServicerContext, codigo: str, mensaje: str):
    context.set_trailing_metadata(((TRAILER_CODIGO, codigo),))
    context.abort(codigo_grpc(codigo), mensaje)   # lanza una excepción: no regresa


class InferenciaServicer(pb_grpc.InferenciaServiceServicer):
    """Implementa el contrato sobre un ServicioInferencia ya cargado (HU 05.3).

    timeout_inferencia: segundos máximos por inferencia (None = solo el deadline
    del cliente). La inferencia corre en `ejecutor` para poder dejar de esperarla;
    si no se pasa uno, el servicer crea el suyo.
    """

    def __init__(self, servicio, timeout_inferencia: float | None = None,
                 ejecutor: futures.Executor | None = None):
        self.servicio = servicio
        self.timeout_inferencia = timeout_inferencia
        self._ejecutor = ejecutor
        if timeout_inferencia is not None and ejecutor is None:
            self._ejecutor = futures.ThreadPoolExecutor(max_workers=4, thread_name_prefix="inferencia")

    def Detectar(self, request: pb.DetectarRequest, context: grpc.ServicerContext) -> pb.DetectarResponse:
        error = validar_solicitud(request)
        if error:
            _abortar(context, *error)

        limite = self._limite(context)
        if limite is not None and limite < MARGEN_MINIMO_SEGUNDOS:
            _abortar(context, "TIEMPO_AGOTADO", "La solicitud llegó sin tiempo suficiente para inferir")

        r = self._inferir(request.ruta_archivo, limite, context)
        if r.resultado not in _RESULTADOS:
            _abortar(context, r.codigo_error or "FALLO_INFERENCIA", r.mensaje_error or "Error al inferir")

        try:
            r = normalizar(r)
        except ErrorInferencia as e:
            log.error("resultado descartado id_solicitud=%s: %s", request.id_solicitud or "-", e.mensaje)
            _abortar(context, e.codigo, e.mensaje)
        return a_respuesta(r, request)

    def ObtenerInfo(self, request: pb.ObtenerInfoRequest, context: grpc.ServicerContext) -> pb.ObtenerInfoResponse:
        info = self.servicio.detector.info
        estado = self.servicio.estado()
        return pb.ObtenerInfoResponse(
            version_contrato=VERSION_CONTRATO,
            modelo=modelo_usado(info),
            clases=[clases.NOMBRES[i] for i in sorted(clases.NOMBRES)],
            imgsz=info.imgsz,
            dispositivo=info.dispositivo,
            sha256_pesos=info.sha256,
            peticiones=estado["peticiones"],
            errores=estado["errores"],
        )

    # ---------------------------------------------------------------- interno

    def _limite(self, context) -> float | None:
        """Lo que sea menor: el tiempo que le queda al cliente o timeout_inferencia."""
        restante = context.time_remaining()
        candidatos = [t for t in (restante, self.timeout_inferencia) if t is not None]
        return min(candidatos) if candidatos else None

    def _inferir(self, ruta: str, limite: float | None, context):
        if self._ejecutor is None or limite is None:
            return self.servicio.inferir_archivo(ruta)
        futuro = self._ejecutor.submit(self.servicio.inferir_archivo, ruta)
        try:
            return futuro.result(timeout=limite)
        except futures.TimeoutError:
            # Si aún no empezaba, se cancela. Si ya empezó no se puede interrumpir
            # (PyTorch no lo permite): termina en segundo plano y se descarta.
            futuro.cancel()
            _abortar(context, "TIEMPO_AGOTADO", f"La inferencia superó {limite:.1f} s")

    def cerrar(self) -> None:
        if self._ejecutor is not None:
            self._ejecutor.shutdown(wait=False, cancel_futures=True)


def crear_servidor(servicio, direccion: str, max_hilos: int = 4, *,
                   timeout_inferencia: float | None = None,
                   ejecutor: futures.Executor | None = None,
                   interceptores=()) -> tuple[grpc.Server, int]:
    """Servidor gRPC sin TLS (red interna de Docker). Devuelve (servidor, puerto)."""
    servidor = grpc.server(
        futures.ThreadPoolExecutor(max_workers=max_hilos, thread_name_prefix="grpc"),
        interceptors=list(interceptores),
    )
    servicer = InferenciaServicer(servicio, timeout_inferencia, ejecutor)
    pb_grpc.add_InferenciaServiceServicer_to_server(servicer, servidor)
    puerto = servidor.add_insecure_port(direccion)
    if puerto == 0:
        raise OSError(f"No se pudo abrir {direccion} (¿puerto ocupado?)")
    return servidor, puerto