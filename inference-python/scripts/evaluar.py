"""
Evalúa el detector de URBIA sobre el conjunto de prueba (HU 05.2).

Hace tres cosas:
1. Métricas oficiales de Ultralytics por clase (precisión, recall, mAP50,
   mAP50-95) y sus gráficas (matriz de confusión, curvas PR).
2. Barrido de umbrales de confianza (0.05 a 0.95) y elección del umbral con
   mayor F1 macro, con matriz de confusión propia a ese umbral y la exactitud
   de la sugerencia de categoría por foto.
3. Copia ejemplos de falsos positivos, falsos negativos y confusiones con las
   cajas dibujadas (verde = real, rojo = predicción) para revisarlos a mano.

Uso (desde inference-python/):
    python -m scripts.evaluar --pesos runs/urbia/entrenamiento/weights/best.pt

Salida: <carpeta del run>/evaluacion_test/
    metricas.json, reporte.md, ultralytics/ (gráficas),
    falsos_positivos/, falsos_negativos/, confusiones/
"""

from __future__ import annotations

import argparse
import json
import shutil
from datetime import datetime
from pathlib import Path

import yaml

from urbia_vision import clases
from urbia_vision.decision import Deteccion
from urbia_vision.evaluacion import (
    FONDO,
    ImagenEvaluada,
    barrer_umbrales,
    elegir_umbral,
    emparejar,
    evaluar_por_imagen,
    leer_verdades,
)

RAIZ_SERVICIO = Path(__file__).resolve().parents[1]
EXT = {".jpg", ".jpeg", ".png", ".bmp", ".webp"}


def nombre_etiqueta(i: int) -> str:
    return "fondo" if i == FONDO else clases.NOMBRES[i]


def metricas_ultralytics(modelo, data: str, split: str, imgsz: int, batch: int,
                         salida: Path) -> dict:
    m = modelo.val(data=data, split=split, imgsz=imgsz, batch=batch, plots=True,
                   project=str(salida), name="ultralytics", exist_ok=True, verbose=False)
    por_clase = {}
    for i, id_clase in enumerate(m.box.ap_class_index):
        p, r, ap50, ap = m.box.class_result(i)
        por_clase[clases.NOMBRES[int(id_clase)]] = {
            "precision": round(float(p), 4), "recall": round(float(r), 4),
            "mAP50": round(float(ap50), 4), "mAP50_95": round(float(ap), 4),
        }
    return {
        "global": {
            "precision": round(float(m.box.mp), 4), "recall": round(float(m.box.mr), 4),
            "mAP50": round(float(m.box.map50), 4), "mAP50_95": round(float(m.box.map), 4),
        },
        "por_clase": por_clase,
    }


def predecir(modelo, dir_imagenes: Path, dir_etiquetas: Path, imgsz: int,
             batch: int) -> list[ImagenEvaluada]:
    """
    Se pasa la CARPETA (no una lista de rutas): con una lista, Ultralytics
    carga todas las imágenes y las manda a la GPU en un solo lote, lo que
    agota la VRAM. Con la carpeta procesa de `batch` en `batch`.
    """
    imagenes = []
    # conf baja para poder barrer umbrales después
    for r in modelo.predict(source=str(dir_imagenes), conf=0.01, imgsz=imgsz,
                            batch=batch, stream=True, verbose=False):
        ruta = Path(r.path)
        if ruta.suffix.lower() not in EXT:
            continue
        alto, ancho = r.orig_shape
        preds = [
            Deteccion(int(c), float(s), tuple(float(v) for v in caja))
            for caja, c, s in zip(r.boxes.xyxy.tolist(), r.boxes.cls.tolist(), r.boxes.conf.tolist())
        ]
        verdades = leer_verdades(dir_etiquetas / f"{ruta.stem}.txt", ancho, alto)
        imagenes.append(ImagenEvaluada(ruta.name, verdades, preds))
    return sorted(imagenes, key=lambda i: i.nombre)


