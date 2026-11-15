"""
Evaluación del detector de URBIA por clase (HU 05.2).

Todo aquí es Python puro (sin torch) para poder probarlo con pytest:
- emparejar predicciones con etiquetas reales por IoU,
- calcular precisión, recall y F1 por clase,
- construir la matriz de confusión (incluye la fila/columna "fondo"),
- barrer umbrales de confianza y elegir el mejor,
- listar falsos positivos y falsos negativos para inspeccionarlos.
"""

from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path

from urbia_vision.decision import Deteccion, interpretar

Caja = tuple[float, float, float, float]  # x1, y1, x2, y2


@dataclass(frozen=True)
class Verdad:
    clase: int
    caja: Caja


@dataclass
class ImagenEvaluada:
    nombre: str
    verdades: list[Verdad]
    predicciones: list[Deteccion]


# ---------------------------------------------------------------- utilidades

def iou(a: Caja, b: Caja) -> float:
    ix1, iy1 = max(a[0], b[0]), max(a[1], b[1])
    ix2, iy2 = min(a[2], b[2]), min(a[3], b[3])
    inter = max(0.0, ix2 - ix1) * max(0.0, iy2 - iy1)
    area_a = max(0.0, a[2] - a[0]) * max(0.0, a[3] - a[1])
    area_b = max(0.0, b[2] - b[0]) * max(0.0, b[3] - b[1])
    union = area_a + area_b - inter
    return inter / union if union > 0 else 0.0


def yolo_a_caja(cx: float, cy: float, w: float, h: float, ancho: int, alto: int) -> Caja:
    """Convierte una caja YOLO normalizada a píxeles (x1, y1, x2, y2)."""
    return (
        (cx - w / 2) * ancho, (cy - h / 2) * alto,
        (cx + w / 2) * ancho, (cy + h / 2) * alto,
    )


def leer_verdades(ruta_txt: Path, ancho: int, alto: int) -> list[Verdad]:
    if not ruta_txt.exists():
        return []
    verdades = []
    for linea in ruta_txt.read_text(encoding="utf-8").splitlines():
        partes = linea.split()
        if len(partes) == 5:
            c, cx, cy, w, h = int(partes[0]), *map(float, partes[1:])
            verdades.append(Verdad(c, yolo_a_caja(cx, cy, w, h, ancho, alto)))
    return verdades


# -------------------------------------------------------------- emparejamiento

@dataclass
class Emparejamiento:
    """Resultado de comparar una imagen con un umbral dado."""
    # pares (clase_predicha, clase_real); -1 representa "fondo"
    pares: list[tuple[int, int]] = field(default_factory=list)
    falsos_positivos: list[Deteccion] = field(default_factory=list)
    falsos_negativos: list[Verdad] = field(default_factory=list)
    confusiones: list[tuple[Deteccion, Verdad]] = field(default_factory=list)


FONDO = -1


def emparejar(img: ImagenEvaluada, umbral: float, umbral_iou: float = 0.5) -> Emparejamiento:
    """
    Emparejamiento voraz por confianza, sin importar la clase, para poder
    distinguir "detectó algo en el lugar correcto pero con la clase equivocada"
    (confusión) de "detectó algo donde no había nada" (falso positivo).
    """
    res = Emparejamiento()
    preds = sorted((p for p in img.predicciones if p.confianza >= umbral),
                   key=lambda p: p.confianza, reverse=True)
    usadas: set[int] = set()

    for p in preds:
        mejor_i, mejor_iou = -1, umbral_iou
        for i, v in enumerate(img.verdades):
            if i in usadas:
                continue
            valor = iou(p.caja, v.caja)
            if valor >= mejor_iou:
                mejor_i, mejor_iou = i, valor
        if mejor_i == -1:
            res.pares.append((p.clase, FONDO))
            res.falsos_positivos.append(p)
        else:
            usadas.add(mejor_i)
            v = img.verdades[mejor_i]
            res.pares.append((p.clase, v.clase))
            if p.clase != v.clase:
                res.confusiones.append((p, v))

    for i, v in enumerate(img.verdades):
        if i not in usadas:
            res.pares.append((FONDO, v.clase))
            res.falsos_negativos.append(v)
    return res


