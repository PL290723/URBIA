"""
Detección del hardware disponible y recomendación de variante YOLO (HU 05.2).
"""

from __future__ import annotations

import os
import platform
from dataclasses import asdict, dataclass

VRAM_MINIMA_SMALL_GB = 3.5     # una GPU de "4 GB" reporta ~3.7-4.0 GB
VRAM_AUTOBATCH_GB = 6.0        # por debajo, batch fijo para evitar quedarse sin VRAM


@dataclass
class InfoHardware:
    sistema: str
    python: str
    torch: str
    ultralytics: str
    cuda_disponible: bool
    gpu: str | None
    vram_gb: float | None
    nucleos_cpu: int

    def como_dict(self) -> dict:
        return asdict(self)


@dataclass
class Recomendacion:
    modelo_base: str
    device: str
    batch: int
    nota: str


def detectar() -> InfoHardware:
    import torch
    import ultralytics

    cuda = torch.cuda.is_available()
    gpu = vram = None
    if cuda:
        props = torch.cuda.get_device_properties(0)
        gpu = props.name
        vram = round(props.total_memory / 1024**3, 1)

    return InfoHardware(
        sistema=f"{platform.system()} {platform.release()}",
        python=platform.python_version(),
        torch=torch.__version__,
        ultralytics=ultralytics.__version__,
        cuda_disponible=cuda,
        gpu=gpu,
        vram_gb=vram,
        nucleos_cpu=os.cpu_count() or 1,
    )


def recomendar(info: InfoHardware) -> Recomendacion:
    """
    - GPU con 6 GB o más  -> yolo26s, batch automático (-1).
    - GPU con 4 GB        -> yolo26s, batch fijo de 8 (con AMP cabe en 4 GB).
    - GPU con menos       -> yolo26n, batch fijo de 8.
    - Sin GPU             -> yolo26n en CPU (lento; mejor Colab).
    """
    vram = info.vram_gb or 0
    if info.cuda_disponible and vram >= VRAM_AUTOBATCH_GB:
        return Recomendacion(
            modelo_base="yolo26s.pt", device="0", batch=-1,
            nota=f"GPU {info.gpu} con {vram} GB: variante small con batch automático.",
        )
    if info.cuda_disponible and vram >= VRAM_MINIMA_SMALL_GB:
        return Recomendacion(
            modelo_base="yolo26s.pt", device="0", batch=8,
            nota=f"GPU {info.gpu} con {vram} GB: variante small con batch 8. "
                 "Si aparece 'CUDA out of memory', usar --batch 4.",
        )
    if info.cuda_disponible:
        return Recomendacion(
            modelo_base="yolo26n.pt", device="0", batch=8,
            nota=f"GPU {info.gpu} con {vram} GB: se usa nano para no quedarse sin VRAM.",
        )
    return Recomendacion(
        modelo_base="yolo26n.pt", device="cpu", batch=8,
        nota="Sin GPU CUDA: entrenar en CPU es muy lento. Se recomienda Google Colab "
             "(GPU T4 gratuita) para el entrenamiento completo; la inferencia sí funciona en CPU.",
    )