def dibujar(ruta_img: Path, img: ImagenEvaluada, umbral: float, destino: Path) -> None:
    import cv2

    imagen = cv2.imread(str(ruta_img))
    if imagen is None:
        return
    for v in img.verdades:
        x1, y1, x2, y2 = map(int, v.caja)
        cv2.rectangle(imagen, (x1, y1), (x2, y2), (0, 200, 0), 3)
        cv2.putText(imagen, nombre_etiqueta(v.clase), (x1, max(y1 - 8, 15)),
                    cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 200, 0), 2)
    for p in img.predicciones:
        if p.confianza < umbral:
            continue
        x1, y1, x2, y2 = map(int, p.caja)
        cv2.rectangle(imagen, (x1, y1), (x2, y2), (0, 0, 230), 2)
        cv2.putText(imagen, f"{nombre_etiqueta(p.clase)} {p.confianza:.2f}", (x1, y2 + 22),
                    cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 0, 230), 2)
    destino.parent.mkdir(parents=True, exist_ok=True)
    cv2.imwrite(str(destino), imagen)


def guardar_ejemplos(imagenes, dir_imagenes: Path, umbral: float, salida: Path, n: int) -> dict:
    grupos = {"falsos_positivos": [], "falsos_negativos": [], "confusiones": []}
    for img in imagenes:
        e = emparejar(img, umbral)
        if e.falsos_positivos:
            grupos["falsos_positivos"].append((len(e.falsos_positivos), img))
        if e.falsos_negativos:
            grupos["falsos_negativos"].append((len(e.falsos_negativos), img))
        if e.confusiones:
            grupos["confusiones"].append((len(e.confusiones), img))

    resumen = {}
    for carpeta, lista in grupos.items():
        destino = salida / carpeta
        shutil.rmtree(destino, ignore_errors=True)
        lista.sort(key=lambda t: (-t[0], t[1].nombre))
        resumen[carpeta] = {"imagenes_afectadas": len(lista),
                            "ejemplos": [img.nombre for _, img in lista[:n]]}
        for _, img in lista[:n]:
            dibujar(dir_imagenes / img.nombre, img, umbral, destino / img.nombre)
    return resumen


def tabla_matriz(matriz: dict) -> str:
    etiquetas = list(clases.NOMBRES) + [FONDO]
    filas = ["| predicho \\ real | " + " | ".join(nombre_etiqueta(e) for e in etiquetas) + " |",
             "|---" * (len(etiquetas) + 1) + "|"]
    for p in etiquetas:
        filas.append(f"| **{nombre_etiqueta(p)}** | " +
                     " | ".join(str(matriz[p][r]) for r in etiquetas) + " |")
    return "\n".join(filas)


