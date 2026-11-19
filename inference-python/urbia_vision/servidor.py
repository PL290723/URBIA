"""
Servidor del servicio de inferencia (HU 05.5).

Une todo lo anterior en un proceso que el backend en Go puede usar:

    1. Health HTTP en URBIA_SALUD_DIRECCION       -> 503 "iniciando"
    2. Carga ÚNICA del modelo (HU 05.3)           -> si falla, el proceso termina (código 2)
    3. Servidor gRPC en URBIA_GRPC_DIRECCION      -> contrato urbia.inferencia.v1 (HU 05.4)
       con log por petición y timeout de inferencia
    4. Health                                    -> 200 "listo"
    5. SIGTERM / Ctrl+C -> health 503 "apagando", deja de aceptar peticiones,
       espera hasta URBIA_APAGADO_GRACIA a las que están en curso y termina.

Responsabilidad: SOLO responder detecciones. Nunca escribe en la base de datos,
nunca cambia el estado del reporte y nunca escribe, mueve ni borra fotos.
Go decide qué guardar en `deteccion` y si el reporte cambia de estado.
"""

from __future__ import annotations

import logging
import signal
import threading
import time
from concurrent import futures

from urbia_vision import salud
from urbia_vision.configuracion_servidor import ConfigServidor
from urbia_vision.contrato_grpc import VERSION_CONTRATO, crear_servidor
from urbia_vision.registro import InterceptorRegistro, MetricasGrpc
from urbia_vision.servicio import obtener_servicio

log = logging.getLogger("urbia.inferencia")


class ServidorInferencia:
    def __init__(self, config: ConfigServidor, obtener=None):
        self.config = config
        self._obtener = obtener or obtener_servicio
        self._estado = salud.INICIANDO
        self._inicio = time.monotonic()
        self._detener = threading.Event()
        self._candado_apagado = threading.Lock()
        self._apagado = False

        self.metricas = MetricasGrpc()
        self.servicio = None
        self.grpc = None
        self.puerto_grpc: int | None = None
        self.salud: salud.ServidorSalud | None = None
        self._ejecutor: futures.ThreadPoolExecutor | None = None

    # ------------------------------------------------------------- ciclo de vida

    def iniciar(self) -> None:
        """Arranca health -> modelo -> gRPC. Si el modelo falla, lanza ErrorInferencia."""
        self.salud = salud.ServidorSalud(self.config.direccion_salud, self.estado_salud)
        self.salud.iniciar()
        log.info("health escuchando direccion=%s estado=%s", self.config.direccion_salud, self._estado)

        try:
            self.servicio = self._obtener()
            self._ejecutor = futures.ThreadPoolExecutor(
                max_workers=self.config.hilos, thread_name_prefix="inferencia")
            self.grpc, self.puerto_grpc = crear_servidor(
                self.servicio, self.config.direccion_grpc, self.config.hilos,
                timeout_inferencia=self.config.timeout_inferencia,
                ejecutor=self._ejecutor,
                interceptores=[InterceptorRegistro(self.metricas)],
            )
            self.grpc.start()
        except BaseException:
            self.apagar()
            raise

        self._estado = salud.LISTO
        info = self.servicio.detector.info
        log.info("servidor listo grpc=%s salud=%s contrato=v%s modelo=%s version=%s dispositivo=%s "
                 "hilos=%d timeout_inferencia=%.1fs",
                 self.config.direccion_grpc, self.config.direccion_salud, VERSION_CONTRATO,
                 info.nombre, info.version, info.dispositivo, self.config.hilos,
                 self.config.timeout_inferencia)

    def solicitar_apagado(self, motivo: str = "solicitud") -> None:
        if not self._detener.is_set():
            log.info("apagado solicitado motivo=%s", motivo)
            self._estado = salud.APAGANDO   # el health responde 503 de inmediato
            self._detener.set()

    def esperar(self) -> None:
        """Bloquea hasta solicitar_apagado(). Con espera corta para que Ctrl+C funcione en Windows."""
        while not self._detener.wait(0.5):
            pass
        self.apagar()

    def apagar(self) -> None:
        with self._candado_apagado:
            if self._apagado:
                return
            self._apagado = True
        self._estado = salud.APAGANDO
        inicio = time.monotonic()
        if self.grpc is not None:
            # Rechaza peticiones nuevas y espera a las que están en curso hasta `gracia`
            self.grpc.stop(grace=self.config.gracia_apagado).wait(self.config.gracia_apagado + 1)
        if self._ejecutor is not None:
            self._ejecutor.shutdown(wait=False, cancel_futures=True)
        if self.salud is not None:
            self.salud.detener()
        log.info("servidor detenido segundos=%.2f peticiones_grpc=%d",
                 time.monotonic() - inicio, self.metricas.como_dict()["peticiones"])

    # ------------------------------------------------------------------ salud

    def estado_salud(self) -> dict:
        estado = {
            "estado": self._estado,
            "servicio": "urbia-inferencia",
            "version_contrato": VERSION_CONTRATO,
            "segundos_activo": round(time.monotonic() - self._inicio, 1),
        }
        if self.servicio is not None:
            info = self.servicio.detector.info
            datos = self.servicio.estado()
            estado["modelo"] = {"nombre": info.nombre, "version": info.version,
                                "umbral_confianza": info.umbral_confianza, "dispositivo": info.dispositivo}
            estado["inferencias"] = {"peticiones": datos["peticiones"], "errores": datos["errores"]}
            estado["grpc"] = self.metricas.como_dict()
        return estado


def instalar_senales(servidor: ServidorInferencia) -> None:
    """SIGTERM (docker stop), SIGINT (Ctrl+C) y SIGBREAK (Ctrl+Break en Windows)."""
    def manejar(numero, _frame):
        servidor.solicitar_apagado(signal.Signals(numero).name)

    for nombre in ("SIGTERM", "SIGINT", "SIGBREAK"):
        if hasattr(signal, nombre):
            signal.signal(getattr(signal, nombre), manejar)