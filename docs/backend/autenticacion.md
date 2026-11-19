# Autenticación y autorización

## Alta de usuarios
- `POST /api/v1/auth/registro` es público y **siempre** crea usuarios con rol `ciudadano`.
  El rol nunca se acepta desde el cliente.
- Los administradores se promueven manualmente (DataGrip, como `urbia_owner`):

  ```sql
  UPDATE urbia.usuario
  SET rol = 'administrador', fecha_actualizacion = now()
  WHERE lower(correo) = lower('correo@ejemplo.mx');
  ```
  El usuario debe volver a iniciar sesión para que su token lleve el nuevo rol.

## Contraseñas
bcrypt (costo 12), de 8 a 72 bytes. Solo se guarda `hash_contrasena`.

## Tokens
JWT HS256 con `sub` (id_usuario), `rol`, `iss=urbia-api`, `iat` y `exp`.
Duración: `JWT_TTL` (por defecto 2h). Secreto: `JWT_SECRET` (mínimo 32 caracteres).
Enviar como `Authorization: Bearer <token>`.

## Endpoints
| Método | Ruta | Acceso |
|---|---|---|
| POST | /api/v1/auth/registro | Público |
| POST | /api/v1/auth/login | Público |
| GET | /api/v1/auth/perfil | Autenticado |
| GET | /api/v1/admin/usuarios | Administrador |

## Respuestas de error
- 401 `NO_AUTORIZADO`: sin token, token inválido/expirado o credenciales incorrectas.
  El login responde igual si el correo no existe o la contraseña es incorrecta.
- 403 `PROHIBIDO`: autenticado pero sin el rol requerido.
- El cambio de estado de reportes usará `RequirePermiso(dominio.Rol.PuedeCambiarEstado)`.