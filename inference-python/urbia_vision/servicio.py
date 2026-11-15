"""
Servicio de inferencia de URBIA (HU 05.3).

Une las piezas: configuración → lector de STORAGE_DIR → preprocesamiento →
modelo (cargado UNA vez) → regla de decisión.

    servicio = obtener_servicio()          # 1.ª llamada: carga el modelo
    servicio = obtener_servicio()          # siguientes: el mismo objeto, sin recargar
    r = servicio.inferir_archivo("originales/2026/11/<26 caracteres>.jpg")

Reglas:
- Un error de la imagen o de la inferencia NO tumba el servicio: se devuelve un
  resultado con `resultado="error"` y su `codigo_error`, y la siguiente petición
  se atiende normal.
- Un error del MODELO (faltan pesos, SHA-256 distinto, clases equivocadas) sí
  impide arrancar: `obtener_servicio()` lanza la excepción.
- El servicio NO guarda nada ni toca el estado del reporte: solo responde.
  El backend en Go decide qué guardar en la tabla `deteccion`.

La HU 05.5 envolverá `inferir_archivo()` en el servidor gRPC.
"""

from __future__ import annotations

import logging
import threading
import time
from dataclasses import asdict, dataclass, field

from urbia_vision.almacen import LectorImagenes
from urbia_vision.configuracion import Configuracion
from urbia_vision.decision import ERROR
from urbia_vision.errores import ErrorInferencia, FalloInferencia
from urbia_vision.modelo import Caja, DetectorUrbia
from urbia_vision.preprocesamiento import LimitesImagen, preparar

log = logging.getLogger("urbia.inferencia")


@dataclass
class ResultadoInferencia:
    """Respuesta por imagen. Los nombres coinciden con la tabla `deteccion`."""
    resultado: str                       # detectado | no_detectado | error
    clase_detectada: str | None
    confianza: float | None
    modelo: str                          # deteccion.modelo
    version_modelo: str                  # deteccion.version_modelo
    umbral_confianza: float
    cajas: list[Caja] = field(default_factory=list)
    ancho: int | None = None
    alto: int | None = None
    milisegundos: float = 0.0
    codigo_error: str | None = None
    mensaje_error: str | None = None

    def como_dict(self) -> dict:
        return asdict(self)


class ServicioInferencia:
    def __init__(self, config: Configuracion, detector: DetectorUrbia):
        self.config = config
        self.detector = detector
        self.lector = LectorImagenes(config.storage_dir, config.max_bytes)
        self.limites = LimitesImagen(config.lado_min, config.lado_max,
                                     config.max_pixeles, config.lado_max_inferencia)
        self.peticiones = 0
        self.errores = 0
        self._candado_contadores = threading.Lock()

    @classmethod
    def iniciar(cls, config: Configuracion | None = None) -> "ServicioInferencia":
        config = config or Configuracion.desde_entorno()
        log.info("iniciando servicio storage_dir=%s modelo_config=%s",
                 config.storage_dir, config.modelo_config)
        if not config.storage_dir.is_dir():
            log.warning("STORAGE_DIR no existe todavía: %s", config.storage_dir)
        detector = DetectorUrbia.cargar(config.modelo_config, config.device)
        return cls(config, detector)

    # --------------------------------------------------------------- entradas

    def inferir_archivo(self, ruta_relativa: str) -> ResultadoInferencia:
        """Imagen guardada por Go en STORAGE_DIR (uso normal)."""
        return self._procesar(lambda: self.lector.leer(ruta_relativa), ruta_relativa)

    def inferir_bytes(self, datos: bytes, etiqueta: str = "<bytes>") -> ResultadoInferencia:
        """Imagen en memoria (pruebas y scripts)."""
        return self._procesar(lambda: datos, etiqueta)

    # ---------------------------------------------------------------- interno

    def _procesar(self, obtener_datos, etiqueta: str) -> ResultadoInferencia:
        inicio = time.perf_counter()
        info = self.detector.info
        ancho = alto = None
        try:
            imagen = preparar(obtener_datos(), self.limites)
            ancho, alto = imagen.ancho_original, imagen.alto_original
            salida = self.detector.detectar(imagen)
            r = salida.resultado
            resultado = ResultadoInferencia(
                resultado=r.resultado, clase_detectada=r.clase_detectada, confianza=r.confianza,
                modelo=info.nombre, version_modelo=info.version,
                umbral_confianza=info.umbral_confianza, cajas=salida.cajas,
                ancho=ancho, alto=alto,
            )
        except ErrorInferencia as e:
            resultado = self._error(e.codigo, e.mensaje, ancho, alto)
        except Exception as e:  # cualquier cosa inesperada tampoco tumba el servicio
            log.exception("error inesperado procesando %s", etiqueta)
            resultado = self._error(FalloInferencia.codigo, f"Error interno: {type(e).__name__}", ancho, alto)

        resultado.milisegundos = round((time.perf_counter() - inicio) * 1000, 1)
        with self._candado_contadores:
            self.peticiones += 1
            if resultado.resultado == ERROR:
                self.errores += 1

        nivel = logging.WARNING if resultado.resultado == ERROR else logging.INFO
        log.log(nivel, "inferencia entrada=%s resultado=%s clase=%s confianza=%s cajas=%d ms=%.1f%s",
                etiqueta, resultado.resultado, resultado.clase_detectada, resultado.confianza,
                len(resultado.cajas), resultado.milisegundos,
                f" codigo={resultado.codigo_error}" if resultado.codigo_error else "")
        return resultado

    def _error(self, codigo: str, mensaje: str, ancho, alto) -> ResultadoInferencia:
        info = self.detector.info
        return ResultadoInferencia(
            resultado=ERROR, clase_detectada=None, confianza=None,
            modelo=info.nombre, version_modelo=info.version,
            umbral_confianza=info.umbral_confianza, ancho=ancho, alto=alto,
            codigo_error=codigo, mensaje_error=mensaje,
        )

    def estado(self) -> dict:
        """Para el health check (HU 05.5) y para los logs."""
        return {"listo": True, "modelo": self.detector.info.como_dict(),
                "peticiones": self.peticiones, "errores": self.errores}


# ----------------------------------------------------------- instancia única

_servicio: ServicioInferencia | None = None
_candado_servicio = threading.Lock()


def obtener_servicio(config: Configuracion | None = None) -> ServicioInferencia:
    """Devuelve el servicio del proceso; lo crea (y carga el modelo) solo la primera vez."""
    global _servicio
    if _servicio is None:
        with _candado_servicio:
            if _servicio is None:  # doble verificación: dos hilos no cargan dos veces
                _servicio = ServicioInferencia.iniciar(config)
    return _servicio


def _reiniciar_para_pruebas() -> None:
    global _servicio
    with _candado_servicio:
        _servicio = None