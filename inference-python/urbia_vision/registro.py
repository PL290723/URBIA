"""
Logging del servidor de inferencia (HU 05.5).

- `configurar_logging()` deja un formato único `clave=valor` para todo el proceso.
- `InterceptorRegistro` envuelve cada RPC y escribe UNA línea por petición con
  id_solicitud, id_reporte, id_archivo, código gRPC, código de URBIA y tiempo.
  Así una petición se sigue con el mismo X-Request-ID en los logs de Go y Python.
- Si un método lanza una excepción inesperada (no un `context.abort`), el
  interceptor la registra con su traza y responde INTERNAL + FALLO_INFERENCIA,
  en vez del UNKNOWN genérico de grpcio. El servidor sigue atendiendo.
- `MetricasGrpc` cuenta peticiones por código para el GET /health.
"""

from __future__ import annotations

import logging
import threading
import time
from collections import Counter

import grpc

from urbia_vision.contrato_grpc import TRAILER_CODIGO

log = logging.getLogger("urbia.inferencia")

FORMATO_LOG = "%(asctime)s %(levelname)s %(name)s %(message)s"
MAX_LARGO_LOG = 64

# Errores causados por la petición (WARNING) vs. por el servicio (ERROR)
_CODIGOS_CLIENTE = {grpc.StatusCode.INVALID_ARGUMENT, grpc.StatusCode.NOT_FOUND,
                    grpc.StatusCode.CANCELLED}


def configurar_logging(nivel: int = logging.INFO) -> None:
    logging.basicConfig(level=nivel, format=FORMATO_LOG, force=True)
    # grpcio y urllib3 son muy verbosos en DEBUG
    for ruidoso in ("grpc", "urllib3", "PIL"):
        logging.getLogger(ruidoso).setLevel(max(nivel, logging.INFO))


def limpiar(valor, largo: int = MAX_LARGO_LOG) -> str:
    """Texto seguro para una línea de log: sin espacios, saltos ni caracteres de control."""
    texto = str(valor) if valor not in (None, "") else "-"
    texto = "".join(c if c.isprintable() and not c.isspace() else "_" for c in texto)
    return texto if len(texto) <= largo else texto[: largo - 1] + "…"


class MetricasGrpc:
    """Contadores de peticiones por código gRPC, seguros entre hilos."""

    def __init__(self):
        self._candado = threading.Lock()
        self._por_codigo: Counter[str] = Counter()

    def registrar(self, codigo: grpc.StatusCode) -> None:
        with self._candado:
            self._por_codigo[codigo.name] += 1

    def como_dict(self) -> dict:
        with self._candado:
            por_codigo = dict(self._por_codigo)
        total = sum(por_codigo.values())
        return {"peticiones": total, "errores": total - por_codigo.get("OK", 0), "por_codigo": por_codigo}


def _trailer(context) -> str:
    for clave, valor in context.trailing_metadata() or ():
        if clave == TRAILER_CODIGO:
            return valor
    return "-"


class InterceptorRegistro(grpc.ServerInterceptor):
    def __init__(self, metricas: MetricasGrpc | None = None):
        self.metricas = metricas or MetricasGrpc()

    def intercept_service(self, continuation, handler_call_details):
        handler = continuation(handler_call_details)
        if handler is None or handler.unary_unary is None:
            return handler   # método inexistente (UNIMPLEMENTED) o de streaming
        metodo = handler_call_details.method.rsplit("/", 1)[-1]
        interno = handler.unary_unary

        def envoltura(request, context):
            inicio = time.perf_counter()
            try:
                respuesta = interno(request, context)
            except Exception:
                if context.code() is None:
                    # No fue un context.abort(): error de programación o algo imprevisto
                    log.exception("excepción no controlada en %s", metodo)
                    self._terminar(request, context, metodo, inicio, grpc.StatusCode.INTERNAL, "FALLO_INFERENCIA")
                    context.set_trailing_metadata(((TRAILER_CODIGO, "FALLO_INFERENCIA"),))
                    context.abort(grpc.StatusCode.INTERNAL, "Error interno del servicio de inferencia")
                self._terminar(request, context, metodo, inicio, context.code(), _trailer(context))
                raise
            self._terminar(request, context, metodo, inicio, context.code() or grpc.StatusCode.OK, "-")
            return respuesta

        return grpc.unary_unary_rpc_method_handler(
            envoltura,
            request_deserializer=handler.request_deserializer,
            response_serializer=handler.response_serializer,
        )

    def _terminar(self, request, context, metodo, inicio, codigo, codigo_urbia) -> None:
        self.metricas.registrar(codigo)
        ms = (time.perf_counter() - inicio) * 1000
        if codigo == grpc.StatusCode.OK:
            nivel = logging.INFO
        elif codigo in _CODIGOS_CLIENTE:
            nivel = logging.WARNING
        else:
            nivel = logging.ERROR
        log.log(
            nivel,
            "grpc metodo=%s id_solicitud=%s id_reporte=%s id_archivo=%s codigo=%s urbia_codigo=%s ms=%.1f peer=%s",
            metodo,
            limpiar(getattr(request, "id_solicitud", "")),
            getattr(request, "id_reporte", "-"),
            getattr(request, "id_archivo", "-"),
            codigo.name, limpiar(codigo_urbia), ms, limpiar(context.peer()),
        )