"""
Utilidades para preparar y validar el dataset de URBIA (HU 05.1).

Formato de trabajo: YOLO de detección (Ultralytics).

    <raiz>/
      images/{train,val,test}/<imagen>.jpg
      labels/{train,val,test}/<imagen>.txt

Cada línea de un .txt es:  <id_clase> <x_centro> <y_centro> <ancho> <alto>
con las coordenadas normalizadas entre 0 y 1 respecto al tamaño de la imagen.
Un .txt vacío significa "imagen sin objetos" (ejemplo negativo).
"""

from __future__ import annotations

import csv
import hashlib
import random
import re
import shutil
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path

EXTENSIONES_IMAGEN = {".jpg", ".jpeg", ".png", ".bmp", ".webp"}
SPLITS = ("train", "val", "test")

# Roboflow exporta las imágenes aumentadas como  foto_jpg.rf.<hash>.jpg.
# Todas las variantes de la misma foto deben caer en el mismo split,
# si no, el conjunto de prueba "ya lo vio" el modelo (fuga de datos).
_PATRON_ROBOFLOW = re.compile(r"\.rf\.[0-9a-fA-F]+$")
_PATRON_EXT_EN_NOMBRE = re.compile(r"_(jpg|jpeg|png|bmp|webp)$", re.IGNORECASE)


# ---------------------------------------------------------------------------
# Validación de etiquetas
# ---------------------------------------------------------------------------

def validar_linea_yolo(linea: str, num_clases: int) -> str | None:
    """Devuelve un mensaje de error si la línea no es YOLO válido, o None."""
    partes = linea.split()
    if len(partes) != 5:
        return f"se esperaban 5 valores y hay {len(partes)}"
    try:
        id_clase = int(partes[0])
    except ValueError:
        return f"id de clase no entero: '{partes[0]}'"
    if not 0 <= id_clase < num_clases:
        return f"id de clase {id_clase} fuera de rango (0..{num_clases - 1})"
    try:
        cx, cy, w, h = (float(v) for v in partes[1:])
    except ValueError:
        return "coordenadas no numéricas"
    for nombre, valor in (("x_centro", cx), ("y_centro", cy), ("ancho", w), ("alto", h)):
        if not 0.0 <= valor <= 1.0:
            return f"{nombre}={valor} fuera de [0, 1]"
    if w <= 0 or h <= 0:
        return "ancho y alto deben ser mayores que 0"
    return None


# ---------------------------------------------------------------------------
# Remapeo de clases de una fuente pública a las clases de URBIA
# ---------------------------------------------------------------------------

def remapear_lineas(
    lineas: list[str],
    nombres_origen: list[str],
    mapeo: dict[str, str],
    ids_destino: dict[str, int],
) -> list[str]:
    """
    Traduce líneas YOLO de la numeración de la fuente a la de URBIA.
    Las clases que no están en `mapeo` se descartan.
    """
    salida: list[str] = []
    for linea in lineas:
        partes = linea.split()
        if not partes:
            continue
        id_origen = int(partes[0])
        if not 0 <= id_origen < len(nombres_origen):
            raise ValueError(f"id {id_origen} no existe en las clases de la fuente")
        nombre_origen = nombres_origen[id_origen]
        if nombre_origen not in mapeo:
            continue
        id_nuevo = ids_destino[mapeo[nombre_origen]]
        # Roboflow a veces exporta polígonos (segmentación). Se convierten
        # a caja: min/max de los puntos.
        coords = [float(v) for v in partes[1:]]
        if len(coords) > 4:
            xs, ys = coords[0::2], coords[1::2]
            x1, x2, y1, y2 = min(xs), max(xs), min(ys), max(ys)
            coords = [(x1 + x2) / 2, (y1 + y2) / 2, x2 - x1, y2 - y1]
        salida.append(f"{id_nuevo} " + " ".join(f"{c:.6f}" for c in coords))
    return salida


# ---------------------------------------------------------------------------
# División train / val / test
# ---------------------------------------------------------------------------

def clave_grupo(nombre_archivo: str) -> str:
    """Nombre base de la foto original, sin sufijos de aumentación de Roboflow."""
    base = Path(nombre_archivo).stem
    base = _PATRON_ROBOFLOW.sub("", base)
    base = _PATRON_EXT_EN_NOMBRE.sub("", base)
    return base


