# URBIA

**Sistema inteligente de detección y gestión de problemas urbanos**

## Objetivo del proyecto
URBIA proporciona la infraestructura central para recibir, procesar y gestionar reportes ciudadanos sobre incidencias urbanas (como baches y acumulación de basura). Su objetivo principal es orquestar la ingesta de datos geolocalizados, evaluar las evidencias fotográficas mediante un modelo de visión por computadora (IA) y exponer los resultados a través de una API robusta para su posterior visualización y administración.

Este proyecto está diseñado como un Producto Mínimo Viable (MVP) para la Feria de Proyectos Linces 2026.

## Arquitectura
El sistema adopta un enfoque de microservicios orientados al rendimiento y procesamiento de datos espaciales:

1. **API principal (Go + ???):** Encargada de gestionar la concurrencia, autenticación, validación de reportes (CRUD) y el flujo lógico de la aplicación.
2. **Servicio de inferencia (Python + PyTorch/YOLO):** Microservicio dedicado exclusivamente a cargar el modelo de IA en memoria y ejecutar la inferencia sobre las imágenes de los reportes.
3. **Comunicación Interna (gRPC):** La API en Go y el motor de inferencia en Python se comunican mediante gRPC y Protocol Buffers, garantizando una latencia mínima al evitar la sobrecarga del protocolo HTTP tradicional.
4. **Base de datos espacial (PostgreSQL + PostGIS):** Almacenamiento persistente relacional con capacidades nativas para consultas geográficas (ej. encontrar reportes duplicados en un radio específico).
5. **Almacenamiento de objetos (Volúmenes locales):** Persistencia de las fotografías originales y procesadas gestionada directamente en el servidor.
6. **Orquestación (Docker Compose):** Contenedorización de todos los servicios para garantizar un despliegue unificado y reproducible en el equipo de demostración local.

## Requisitos previos (Prerequisites)
Para ejecutar este proyecto en tu entorno de desarrollo, asegúrate de contar con las siguientes herramientas instaladas:

### Obligatorios (Ejecución vía Docker)
* **Docker Engine** (v24.0+)
* **Docker Compose** (v2.20+)
* Tarjeta Gráfica NVIDIA (Opcional, pero recomendada para acelerar la inferencia con CUDA)

### Para desarrollo local (Sin contenedores)
* **Go** (v1.21 o superior)
* **Python** (v3.10 o superior)
* **PostgreSQL** (v15+) con la extensión **PostGIS** habilitada.
* **Make** (para automatizar la generación de código gRPC y migraciones).
* **Protocol Buffers Compiler (`protoc`)** (para generar los contratos gRPC).