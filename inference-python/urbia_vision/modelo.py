"""
Carga única y uso del modelo de detección (HU 05.3).

`DetectorUrbia.cargar()` hace, una sola vez al iniciar el servicio:
1. Lee y valida `config/modelo.json` (HU 05.2).
2. Verifica que el archivo de pesos exista y que su SHA-256 coincida.
3. Carga YOLO en el dispositivo elegido (GPU si hay, si no CPU).
4. Verifica que las clases del modelo sean las de `clases.py`.
5. Hace un "calentamiento" (una inferencia con una imagen vacía) para que la
   primera petición real no pague la inicialización de CUDA.

Si algo de eso falla, lanza ModeloNoDisponible o ModeloInvalido y el servicio
NO arranca: es mejor no levantar que sugerir categorías con un modelo equivocado.

`detectar()` es seguro para varios hilos: un candado evita que dos peticiones
usen el modelo al mismo tiempo (Ultralytics no es thread-safe). El servidor
gRPC de la HU 05.5 atiende peticiones en un pool de hilos.
"""

from __future__ import annotations

import json
import logging
import threading
import time
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np

from urbia_vision import clases
from urbia_vision import metadatos as md
from urbia_vision.configuracion import RAIZ_SERVICIO, blindar_ultralytics
from urbia_vision.decision import Deteccion, ResultadoImagen, interpretar
from urbia_vision.errores import FalloInferencia, ModeloInvalido, ModeloNoDisponible
from urbia_vision.preprocesamiento import ImagenPreparada

log = logging.getLogger("urbia.inferencia")

blindar_ultralytics()  # antes de que cualquier parte del servicio importe ultralytics


def _crear_yolo(ruta_pesos: Path):
    """Punto único donde se instancia YOLO (en las pruebas se reemplaza)."""
    from ultralytics import YOLO

    return YOLO(str(ruta_pesos), task="detect")


def elegir_dispositivo(preferido: str) -> str:
    if preferido != "auto":
        return preferido
    try:
        import torch

        return "0" if torch.cuda.is_available() else "cpu"
    except ImportError:
        return "cpu"


@dataclass(frozen=True)
class InfoModelo:
    nombre: str
    version: str
    sha256: str
    umbral_confianza: float
    imgsz: int
    clases: dict[int, str]
    dispositivo: str
    segundos_carga: float

    def como_dict(self) -> dict:
        return {
            "nombre": self.nombre, "version": self.version, "sha256": self.sha256,
            "umbral_confianza": self.umbral_confianza, "imgsz": self.imgsz,
            "clases": self.clases, "dispositivo": self.dispositivo,
            "segundos_carga": round(self.segundos_carga, 2),
        }


@dataclass
class Caja:
    clase: str
    confianza: float
    x1: float
    y1: float
    x2: float
    y2: float


@dataclass
class SalidaModelo:
    resultado: ResultadoImagen
    cajas: list[Caja] = field(default_factory=list)
    milisegundos: float = 0.0


