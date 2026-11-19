"""
Health check HTTP del servicio de inferencia (HU 05.5).

El backend en Go (HU de salud, docs/backend/salud.md) consulta
`INFERENCE_URL/health` cada 15 s y espera **200 solo cuando el modelo está
cargado**. Este servidor HTTP es mínimo (biblioteca estándar) y corre en un
hilo aparte del gRPC:

    GET /health   200 {"estado": "listo", ...}        modelo cargado, gRPC atendiendo
                  503 {"estado": "iniciando", ...}    cargando el modelo
                  503 {"estado": "apagando", ...}     recibió SIGTERM/Ctrl+C
    otra ruta     404       otro método   405

No expone rutas de fotos, variables de entorno ni trazas de error.
"""

from __future__ import annotations

import json
import logging
import threading
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Callable

log = logging.getLogger("urbia.inferencia")

INICIANDO = "iniciando"
LISTO = "listo"
APAGANDO = "apagando"

ProveedorEstado = Callable[[], dict]


def codigo_http(estado: dict) -> int:
    return HTTPStatus.OK if estado.get("estado") == LISTO else HTTPStatus.SERVICE_UNAVAILABLE


def _crear_manejador(proveedor: ProveedorEstado):
    class ManejadorSalud(BaseHTTPRequestHandler):
        server_version = "urbia-inferencia"
        sys_version = ""

        def do_GET(self):
            if self.path.split("?", 1)[0] != "/health":
                return self._responder(HTTPStatus.NOT_FOUND, {"error": "no encontrado"})
            try:
                estado = proveedor()
            except Exception:
                log.exception("no se pudo calcular el estado de salud")
                estado = {"estado": "error"}
            self._responder(codigo_http(estado), estado)

        def do_HEAD(self):
            self.do_GET()

        def _no_permitido(self):
            self._responder(HTTPStatus.METHOD_NOT_ALLOWED, {"error": "método no permitido"},
                            {"Allow": "GET, HEAD"})

        do_POST = do_PUT = do_PATCH = do_DELETE = _no_permitido

        def _responder(self, codigo: int, cuerpo: dict, extra: dict | None = None):
            datos = json.dumps(cuerpo, ensure_ascii=False).encode("utf-8")
            self.send_response(codigo)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(datos)))
            self.send_header("Cache-Control", "no-store")
            for k, v in (extra or {}).items():
                self.send_header(k, v)
            self.end_headers()
            if self.command != "HEAD":
                self.wfile.write(datos)

        def log_message(self, formato, *args):
            # Go consulta cada 15 s: solo en DEBUG para no llenar el log
            log.debug("salud %s %s", self.address_string(), formato % args)

    return ManejadorSalud


class ServidorSalud:
    def __init__(self, direccion: str, proveedor: ProveedorEstado):
        host, _, puerto = direccion.rpartition(":")
        self._http = ThreadingHTTPServer((host, int(puerto)), _crear_manejador(proveedor))
        self._http.daemon_threads = True
        self._hilo: threading.Thread | None = None

    @property
    def puerto(self) -> int:
        return self._http.server_address[1]

    def iniciar(self) -> None:
        self._hilo = threading.Thread(target=self._http.serve_forever, kwargs={"poll_interval": 0.1},
                                      name="salud-http", daemon=True)
        self._hilo.start()

    def detener(self) -> None:
        self._http.shutdown()
        self._http.server_close()
        if self._hilo:
            self._hilo.join(timeout=5)