"""
Lado Python del contrato gRPC `urbia.inferencia.v1` (HU 05.4).

Traduce entre los mensajes del .proto y el ServicioInferencia de la HU 05.3:

    DetectarRequest  --validar_solicitud-->  servicio.inferir_archivo(ruta)
    ResultadoInferencia  --a_respuesta-->    DetectarResponse
    resultado "error" + codigo_error  -->    código gRPC + trailer "urbia-codigo"

El servidor completo (puerto, health check, logging por petición, apagado
ordenado) es la HU 05.5. Aquí `crear_servidor()` arma uno mínimo para
validar que Go y Python se entienden con el contrato.
"""

from __future__ import annotations

import logging
from concurrent import futures

import grpc

from urbia.inferencia.v1 import inferencia_pb2 as pb
from urbia.inferencia.v1 import inferencia_pb2_grpc as pb_grpc
from urbia_vision import clases
from urbia_vision.decision import DETECTADO, NO_DETECTADO

log = logging.getLogger("urbia.inferencia")

VERSION_CONTRATO = "1.0.0"
TRAILER_CODIGO = "urbia-codigo"
MAX_CAJAS_PERMITIDO = 100

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


def validar_solicitud(req: pb.DetectarRequest) -> str | None:
    """Mensaje de error si la solicitud está mal formada, o None. La ruta la valida 05.3."""
    if req.id_reporte <= 0:
        return "id_reporte es obligatorio y debe ser mayor que 0"
    if req.id_archivo <= 0:
        return "id_archivo es obligatorio y debe ser mayor que 0"
    if not req.ruta_archivo:
        return "ruta_archivo es obligatoria"
    if req.opciones.max_cajas > MAX_CAJAS_PERMITIDO:
        return f"opciones.max_cajas debe estar entre 0 y {MAX_CAJAS_PERMITIDO}"
    return None


def modelo_usado(info) -> pb.ModeloUsado:
    return pb.ModeloUsado(nombre=info.nombre, version=info.version,
                          umbral_confianza=info.umbral_confianza)


def a_respuesta(resultado, req: pb.DetectarRequest) -> pb.DetectarResponse:
    """ResultadoInferencia (05.3, sin error) -> DetectarResponse."""
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
    """Implementa el contrato sobre un ServicioInferencia ya cargado (HU 05.3)."""

    def __init__(self, servicio):
        self.servicio = servicio

    def Detectar(self, request: pb.DetectarRequest, context: grpc.ServicerContext) -> pb.DetectarResponse:
        error = validar_solicitud(request)
        if error:
            _abortar(context, "SOLICITUD_INVALIDA", error)

        restante = context.time_remaining()
        if restante is not None and restante < MARGEN_MINIMO_SEGUNDOS:
            _abortar(context, "TIEMPO_AGOTADO", "La solicitud llegó sin tiempo suficiente para inferir")

        r = self.servicio.inferir_archivo(request.ruta_archivo)
        log.info("grpc Detectar id_solicitud=%s id_reporte=%d id_archivo=%d resultado=%s",
                 request.id_solicitud or "-", request.id_reporte, request.id_archivo, r.resultado)

        if r.resultado not in _RESULTADOS:
            _abortar(context, r.codigo_error or "FALLO_INFERENCIA", r.mensaje_error or "Error al inferir")
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


def crear_servidor(servicio, direccion: str, max_hilos: int = 4) -> tuple[grpc.Server, int]:
    """Servidor gRPC sin TLS (red interna de Docker). Devuelve (servidor, puerto)."""
    servidor = grpc.server(futures.ThreadPoolExecutor(max_workers=max_hilos))
    pb_grpc.add_InferenciaServiceServicer_to_server(InferenciaServicer(servicio), servidor)
    puerto = servidor.add_insecure_port(direccion)
    if puerto == 0:
        raise OSError(f"No se pudo abrir {direccion} (¿puerto ocupado?)")
    return servidor, puerto