class DetectorUrbia:
    def __init__(self, yolo, info: InfoModelo):
        self._yolo = yolo
        self.info = info
        self._candado = threading.Lock()

    # ------------------------------------------------------------------ carga

    @classmethod
    def cargar(cls, modelo_config: Path, dispositivo: str = "auto") -> "DetectorUrbia":
        inicio = time.perf_counter()
        meta = cls._leer_metadatos(modelo_config)

        pesos = Path(meta["archivo"])
        pesos = pesos if pesos.is_absolute() else RAIZ_SERVICIO / pesos
        if not pesos.is_file():
            raise ModeloNoDisponible(
                f"No existen los pesos {pesos}. Descárgalos con: python -m scripts.descargar_modelo"
            )
        sha = md.sha256_archivo(pesos)
        if sha != meta["sha256"]:
            raise ModeloInvalido(
                f"El SHA-256 de {pesos.name} no coincide con modelo.json "
                f"(esperado {meta['sha256'][:12]}…, real {sha[:12]}…)"
            )

        disp = elegir_dispositivo(dispositivo)
        try:
            yolo = _crear_yolo(pesos)
        except Exception as e:  # pesos corruptos, versión de Ultralytics incompatible…
            raise ModeloInvalido(f"Ultralytics no pudo cargar {pesos.name}: {e}") from e

        nombres = {int(k): v for k, v in yolo.names.items()}
        if nombres != clases.NOMBRES:
            raise ModeloInvalido(f"El modelo tiene clases {nombres}, se esperaba {clases.NOMBRES}")

        info = InfoModelo(
            nombre=meta["nombre"], version=str(meta["version"]), sha256=sha,
            umbral_confianza=float(meta["umbral_confianza"]), imgsz=int(meta["imgsz"]),
            clases=dict(clases.NOMBRES), dispositivo=disp, segundos_carga=0.0,
        )
        detector = cls(yolo, info)
        detector._calentar()

        info_final = InfoModelo(**{**info.__dict__, "segundos_carga": time.perf_counter() - inicio})
        detector.info = info_final
        log.info(
            "modelo cargado nombre=%s version=%s umbral=%.2f imgsz=%d dispositivo=%s "
            "sha256=%s segundos=%.2f",
            info_final.nombre, info_final.version, info_final.umbral_confianza, info_final.imgsz,
            info_final.dispositivo, info_final.sha256[:12], info_final.segundos_carga,
        )
        return detector

    @staticmethod
    def _leer_metadatos(ruta: Path) -> dict:
        if not ruta.is_file():
            raise ModeloNoDisponible(f"No existe {ruta}. Genera modelo.json con scripts.publicar_modelo (05.2)")
        try:
            meta = json.loads(ruta.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as e:
            raise ModeloInvalido(f"{ruta.name} no es un JSON válido: {e}") from e
        errores = md.validar_metadatos(meta, clases.NOMBRES)
        if errores:
            raise ModeloInvalido(f"{ruta.name} no es válido: " + "; ".join(errores))
        return meta

    def _calentar(self) -> None:
        vacia = np.zeros((self.info.imgsz, self.info.imgsz, 3), dtype=np.uint8)
        try:
            self._predecir(vacia)
        except Exception as e:
            raise ModeloInvalido(f"El modelo falló en la inferencia de calentamiento: {e}") from e

    # ------------------------------------------------------------- inferencia

    def _predecir(self, arreglo: np.ndarray):
        with self._candado:
            return self._yolo.predict(
                source=arreglo, conf=self.info.umbral_confianza, imgsz=self.info.imgsz,
                device=self.info.dispositivo, verbose=False,
            )

    def detectar(self, imagen: ImagenPreparada) -> SalidaModelo:
        inicio = time.perf_counter()
        try:
            resultados = self._predecir(imagen.arreglo)
            r = resultados[0]
            xyxy = r.boxes.xyxy.tolist()
            ids = r.boxes.cls.tolist()
            confs = r.boxes.conf.tolist()
        except Exception as e:
            # Incluye CUDA out of memory: la petición falla, el servicio sigue vivo
            log.exception("falló la inferencia")
            raise FalloInferencia(f"El modelo no pudo procesar la imagen: {type(e).__name__}") from e

        # Cajas de vuelta a píxeles de la imagen ORIGINAL, recortadas a sus bordes
        f = 1.0 / imagen.escala
        detecciones, cajas = [], []
        for (x1, y1, x2, y2), c, s in zip(xyxy, ids, confs):
            x1, x2 = (min(max(v * f, 0.0), imagen.ancho_original) for v in (x1, x2))
            y1, y2 = (min(max(v * f, 0.0), imagen.alto_original) for v in (y1, y2))
            detecciones.append(Deteccion(int(c), float(s), (x1, y1, x2, y2)))
            cajas.append(Caja(clases.NOMBRES[int(c)], round(float(s), 4),
                              round(x1, 1), round(y1, 1), round(x2, 1), round(y2, 1)))

        resultado = interpretar(detecciones, self.info.umbral_confianza, clases.NOMBRES)
        cajas.sort(key=lambda k: k.confianza, reverse=True)
        return SalidaModelo(resultado, cajas, (time.perf_counter() - inicio) * 1000)