def escribir_reporte(salida: Path, res: dict) -> None:
    u = res["ultralytics"]
    lineas = [
        f"# Evaluación del detector — split `{res['split']}`",
        "",
        f"- Pesos: `{res['pesos']}`",
        f"- Imágenes: {res['imagenes']}",
        f"- Fecha: {res['fecha']}",
        "",
        "## Métricas de Ultralytics",
        "",
        "| Clase | Precisión | Recall | mAP50 | mAP50-95 |",
        "|---|---|---|---|---|",
        f"| **todas** | {u['global']['precision']} | {u['global']['recall']} | "
        f"{u['global']['mAP50']} | {u['global']['mAP50_95']} |",
    ]
    for nombre, m in u["por_clase"].items():
        lineas.append(f"| {nombre} | {m['precision']} | {m['recall']} | {m['mAP50']} | {m['mAP50_95']} |")

    e = res["umbral_elegido"]
    lineas += [
        "",
        f"## Umbral elegido: {e['umbral']} (F1 macro {e['f1_macro']})",
        "",
        "| Clase | TP | FP | FN | Precisión | Recall | F1 |",
        "|---|---|---|---|---|---|---|",
    ]
    for nombre, m in e["por_clase"].items():
        lineas.append(f"| {nombre} | {m['tp']} | {m['fp']} | {m['fn']} | "
                      f"{m['precision']} | {m['recall']} | {m['f1']} |")
    lineas += ["", "### Matriz de confusión por caja (IoU ≥ 0.5)", "", tabla_matriz(e["matriz"]),
               "", "### Sugerencia de categoría por foto", "",
               f"Exactitud: **{res['por_imagen']['exactitud']}** "
               f"sobre {res['por_imagen']['imagenes']} fotos.", "",
               tabla_matriz(res["por_imagen"]["matriz"]), "",
               "## Barrido de umbrales", "", "| Umbral | F1 macro |", "|---|---|"]
    for b in res["barrido"]:
        lineas.append(f"| {b['umbral']} | {b['f1_macro']} |")
    lineas += ["", "## Errores para inspeccionar", ""]
    for tipo, d in res["ejemplos"].items():
        lineas.append(f"- **{tipo.replace('_', ' ')}**: {d['imagenes_afectadas']} imágenes "
                      f"(ejemplos en `{tipo}/`)")
    (salida / "reporte.md").write_text("\n".join(lineas) + "\n", encoding="utf-8")


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--pesos", type=Path, required=True)
    p.add_argument("--data", type=Path, default=RAIZ_SERVICIO / "datasets" / "urbia" / "data.yaml")
    p.add_argument("--split", default="test", choices=["val", "test"])
    p.add_argument("--imgsz", type=int, default=640)
    p.add_argument("--batch", type=int, default=8, help="imágenes por lote al evaluar (baja si falta VRAM)")
    p.add_argument("--umbral", type=float, help="usar este umbral en vez de elegirlo")
    p.add_argument("--ejemplos", type=int, default=20, help="imágenes a guardar por tipo de error")
    p.add_argument("--salida", type=Path)
    args = p.parse_args()

    from ultralytics import YOLO

    pesos = args.pesos.resolve()
    salida = args.salida or pesos.parent.parent / f"evaluacion_{args.split}"
    salida.mkdir(parents=True, exist_ok=True)

    modelo = YOLO(str(pesos))
    nombres_modelo = {int(k): v for k, v in modelo.names.items()}
    if nombres_modelo != clases.NOMBRES:
        raise SystemExit(f"ERROR: el modelo tiene clases {nombres_modelo}, se esperaba {clases.NOMBRES}")

    with args.data.open(encoding="utf-8") as f:
        raiz_data = Path(yaml.safe_load(f)["path"])
    dir_img = raiz_data / "images" / args.split
    dir_lbl = raiz_data / "labels" / args.split

    print("1/3 Métricas de Ultralytics...")
    ultra = metricas_ultralytics(modelo, str(args.data.resolve()), args.split, args.imgsz,
                                 args.batch, salida)

    print("2/3 Prediciendo y barriendo umbrales...")
    imagenes = predecir(modelo, dir_img, dir_lbl, args.imgsz, args.batch)
    ids = list(clases.NOMBRES)
    barrido = barrer_umbrales(imagenes, ids)
    elegido = (next(r for r in barrer_umbrales(imagenes, ids, [args.umbral]))
               if args.umbral else elegir_umbral(barrido))

    print("3/3 Guardando ejemplos de errores...")
    ejemplos = guardar_ejemplos(imagenes, dir_img, elegido.umbral, salida, args.ejemplos)

    res = {
        "pesos": str(pesos),
        "split": args.split,
        "imagenes": len(imagenes),
        "fecha": datetime.now().isoformat(timespec="seconds"),
        "ultralytics": ultra,
        "umbral_elegido": {
            "umbral": elegido.umbral,
            "criterio": "fijado a mano" if args.umbral else "máximo F1 macro (IoU 0.5)",
            "f1_macro": round(elegido.f1_macro, 4),
            "por_clase": {clases.NOMBRES[c]: m.como_dict() for c, m in elegido.por_clase.items()},
            "matriz": elegido.matriz,
        },
        "por_imagen": evaluar_por_imagen(imagenes, elegido.umbral, clases.NOMBRES),
        "barrido": [{"umbral": r.umbral, "f1_macro": round(r.f1_macro, 4)} for r in barrido],
        "ejemplos": ejemplos,
    }
    (salida / "metricas.json").write_text(json.dumps(res, indent=2, ensure_ascii=False), encoding="utf-8")
    escribir_reporte(salida, res)

    print(f"\nmAP50 global: {ultra['global']['mAP50']}  |  mAP50-95: {ultra['global']['mAP50_95']}")
    print(f"Umbral elegido: {elegido.umbral}  (F1 macro {elegido.f1_macro:.4f})")
    for nombre, m in res["umbral_elegido"]["por_clase"].items():
        print(f"  {nombre:7} P={m['precision']:.3f}  R={m['recall']:.3f}  F1={m['f1']:.3f}")
    print(f"Exactitud de la sugerencia por foto: {res['por_imagen']['exactitud']}")
    print(f"\nReporte: {salida / 'reporte.md'}")
    print("Siguiente paso:")
    print(f"  python -m scripts.publicar_modelo --pesos {pesos} "
          f"--metricas {salida / 'metricas.json'} --version 1.0.0")


if __name__ == "__main__":
    main()