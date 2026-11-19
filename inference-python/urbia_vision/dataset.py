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

_SPLITS_ORIGEN = {"train": "train", "valid": "val", "val": "val", "test": "test"}


def split_de_origen(img: Path) -> str | None:
    """
    Split que la fuente le asignó a la imagen, según la carpeta que está
    justo antes de 'images' (train/, valid/, test/ en los exports de Roboflow).
    """
    partes = img.parts
    idx = len(partes) - 1 - partes[::-1].index("images")
    return _SPLITS_ORIGEN.get(partes[idx - 1].lower()) if idx > 0 else None


def dividir_por_origen(por_nombre: dict[str, tuple[Path, Path | None]]) -> dict[str, list[str]]:
    """
    Respeta la división del export original. En Roboflow las aumentaciones
    (ruido, rotación, etc.) solo se aplican a train/, así que val y test
    quedan con fotos sin alterar y sin variantes de las fotos de train.
    """
    division: dict[str, list[str]] = {s: [] for s in SPLITS}
    sin_split = []
    for nombre, (img, _) in sorted(por_nombre.items()):
        split = split_de_origen(img)
        if split is None:
            sin_split.append(nombre)
        else:
            division[split].append(nombre)
    if sin_split:
        raise ValueError(
            f"{len(sin_split)} imágenes no están en carpetas train/valid/test, "
            f"por ejemplo '{sin_split[0]}'. Usa la división aleatoria."
        )
    return division

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
    duplicados_quitados: list = field(default_factory=list)


