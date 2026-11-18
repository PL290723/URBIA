# Ejecución local y en la feria (HU 10.4)

Objetivo: que celulares y otros equipos de la **misma red local** usen el backend que corre
en el equipo servidor (la laptop con Docker Desktop). Reglas de seguridad de la red:
[docs/backend/seguridad.md → Red de la feria](../docs/backend/seguridad.md#red-de-la-feria).

## 1. Antes de la feria (en casa, con internet)

Las imágenes llevan todo lo necesario (incluidos los pesos del modelo); en la feria no hace falta internet.

```powershell
cd deployments
docker compose build
docker compose up -d
docker compose ps
```

Energía: conectar el cargador y evitar que el equipo se suspenda mientras está enchufado:

```powershell
powercfg /change standby-timeout-ac 0
```

## 2. Red

- Usar un router o hotspot **propio del equipo** con WPA2/WPA3. No usar la Wi-Fi abierta del evento.
- Algunos hotspots de celular aíslan a los clientes (los dispositivos no se ven entre sí).
  Si el paso 6 falla con todo lo demás bien, probar con otro hotspot o con un router.
- Conectar el equipo servidor y marcar la red como **Privada** (PowerShell como administrador):

```powershell
Get-NetConnectionProfile
Set-NetConnectionProfile -InterfaceAlias "Wi-Fi" -NetworkCategory Private
```

## 3. IP local del equipo servidor

```powershell
Get-NetIPConfiguration |
  Where-Object { $_.IPv4DefaultGateway -ne $null -and $_.NetAdapter.Status -eq 'Up' } |
  Select-Object InterfaceAlias, @{ n = 'IPv4'; e = { $_.IPv4Address.IPAddress } }
```

Usar la IPv4 del adaptador **Wi-Fi** (por ejemplo `192.168.1.50`). No usar las de
`vEthernet (WSL...)` ni `172.x` de Docker: esas no son de la red local.
La IP la asigna el router y puede cambiar de un día a otro: revisarla cada vez.

## 4. Firewall de Windows (una vez, como administrador)

Permitir el puerto de la API **solo en redes privadas**:

```powershell
New-NetFirewallRule -DisplayName "URBIA API (feria)" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -Profile Private
Get-NetFirewallRule -DisplayName "URBIA API (feria)" | Format-List DisplayName, Enabled, Profile, Action
```

PostgreSQL del contenedor solo escucha en `127.0.0.1:5433` y la inferencia no publica
puertos: ninguno de los dos es accesible desde la red.

## 5. Configuración por variables de entorno (`deployments/.env`)

| Variable | Feria | Por qué |
|---|---|---|
| `JWT_SECRET` | uno nuevo | Los tokens de pruebas dejan de valer |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173,http://<IP>:5173` | El navegador del celular abre el frontend por la IP |
| `LOGIN_MAX_INTENTOS` | `50` | Límite compartido por todo el público (IP del proxy de Docker) |
| `LOGIN_VENTANA` | `2m` | Si se llega al límite, el bloqueo dura poco |
| `LOG_LEVEL` | `info` | Sin ruido de depuración |

Aplicar cambios (recrea solo la API):

```powershell
docker compose up -d
docker compose logs api | Select-String "cors_origenes"
```

## 6. Conectar celulares

1. Conectar el celular a la **misma red** que el equipo servidor (no a datos móviles).
2. Abrir en el navegador:
   - `http://<IP>:8080/api/v1/health/live` → `{"status":"ok"}`
   - `http://<IP>:8080/api/v1/docs` → documentación de la API
   - Frontend de la demo: `http://<IP>:5173`
3. Es HTTP sin TLS: aceptable solo dentro de la red cerrada del equipo.

## 7. Alternativa CPU / GPU

La configuración de la feria es **CPU** (`compose.yaml`): una foto tarda del orden de 100 ms
y no depende de drivers. GPU opcional:

```powershell
docker compose -f compose.yaml -f compose.gpu.yaml up -d --build
docker compose logs inferencia | Select-String "dispositivo"
```

Si la inferencia no arranca o no dice `dispositivo=0`, volver a CPU sin reconstruir:

```powershell
docker compose up -d
```

## 8. Problemas comunes

| Síntoma | Causa probable | Solución |
|---|---|---|
| El celular no abre `http://<IP>:8080` y la laptop sí | Red en perfil Público o falta la regla | Pasos 2 y 4 |
| Funciona `http://localhost:8080` pero no `http://<IP>:8080` en la laptop | IP equivocada (WSL/Docker) | Paso 3 |
| El navegador muestra error de CORS | Falta `http://<IP>:5173` en `CORS_ALLOWED_ORIGINS` | Paso 5 |
| Todos reciben 429 al iniciar sesión | Límite de login compartido | Subir `LOGIN_MAX_INTENTOS` o esperar `LOGIN_VENTANA` |
| `/api/v1/health` dice `degradado` | La inferencia sigue cargando o se cayó | `docker compose ps` y `docker compose logs inferencia` |
| Nada responde tras reiniciar Windows | Docker Desktop no abrió | Abrir Docker Desktop; los contenedores vuelven solos (`restart: unless-stopped`) |

## 9. Al terminar

```powershell
docker compose stop
Remove-NetFirewallRule -DisplayName "URBIA API (feria)"
powercfg /change standby-timeout-ac 30
```

Cambiar la contraseña del hotspot. `docker compose stop` conserva base y fotos.
