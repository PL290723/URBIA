import torch
from ultralytics import YOLO
import cv2
import numpy as np


def verificar_entorno():
    """Documenta y verifica si el entorno utiliza CPU o CUDA."""
    if torch.cuda.is_available():
        print(f"Entorno listo utilizando GPU: {torch.cuda.get_device_name(0)}")
    else:
        print("Advertencia: CUDA no disponible. Ejecutando en CPU.")


def cargar_modelo(ruta_modelo="yolov8n.pt"):
    """Carga el modelo. Si no existe localmente, Ultralytics lo descarga."""
    print("Cargando modelo YOLO...")
    return YOLO(ruta_modelo)


def preprocesar_imagen_prueba():
    """Genera una imagen simulada (ruido) para probar el flujo sin requerir archivos externos."""
    # Crea una imagen negra de 640x640 con un cuadro blanco en el centro
    img = np.zeros((640, 640, 3), dtype=np.uint8)
    cv2.rectangle(img, (200, 200), (440, 440), (255, 255, 255), -1)
    return img


def ejecutar_inferencia(modelo, imagen):
    """Ejecuta la inferencia sobre la imagen."""
    print("Ejecutando inferencia...")
    resultados = modelo(imagen)
    return resultados


if __name__ == "__main__":
    verificar_entorno()

    # 1. Cargar configuración y modelo
    modelo_yolo = cargar_modelo()

    # 2. Preprocesamiento
    imagen_prueba = preprocesar_imagen_prueba()

    # 3. Inferencia
    resultados_inferencia = ejecutar_inferencia(modelo_yolo, imagen_prueba)

    # Mostrar resumen
    print(f"Inferencia completada. Objetos detectados: {len(resultados_inferencia[0].boxes)}")