def dividir(
    nombres: list[str],
    proporciones: tuple[float, float, float] = (0.7, 0.2, 0.1),
    semilla: int = 42,
) -> dict[str, list[str]]:
    """
    Divide archivos en train/val/test de forma reproducible (misma semilla
    = misma división) y agrupando las variantes de una misma foto.
    """
    if abs(sum(proporciones) - 1.0) > 1e-6:
        raise ValueError(f"Las proporciones deben sumar 1, suman {sum(proporciones)}")

    grupos: dict[str, list[str]] = defaultdict(list)
    for n in sorted(nombres):
        grupos[clave_grupo(n)].append(n)

    claves = sorted(grupos)
    random.Random(semilla).shuffle(claves)

    total = len(claves)
    n_train = round(total * proporciones[0])
    n_val = round(total * proporciones[1])
    cortes = {
        "train": claves[:n_train],
        "val": claves[n_train:n_train + n_val],
        "test": claves[n_train + n_val:],
    }
    return {split: [a for k in ks for a in grupos[k]] for split, ks in cortes.items()}


# ---------------------------------------------------------------------------
# Lectura de un export de Roboflow (formato YOLOv8) y armado del dataset
# ---------------------------------------------------------------------------

def listar_pares(raiz_fuente: Path) -> list[tuple[Path, Path | None]]:
    """
    Busca todas las imágenes de un export YOLO (cualquier subcarpeta
    train/valid/test) y su .txt correspondiente en la carpeta 'labels' hermana.
    """
    pares = []
    for img in sorted(raiz_fuente.rglob("*")):
        if img.suffix.lower() not in EXTENSIONES_IMAGEN or "images" not in img.parts:
            continue
        partes = list(img.parts)
        idx = len(partes) - 1 - partes[::-1].index("images")
        partes[idx] = "labels"
        etiqueta = Path(*partes).with_suffix(".txt")
        pares.append((img, etiqueta if etiqueta.exists() else None))
    return pares


def sha256_archivo(ruta: Path) -> str:
    h = hashlib.sha256()
    with ruta.open("rb") as f:
        for bloque in iter(lambda: f.read(1 << 20), b""):
            h.update(bloque)
    return h.hexdigest()


@dataclass
class ResumenPreparacion:
    imagenes: int = 0
    cajas_por_clase: Counter = field(default_factory=Counter)
    cajas_descartadas: int = 0
    por_split: Counter = field(default_factory=Counter)


def preparar(
    fuentes: dict[str, tuple[Path, list[str], dict[str, str]]],
    destino: Path,
    ids_destino: dict[str, int],
    manifiesto: Path,
    proporciones: tuple[float, float, float] = (0.7, 0.2, 0.1),
    semilla: int = 42,
) -> ResumenPreparacion:
    """
    fuentes: {nombre_fuente: (ruta_export, nombres_clases_origen, mapeo)}

    Copia imágenes y etiquetas remapeadas a `destino`, divide en
    train/val/test y escribe un manifiesto CSV con el split de cada imagen.
    """
    resumen = ResumenPreparacion()
    filas_manifiesto: list[dict[str, str]] = []

    for split in SPLITS:
        (destino / "images" / split).mkdir(parents=True, exist_ok=True)
        (destino / "labels" / split).mkdir(parents=True, exist_ok=True)

    for nombre_fuente, (ruta, nombres_origen, mapeo) in sorted(fuentes.items()):
        pares = listar_pares(ruta)
        # Prefijo con la fuente para evitar choques de nombres entre datasets
        por_nombre = {f"{nombre_fuente}__{img.name}": (img, lbl) for img, lbl in pares}
        division = dividir(list(por_nombre), proporciones, semilla)

        for split, archivos in division.items():
            for nombre_nuevo in archivos:
                img, lbl = por_nombre[nombre_nuevo]
                lineas = lbl.read_text(encoding="utf-8").splitlines() if lbl else []
                nuevas = remapear_lineas(lineas, nombres_origen, mapeo, ids_destino)
                resumen.cajas_descartadas += sum(1 for l in lineas if l.strip()) - len(nuevas)
                for l in nuevas:
                    resumen.cajas_por_clase[int(l.split()[0])] += 1

                dest_img = destino / "images" / split / nombre_nuevo
                shutil.copy2(img, dest_img)
                dest_lbl = destino / "labels" / split / Path(nombre_nuevo).with_suffix(".txt").name
                dest_lbl.write_text("\n".join(nuevas) + ("\n" if nuevas else ""), encoding="utf-8")

                resumen.imagenes += 1
                resumen.por_split[split] += 1
                filas_manifiesto.append({
                    "archivo": nombre_nuevo,
                    "split": split,
                    "fuente": nombre_fuente,
                    "sha256": sha256_archivo(dest_img),
                })

    escribir_manifiesto(manifiesto, filas_manifiesto)
    return resumen