# ------------------------------------------------------------------- métricas

@dataclass
class MetricasClase:
    tp: int = 0
    fp: int = 0
    fn: int = 0

    @property
    def precision(self) -> float:
        return self.tp / (self.tp + self.fp) if self.tp + self.fp else 0.0

    @property
    def recall(self) -> float:
        return self.tp / (self.tp + self.fn) if self.tp + self.fn else 0.0

    @property
    def f1(self) -> float:
        p, r = self.precision, self.recall
        return 2 * p * r / (p + r) if p + r else 0.0

    def como_dict(self) -> dict:
        return {
            "tp": self.tp, "fp": self.fp, "fn": self.fn,
            "precision": round(self.precision, 4),
            "recall": round(self.recall, 4),
            "f1": round(self.f1, 4),
        }


@dataclass
class ResultadoEvaluacion:
    umbral: float
    por_clase: dict[int, MetricasClase]
    # matriz[pred][real], con FONDO (-1) incluido
    matriz: dict[int, dict[int, int]]

    @property
    def f1_macro(self) -> float:
        return sum(m.f1 for m in self.por_clase.values()) / len(self.por_clase)


def evaluar(imagenes: list[ImagenEvaluada], clases: list[int], umbral: float,
            umbral_iou: float = 0.5) -> ResultadoEvaluacion:
    por_clase = {c: MetricasClase() for c in clases}
    etiquetas = clases + [FONDO]
    matriz = {p: {r: 0 for r in etiquetas} for p in etiquetas}

    for img in imagenes:
        for pred, real in emparejar(img, umbral, umbral_iou).pares:
            matriz[pred][real] += 1
            if pred == real:
                por_clase[pred].tp += 1
            else:
                if pred != FONDO:
                    por_clase[pred].fp += 1
                if real != FONDO:
                    por_clase[real].fn += 1
    return ResultadoEvaluacion(umbral, por_clase, matriz)


def barrer_umbrales(imagenes: list[ImagenEvaluada], clases: list[int],
                    umbrales: list[float] | None = None,
                    umbral_iou: float = 0.5) -> list[ResultadoEvaluacion]:
    umbrales = umbrales or [round(0.05 * i, 2) for i in range(1, 20)]  # 0.05 .. 0.95
    return [evaluar(imagenes, clases, u, umbral_iou) for u in umbrales]


def elegir_umbral(resultados: list[ResultadoEvaluacion]) -> ResultadoEvaluacion:
    """El umbral con mayor F1 macro; en empate, el más alto (menos falsas alarmas)."""
    return max(resultados, key=lambda r: (round(r.f1_macro, 6), r.umbral))


# ------------------------------------------------ evaluación a nivel de reporte

def clase_esperada(verdades: list[Verdad]) -> int:
    """La categoría 'correcta' de la foto: la clase con más cajas, o FONDO."""
    if not verdades:
        return FONDO
    conteo: dict[int, int] = defaultdict(int)
    for v in verdades:
        conteo[v.clase] += 1
    return max(sorted(conteo), key=lambda c: conteo[c])


def evaluar_por_imagen(imagenes: list[ImagenEvaluada], umbral: float,
                       nombres: dict[int, str]) -> dict:
    """
    Aplica la regla de decisión (decision.interpretar) a cada foto y la
    compara con la categoría esperada. Es lo que verá el usuario final:
    una sugerencia de categoría por reporte.
    """
    ids = {v: k for k, v in nombres.items()}
    etiquetas = list(nombres) + [FONDO]
    matriz = {p: {r: 0 for r in etiquetas} for p in etiquetas}
    aciertos = 0
    for img in imagenes:
        res = interpretar(img.predicciones, umbral, nombres)
        pred = ids[res.clase_detectada] if res.clase_detectada else FONDO
        real = clase_esperada(img.verdades)
        matriz[pred][real] += 1
        aciertos += pred == real
    total = len(imagenes)
    return {
        "imagenes": total,
        "exactitud": round(aciertos / total, 4) if total else 0.0,
        "matriz": matriz,
    }