def preparar(
    fuentes: dict[str, tuple[Path, list[str], dict[str, str]]],
    destino: Path,
    ids_destino: dict[str, int],
    manifiesto: Path,
    proporciones: tuple[float, float, float] = (0.7, 0.2, 0.1),
    semilla: int = 42,
    split_original: bool = False,
    registro_duplicados: Path | None = None,
) -> ResumenPreparacion:
    """
    fuentes: {nombre_fuente: (ruta_export, nombres_clases_origen, mapeo)}

    Copia imágenes y etiquetas remapeadas a `destino`, divide en
    train/val/test y escribe un manifiesto CSV con el split de cada imagen.

    split_original=True respeta la división del export (recomendado con
    exports de Roboflow que traen aumentaciones). Si es False, se vuelve a
    dividir todo al azar con `proporciones` y `semilla`.

    registro_duplicados: si se indica, quita de val/test las copias de fotos
    de un split anterior (ver quitar_duplicados) y guarda ahí qué se quitó.
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
        if split_original:
            division = dividir_por_origen(por_nombre)
        else:
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

    if registro_duplicados is not None:
        quitados = quitar_duplicados(destino)
        fuera = {(d.split, d.nombre) for d in quitados}
        for d in quitados:
            resumen.por_split[d.split] -= 1
            resumen.imagenes -= 1
        # Recontar cajas con lo que quedó en disco
        resumen.cajas_por_clase = Counter(
            int(linea.split()[0])
            for lbl in (destino / "labels").glob("*/*.txt")
            for linea in lbl.read_text(encoding="utf-8").splitlines() if linea.strip()
        )
        filas_manifiesto = [f for f in filas_manifiesto if (f["split"], f["archivo"]) not in fuera]
        resumen.duplicados_quitados = quitados
        escribir_registro_duplicados(registro_duplicados, quitados)

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

# Distancia de Hamming máxima (de 64 bits) para considerar dos imágenes
# "casi iguales" (recompresión, cambio de tamaño, ruido leve).
DISTANCIA_CASI_DUPLICADO = 5


def huella_visual(ruta: Path) -> int | None:
    """
    Huella perceptual (dHash de 64 bits): reduce la imagen a 9x8 en grises y
    compara cada píxel con su vecino. Dos fotos iguales o casi iguales dan
    huellas con pocos bits distintos aunque cambien nombre, tamaño o calidad.
    """
    from PIL import Image, UnidentifiedImageError

    try:
        with Image.open(ruta) as im:
            im.draft("L", (64, 64))  # en JPG decodifica a baja resolución: mucho más rápido
            pixeles = im.convert("L").resize((9, 8)).tobytes()
    except (UnidentifiedImageError, OSError):
        return None
    bits = 0
    for fila in range(8):
        for col in range(8):
            izq, der = pixeles[fila * 9 + col], pixeles[fila * 9 + col + 1]
            bits = (bits << 1) | (izq > der)
    return bits


def distancia(a: int, b: int) -> int:
    return bin(a ^ b).count("1")


@dataclass
class ReporteValidacion:
    errores: list[str] = field(default_factory=list)
    advertencias: list[str] = field(default_factory=list)
    imagenes_por_split: Counter = field(default_factory=Counter)
    cajas: dict[str, Counter] = field(default_factory=lambda: defaultdict(Counter))
    negativas_por_split: Counter = field(default_factory=Counter)

    @property
    def ok(self) -> bool:
        return not self.errores


def validar_dataset(raiz: Path, num_clases: int, manifiesto: Path | None = None) -> ReporteValidacion:
    rep = ReporteValidacion()
    rutas_por_split: dict[str, list[Path]] = {}

    for split in SPLITS:
        dir_img = raiz / "images" / split
        dir_lbl = raiz / "labels" / split
        if not dir_img.is_dir():
            rep.errores.append(f"falta la carpeta {dir_img}")
            continue

        imagenes = sorted(p for p in dir_img.iterdir() if p.suffix.lower() in EXTENSIONES_IMAGEN)
        rutas_por_split[split] = imagenes
        if not imagenes:
            rep.errores.append(f"el split '{split}' no tiene imágenes")

        for img in imagenes:
            rep.imagenes_por_split[split] += 1

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

    _detectar_fugas(rutas_por_split, rep)

    if manifiesto is not None:
        _validar_contra_manifiesto(raiz, manifiesto, rep)
    return rep


@dataclass(frozen=True)
class Duplicado:
    """`split/nombre` es una copia de `split_ref/nombre_ref`, que está en un split anterior."""
    split_ref: str
    nombre_ref: str
    split: str
    nombre: str
    tipo: str          # "idéntico" (mismo SHA-256) o "casi idéntico" (huella visual)
    bits: int          # distancia entre huellas (0 en los idénticos)


def buscar_duplicados(rutas_por_split: dict[str, list[Path]]) -> tuple[list[Duplicado], list[str]]:
    """
    Busca la misma foto en dos splits distintos comparando el CONTENIDO,
    no el nombre (muchas fotos de internet se llaman igual: image.jpg, images12.jpg,
    y una misma foto de banco de imágenes aparece con nombres distintos).

    Devuelve (duplicados, ilegibles). El orden de prioridad es train < val < test:
    el duplicado siempre es la copia del split posterior.
    """
    sha: dict[str, list[tuple[str, str]]] = {}          # split -> [(nombre, sha256)]
    huellas: dict[str, dict[str, int]] = {}             # split -> {nombre: dHash}
    ilegibles: list[str] = []

    for split, rutas in rutas_por_split.items():
        sha[split], huellas[split] = [], {}
        for ruta in rutas:
            sha[split].append((ruta.name, sha256_archivo(ruta)))
            h = huella_visual(ruta)
            if h is None:
                ilegibles.append(f"{split}/{ruta.name}")
            else:
                huellas[split][ruta.name] = h

    duplicados: list[Duplicado] = []
    for a, b in (("train", "val"), ("train", "test"), ("val", "test")):
        por_sha_a: dict[str, str] = {}
        for nombre, valor in sha.get(a, []):
            por_sha_a.setdefault(valor, nombre)
        huellas_a = list(huellas.get(a, {}).items())
        for nombre_b, sha_b in sha.get(b, []):
            ref = por_sha_a.get(sha_b)
            if ref is not None:
                duplicados.append(Duplicado(a, ref, b, nombre_b, "idéntico", 0))
                continue
            h_b = huellas.get(b, {}).get(nombre_b)
            if h_b is None:
                continue
            for nombre_a, h_a in huellas_a:
                d = distancia(h_a, h_b)
                if d <= DISTANCIA_CASI_DUPLICADO:
                    duplicados.append(Duplicado(a, nombre_a, b, nombre_b, "casi idéntico", d))
                    break
    return duplicados, ilegibles


def _detectar_fugas(rutas_por_split: dict[str, list[Path]], rep: ReporteValidacion) -> None:
    """
    - Archivo idéntico (mismo SHA-256) en dos splits           -> ERROR (fuga segura).
    - Huella visual casi igual (<= 5 bits de 64) en dos splits -> ADVERTENCIA
      (probable la misma foto recomprimida o redimensionada; revisar a mano).
    """
    duplicados, ilegibles = buscar_duplicados(rutas_por_split)
    for ruta in ilegibles:
        rep.advertencias.append(f"{ruta}: no se pudo leer como imagen")
    for d in duplicados:
        texto = f"{d.split_ref}/{d.nombre_ref} ~ {d.split}/{d.nombre}"
        if d.tipo == "idéntico":
            rep.errores.append(f"fuga de datos (archivo idéntico): {texto}")
        else:
            rep.advertencias.append(f"posible duplicado ({d.bits} bits): {texto}")


def quitar_duplicados(destino: Path) -> list[Duplicado]:
    """
    Borra de val y test las imágenes (y sus etiquetas) que son copia idéntica o
    casi idéntica de una imagen de un split anterior. Train nunca se toca.
    """
    rutas = {
        s: sorted(p for p in (destino / "images" / s).iterdir() if p.suffix.lower() in EXTENSIONES_IMAGEN)
        for s in SPLITS if (destino / "images" / s).is_dir()
    }
    duplicados, _ = buscar_duplicados(rutas)
    vistos: set[tuple[str, str]] = set()
    quitados: list[Duplicado] = []
    for d in duplicados:
        if (d.split, d.nombre) in vistos:
            continue
        vistos.add((d.split, d.nombre))
        (destino / "images" / d.split / d.nombre).unlink(missing_ok=True)
        (destino / "labels" / d.split / Path(d.nombre).with_suffix(".txt").name).unlink(missing_ok=True)
        quitados.append(d)
    return quitados


def escribir_registro_duplicados(ruta: Path, quitados: list[Duplicado]) -> None:
    ruta.parent.mkdir(parents=True, exist_ok=True)
    with ruta.open("w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["split", "archivo_quitado", "tipo", "bits", "split_original", "archivo_original"])
        for d in sorted(quitados, key=lambda d: (d.split, d.nombre)):
            w.writerow([d.split, d.nombre, d.tipo, d.bits, d.split_ref, d.nombre_ref])

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