def escribir_manifiesto(ruta: Path, filas: list[dict[str, str]]) -> None:
    ruta.parent.mkdir(parents=True, exist_ok=True)
    with ruta.open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["archivo", "split", "fuente", "sha256"])
        w.writeheader()
        for fila in sorted(filas, key=lambda r: (r["split"], r["archivo"])):
            w.writerow(fila)


# ---------------------------------------------------------------------------
# Validación de un dataset ya preparado
# ---------------------------------------------------------------------------

@dataclass
class ReporteValidacion:
    errores: list[str] = field(default_factory=list)
    imagenes_por_split: Counter = field(default_factory=Counter)
    cajas: dict[str, Counter] = field(default_factory=lambda: defaultdict(Counter))
    negativas_por_split: Counter = field(default_factory=Counter)

    @property
    def ok(self) -> bool:
        return not self.errores


def validar_dataset(raiz: Path, num_clases: int, manifiesto: Path | None = None) -> ReporteValidacion:
    rep = ReporteValidacion()
    vistos: dict[str, str] = {}  # clave_grupo -> split, para detectar fugas

    for split in SPLITS:
        dir_img = raiz / "images" / split
        dir_lbl = raiz / "labels" / split
        if not dir_img.is_dir():
            rep.errores.append(f"falta la carpeta {dir_img}")
            continue

        imagenes = sorted(p for p in dir_img.iterdir() if p.suffix.lower() in EXTENSIONES_IMAGEN)
        if not imagenes:
            rep.errores.append(f"el split '{split}' no tiene imágenes")

        for img in imagenes:
            rep.imagenes_por_split[split] += 1

            grupo = clave_grupo(img.name)
            if grupo in vistos and vistos[grupo] != split:
                rep.errores.append(
                    f"fuga de datos: '{grupo}' aparece en '{vistos[grupo]}' y en '{split}'"
                )
            vistos.setdefault(grupo, split)

            lbl = dir_lbl / img.with_suffix(".txt").name
            if not lbl.exists():
                rep.errores.append(f"{split}/{img.name}: no tiene archivo de etiquetas")
                continue
            lineas = [l for l in lbl.read_text(encoding="utf-8").splitlines() if l.strip()]
            if not lineas:
                rep.negativas_por_split[split] += 1
            for n, linea in enumerate(lineas, start=1):
                error = validar_linea_yolo(linea, num_clases)
                if error:
                    rep.errores.append(f"{split}/{lbl.name}:{n}: {error}")
                else:
                    rep.cajas[split][int(linea.split()[0])] += 1

        if dir_lbl.is_dir():
            for lbl in dir_lbl.glob("*.txt"):
                if not any((dir_img / (lbl.stem + ext)).exists() for ext in EXTENSIONES_IMAGEN):
                    rep.errores.append(f"{split}/{lbl.name}: etiqueta sin imagen")

    if manifiesto is not None:
        _validar_contra_manifiesto(raiz, manifiesto, rep)
    return rep


def _validar_contra_manifiesto(raiz: Path, manifiesto: Path, rep: ReporteValidacion) -> None:
    """Comprueba que el conjunto de prueba sea exactamente el registrado."""
    if not manifiesto.exists():
        rep.errores.append(f"no existe el manifiesto {manifiesto}")
        return
    with manifiesto.open(encoding="utf-8") as f:
        esperado = {(r["split"], r["archivo"]): r["sha256"] for r in csv.DictReader(f)}

    for (split, archivo), sha in esperado.items():
        ruta = raiz / "images" / split / archivo
        if not ruta.exists():
            rep.errores.append(f"manifiesto: falta {split}/{archivo}")
        elif split == "test" and sha256_archivo(ruta) != sha:
            rep.errores.append(f"manifiesto: {split}/{archivo} cambió (sha256 distinto)")

    for split in SPLITS:
        dir_img = raiz / "images" / split
        if dir_img.is_dir():
            for img in dir_img.iterdir():
                if img.suffix.lower() in EXTENSIONES_IMAGEN and (split, img.name) not in esperado:
                    rep.errores.append(f"manifiesto: {split}/{img.name} no está registrado")