# Configuración del Backend - Documentación

## ¿Qué se cambió?

### 1. **Instalación de godotenv**
Se agregó la dependencia `github.com/joho/godotenv` para cargar variables de entorno desde archivos `.env.local`.

```bash
go get github.com/joho/godotenv
```

### 2. **Actualización de config.go**
Se modificó `internal/config/config.go` para:
- Importar el paquete `godotenv`
- Cargar automáticamente el archivo `.env.local` al iniciar la aplicación
- Mantener compatibilidad con variables de entorno del sistema

**Cambios:**
```go
// Antes
import (
    "fmt"
    "os"
    "strconv"
    "time"
)

// Después
import (
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "time"
    
    "github.com/joho/godotenv"
)
```

En la función `Load()`:
```go
func Load() (*Config, error) {
    // Load from .env.local if it exists
    envPath := filepath.Join(".", ".env.local")
    if _, err := os.Stat(envPath); err == nil {
        // File exists, try to load it
        if err := godotenv.Load(envPath); err != nil {
            fmt.Printf("WARNING: Could not load .env.local: %v\n", err)
        }
    }
    // ... rest of the function
}
```

### 3. **Actualización del archivo .env.local**
Se completó con todas las variables requeridas:

```env
APP_ENV=development
SERVER_PORT=8080

# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=axisflowdb
DB_USER=usraxis
DB_PASSWORD=Admin01

# JWT
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h  # 7 days

# Redis
REDIS_URL=redis://localhost:6379

# Features & Observability
FEATURE_DELETE_ALL_DATA=false
LOG_LEVEL=debug
METRICS_ENABLED=true
```

### 4. **Creación de .env.example**
Archivo de referencia con todas las variables y sus explicaciones.

## Cómo Funciona Ahora

### Orden de Carga de Variables

1. **Al iniciar el servidor**, se intenta cargar `./``.env.local`
2. Si el archivo existe, se cargan las variables en `os.Environ()`
3. Las variables del sistema tienen prioridad sobre las del archivo
4. Si alguna variable requerida falta, se muestra un error claro

### Ejemplos de Uso

**Iniciar con variables locales:**
```bash
cd axis-flow-back
./bin/server
# Lee automáticamente .env.local
```

**Sobrescribir una variable:**
```bash
REDIS_URL=redis://192.168.1.100:6379 ./bin/server
# Usa Redis remoto, ignora el valor de .env.local
```

**Usando el script dev:**
```bash
cd ..
./dev.sh back
# Lee .env.local automáticamente
```

## Formatos Especiales

### Duraciones (JWT TTL)
Go requiere el formato ISO 8601:
- `15m` = 15 minutos
- `1h` = 1 hora
- `168h` = 7 días (NO usar `7d`)
- `720h` = 30 días

### Redis URL
- Sin autenticación: `redis://localhost:6379`
- Con contraseña: `redis://:password@localhost:6379`
- En cluster: `redis://node1:6379,node2:6379`

## Validación

El backend valida todas las variables requeridas al iniciar:

✅ Variables validadas:
- `APP_ENV` - requerida
- `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` - requeridas
- `JWT_SECRET`, `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL` - requeridas
- `REDIS_URL` - requerida

✓ Variables opcionales (con defaults):
- `SERVER_PORT` (default: 8080)
- `DB_MAX_OPEN_CONNS` (default: 10)
- `DB_MAX_IDLE_CONNS` (default: 5)
- `LOG_LEVEL` (default: info)
- `METRICS_ENABLED` (default: true)

## Troubleshooting

### Erro: "JWT_REFRESH_TTL must be a valid duration"
**Problema:** Formato incorrecto como `7d`  
**Solución:** Cambiar a `168h`

### Error: "configuration errors: [... is required]"
**Problema:** Variable faltante en `.env.local`  
**Solución:** Revisar que todas las variables requeridas existan en `.env.local`

### Error: "redis: connection refused"
**Problema:** Redis no está corriendo  
**Solución:** Iniciar Redis: `docker run --name redis -p 6379:6379 -d redis`

### El archivo .env.local no se carga
**Problema:** El archivo está fuera de la ruta esperada  
**Solución:** 
1. Asegurarse que `.env.local` esté en `axis-flow-back/`
2. Ejecutar desde la raíz: `./axis-flow-back/bin/server`

## Archivos Modificados

- `internal/config/config.go` - Agregada carga de .env.local
- `axis-flow-back/.env.local` - Completadas todas las variables
- `go.mod` - Agregada dependencia `github.com/joho/godotenv`
- `go.sum` - Actualizado con checksums
- `axis-flow-back/.env.example` - Creado con documentación

## Integración con dev.sh

El script `dev.sh` ahora funciona correctamente:

```bash
./dev.sh install     # Instala dependencias
./dev.sh build-back  # Compila con godotenv support
./dev.sh back        # Ejecuta backend leyendo .env.local
./dev.sh dev         # Backend + Frontend (ambos leen .env.local)
```
