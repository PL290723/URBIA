# Pruebas unitarias del backend Go (HU 09.2)

Comprueban reglas de negocio, validaciones y respuestas de la API **sin PostgreSQL,
sin Python y sin red**: corren en segundos con `go test`. Las pruebas que sí usan
servicios reales están aparte, en `internal/integracion` (HU 08.2, etiqueta `integracion`).

## Cómo correrlas

```powershell
cd backend-go
go test ./...                       # todas las unitarias
go test ./... -cover                # con porcentaje de cobertura por paquete
go test ./internal/handlers/ -v     # un paquete, viendo cada caso
go test ./... -run Estado -v        # solo las que tengan "Estado" en el nombre
```

En GoLand: clic derecho sobre `backend-go` → **Run 'go test ./...'**, o el ícono ▶ junto a cada `func Test…`.

Para ver qué líneas quedan sin probar:

```powershell
go test ./... -coverprofile=cobertura.out
go tool cover -html=cobertura.out
```

(`cobertura.out` y `*.out` ya están en `.gitignore`).

## Qué se prueba

| Subtarea | Archivos | Qué demuestra |
|---|---|---|
| Validación de reportes | `dominio/reportes_test.go`, `handlers/reporte_handler_test.go`, `handlers/params_test.go` | Descripción, coordenadas, (0,0) sin señal, origen gps/manual, precisión, categoría; todos los errores de campo juntos; multipart obligatorio; 413 antes de leer > 10 MB; ids y nombres de archivo maliciosos (`../`, NTFS `:`, byte nulo) |
| Transiciones de estado | `dominio/cambio_estado_test.go`, `dominio/dominio_test.go`, `services/estado_service_test.go`, `services/estado_cambio_test.go`, `handlers/estado_handler_test.go` | Ciclo pendiente → en_revision → en_proceso → resuelto; saltos, mismo estado y estado final → 409 con estado actual y permitidos; el rechazo del trigger → 409; nada cambia si se rechaza |
| Prioridad y datos faltantes | `dominio/prioridad_test.go`, `services/prioridad_service_test.go` | Fórmula, pesos que suman 1, no inventa factores faltantes, reparte pesos, cobertura mínima para asignar nivel |
| Autenticación y permisos | `auth/auth_test.go`, `middleware/auth_test.go`, `services/auth_service_test.go`, `handlers/auth_handler_test.go`, `middleware/seguridad_test.go` | bcrypt, JWT (expirado, alterado, `alg:none`, otro secreto); 401/403 por rol; registro siempre como ciudadano; correo normalizado; mismo 401 para correo inexistente y contraseña incorrecta; límite de intentos por IP; CORS; encabezados; `X-Request-ID`; logs sin coordenadas ni tokens |
| Errores de repositorio y respuestas HTTP | `repositories/errores_test.go`, `response/response_test.go`, `handlers/*_test.go` | `pgx.ErrNoRows` → no encontrado; FK/unique de PostgreSQL → errores de dominio (404/409); una caída de la BD **nunca** se convierte en 401/404 engañoso, llega como 500 genérico sin filtrar host, puerto ni usuario; consultas 100 % parametrizadas; formato `{success, data, error}` |

## Cómo se prueba sin base de datos

`AuthService` y `EstadoService` reciben su repositorio como **interfaz**
(`services.RepositorioUsuarios`, `services.RepositorioEstados`). En producción se les pasa
el repositorio de PostgreSQL; en las pruebas, un doble en memoria que puede simular
correos duplicados, reportes inexistentes o la BD caída (`falla`).

Los handlers se prueban con un router de Gin real, los middlewares reales
(`RequireAuth`, `RequirePermiso`, `LimiteCuerpo`) y `httptest`: es la misma petición
HTTP que haría el frontend, sin abrir un puerto.

## Cobertura (go test ./... -cover)

| Paquete | Antes | Después |
|---|---|---|
| `response` | 0 % | 100 % |
| `middleware` | 17 % | 91 % |
| `handlers` | 0 % | 33 % |
| `services` | 32 % | 40 % |
| `repositories` | 0 % | 9 % (el resto es SQL: lo cubren las pruebas de integración) |
| `dominio` | 98.5 % | 98.5 % |
