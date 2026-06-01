# axis-flow

Sistema de identidad y gestión de usuarios para axis-flow.

## Estructura del Proyecto

```
axis-flow/
├── axis-flow-back/    # Backend - Go + PostgreSQL + Redis
├── axis-flow-front/   # Frontend - React + TypeScript + Vite
├── docs/              # Documentación
├── tests/             # Tests
├── dev.sh             # Script shell para desarrollo
└── Makefile           # Makefile para desarrollo
```

## Requisitos Previos

- **Go** 1.25.0 o superior
- **Node.js** 18+ y **npm** 9+
- **PostgreSQL** 12+
- **Redis** 6+

## Inicio Rápido

### Opción 1: Usando el Script Shell

```bash
# Ver ayuda
./dev.sh help

# Instalar dependencias
./dev.sh install

# Compilar ambos proyectos
./dev.sh build

# Iniciar ambos servicios
./dev.sh dev

# Iniciar solo el backend
./dev.sh back

# Iniciar solo el frontend
./dev.sh front
```

### Opción 2: Usando Makefile

```bash
# Ver ayuda
make help

# Instalar dependencias
make install

# Compilar ambos proyectos
make build

# Iniciar ambos servicios
make dev

# Iniciar solo el backend
make back

# Iniciar solo el frontend
make front
```

## Configuración

### Variables de Entorno

Copia `.env.example` a `.env` y ajusta según necesites:

```bash
cp .env.example .env
```

Variables principales:

- `BACK_PORT` - Puerto del backend (default: 8080)
- `FRONT_PORT` - Puerto del frontend (default: 5173)
- `DB_HOST` - Host de PostgreSQL (default: localhost)
- `DB_PORT` - Puerto de PostgreSQL (default: 5432)
- `REDIS_HOST` - Host de Redis (default: localhost)
- `REDIS_PORT` - Puerto de Redis (default: 6379)

## Servicios

### Backend

- **Ubicación**: `axis-flow-back/`
- **Lenguaje**: Go 1.25.0
- **Puerto**: 8080 (configurable)
- **Base de Datos**: PostgreSQL
- **Cache**: Redis
- **API**: REST + gRPC

Endpoints principales:
- `POST /auth/login` - Autenticación
- `POST /auth/logout` - Logout
- `POST /users` - Crear usuario
- `GET /users/:id` - Obtener usuario
- `GET /users` - Listar usuarios (admin)
- `PUT /users/:id` - Actualizar usuario
- `DELETE /users/:id` - Eliminar usuario

Healthcheck:
- `GET /health` - Estado del servicio
- `GET /metrics` - Métricas de Prometheus

### Frontend

- **Ubicación**: `axis-flow-front/`
- **Lenguaje**: TypeScript + React 19
- **Build Tool**: Vite 8
- **Puerto**: 5173 (configurable)
- **UI Framework**: Tailwind CSS v4

Scripts:
```bash
npm run dev      # Servidor de desarrollo
npm run build    # Build para producción
npm run lint     # Validar código
npm run preview  # Preview del build
```

## Compilación

### Compilar ambos

```bash
./dev.sh build
# o
make build
```

### Compilar solo backend

```bash
./dev.sh build-back
# o
make build-back
```

El binario se genera en `axis-flow-back/bin/server`.

### Compilar solo frontend

```bash
./dev.sh build-front
# o
make build-front
```

El build se genera en `axis-flow-front/dist/`.

## Ejecución

### Ambos servicios en paralelo

```bash
./dev.sh dev
# o
make dev
```

Esto inicia:
- Backend en `http://localhost:8080`
- Frontend en `http://localhost:5173`

Para parar: `Ctrl+C`

### Portos personalizados

```bash
BACK_PORT=3000 FRONT_PORT=5000 ./dev.sh dev
# o
make dev BACK_PORT=3000 FRONT_PORT=5000
```

## Desarrollo

### Instalación de dependencias

```bash
./dev.sh install
# o
make install
```

### Linting

```bash
make lint
```

### Tests

```bash
make test
```

## Limpieza

```bash
./dev.sh clean
# o
make clean
```

Limpia:
- Binarios compilados
- Directorios `dist/` y `node_modules/`
- Cache de Go y npm

## Estructura de Directorios

### Backend

```
axis-flow-back/
├── cmd/
│   └── server/          # Entrada principal
├── internal/
│   ├── config/          # Configuración
│   ├── domain/          # Entidades de negocio
│   ├── handler/         # HTTP handlers
│   ├── middleware/      # Middlewares (JWT, RBAC, etc.)
│   ├── repository/      # Data access
│   ├── service/         # Lógica de negocio
│   └── telemetry/       # OpenTelemetry
├── db/
│   └── migrations/      # Migraciones SQL
└── tests/
    └── unit/            # Tests unitarios
```

### Frontend

```
axis-flow-front/
├── src/
│   ├── api/             # Clientes API
│   ├── components/      # Componentes React
│   ├── features/        # Features
│   ├── pages/           # Páginas
│   ├── store/           # Estado global (Zustand)
│   ├── types/           # Tipos TypeScript
│   └── test/            # Setup de tests
├── dist/                # Build output
└── public/              # Assets estáticos
```

## Documentación

Ver documentación completa en `docs/services/01_Users_Service_Spec/spec.md`.

## Solución de Problemas

### Backend no inicia

1. Verifica que PostgreSQL esté corriendo: `psql -U postgres`
2. Verifica que Redis esté corriendo: `redis-cli ping`
3. Revisa los logs: `tail -f /tmp/back.log`

### Frontend no compila

1. Limpia dependencies: `rm -rf node_modules && npm install`
2. Limpia cache: `npm cache clean --force`
3. Verifica TypeScript: `npx tsc --noEmit`

### Puertos ya en uso

```bash
# Usa puertos diferentes
make dev BACK_PORT=3000 FRONT_PORT=5000

# O mata procesos existentes
lsof -ti:8080 | xargs kill -9
```

## Contribución

Consulta la documentación de arquitectura en `docs/` antes de comenzar.

## Licencia

MIT
