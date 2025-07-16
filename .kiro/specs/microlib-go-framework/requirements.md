# Requirements Document

## Introduction

MicroLib es una librería de microservicios para Go diseñada para estandarizar y simplificar el desarrollo de microservicios productivos. El objetivo principal es proporcionar un framework cohesivo y unificado que reduzca el time-to-market y asegure la implementación de las mejores prácticas de arquitectura desde el inicio. La librería debe ser opinada pero flexible, ofreciendo implementaciones por defecto robustas mientras permite extensibilidad y personalización.

## Requirements

### Requirement 1

**User Story:** Como desarrollador de microservicios, quiero un núcleo de servicio robusto que maneje el ciclo de vida completo, para que pueda enfocarme en la lógica de negocio sin preocuparme por la infraestructura básica.

#### Acceptance Criteria

1. WHEN se crea un nuevo servicio THEN el sistema SHALL proporcionar un constructor `NewService(metadata ServiceMetadata)`
2. WHEN se inicia el servicio THEN el sistema SHALL verificar que las dependencias críticas estén saludables antes de aceptar tráfico
3. WHEN se recibe una señal del sistema operativo THEN el servicio SHALL ejecutar un apagado ordenado con timeout configurable
4. WHEN se registra el servicio THEN el sistema SHALL gestionar metadatos como nombre, versión semántica, ID de instancia y checksum de build
5. IF se configura autoregistro THEN el servicio SHALL registrarse automáticamente con service registries como Consul

### Requirement 2

**User Story:** Como desarrollador, quiero un sistema de configuración centralizada y validada, para que pueda gestionar configuraciones de manera consistente y segura.

#### Acceptance Criteria

1. WHEN se carga la configuración THEN el sistema SHALL seguir la jerarquía: variables de entorno > archivo (YAML/TOML) > flags de línea de comando
2. WHEN se inicia el servicio THEN el sistema SHALL validar la configuración usando esquemas definidos
3. WHEN se accede a la configuración THEN el sistema SHALL proporcionar un objeto inyectado thread-safe
4. IF la configuración es inválida THEN el servicio SHALL fallar al arrancar con mensajes de error claros

### Requirement 3

**User Story:** Como desarrollador, quiero logging estructurado integrado con tracing, para que pueda tener visibilidad completa del comportamiento del servicio.

#### Acceptance Criteria

1. WHEN se genera un log THEN el sistema SHALL usar formato JSON por defecto con slog
2. WHEN existe un context.Context THEN el logger SHALL enriquecer automáticamente los logs con trace IDs y metadatos
3. WHEN se configura el nivel de log THEN el sistema SHALL respetar la configuración en tiempo de ejecución
4. WHEN se propaga el contexto THEN los logs SHALL mantener la correlación entre requests

### Requirement 4

**User Story:** Como desarrollador, quiero observabilidad completa integrada, para que pueda monitorear y diagnosticar mi servicio en producción.

#### Acceptance Criteria

1. WHEN se expone métricas THEN el sistema SHALL proporcionar un endpoint `/metrics` compatible con Prometheus
2. WHEN se procesan requests THEN el sistema SHALL generar automáticamente métricas de latencia, errores y throughput
3. WHEN se implementa tracing THEN el sistema SHALL usar OpenTelemetry por defecto con propagación automática
4. WHEN se consulta salud THEN el sistema SHALL exponer endpoints `/health` con sondas liveness, readiness y startup
5. IF se añaden dependencias externas THEN el sistema SHALL permitir extensión fácil de health checks

### Requirement 5

**User Story:** Como desarrollador de APIs, quiero comunicación sincrónica robusta con especificaciones estándar, para que pueda crear APIs bien documentadas y mantenibles.

#### Acceptance Criteria

1. WHEN se crea un servidor HTTP THEN el sistema SHALL integrar automáticamente middlewares para logging, métricas, tracing y autenticación
2. WHEN se define una API con TypeSpec THEN el sistema SHALL servir la especificación OpenAPI en `/openapi.json`
3. WHEN se usa gRPC THEN el sistema SHALL proporcionar interceptores equivalentes a los middlewares HTTP
4. WHEN ocurre un panic THEN el middleware SHALL recuperarse y generar logs apropiados

### Requirement 6

**User Story:** Como desarrollador de sistemas distribuidos, quiero comunicación asincrónica confiable con patrón Outbox, para que pueda garantizar consistencia eventual sin complejidad adicional.

#### Acceptance Criteria

1. WHEN se publica un mensaje THEN el sistema SHALL proporcionar una abstracción `messaging.Broker` con métodos `Publish` y `Subscribe`
2. WHEN se usa con base de datos relacional THEN el sistema SHALL implementar el patrón Outbox de forma transparente
3. WHEN se publica dentro de una transacción THEN el mensaje SHALL guardarse atómicamente en la tabla outbox
4. WHEN se procesa la tabla outbox THEN un proceso relay SHALL enviar mensajes de forma fiable con reintentos
5. IF se configura RabbitMQ o Kafka THEN el sistema SHALL proporcionar implementaciones por defecto
6. WHEN se documenta mensajería THEN el sistema SHALL integrar con AsyncAPI para especificaciones

### Requirement 7

**User Story:** Como desarrollador, quiero persistencia de datos simplificada con transacciones y migraciones, para que pueda gestionar datos de manera eficiente y segura.

#### Acceptance Criteria

1. WHEN se accede a datos relacionales THEN el sistema SHALL proporcionar interfaz `sql.Database` con pool de conexiones optimizado
2. WHEN se usa PostgreSQL THEN el sistema SHALL usar pgx como implementación por defecto
3. WHEN se gestionan migraciones THEN el sistema SHALL incluir módulo integrado para migraciones y seeding
4. WHEN se usa caché THEN el sistema SHALL proporcionar interfaz `cache.Store` con operaciones estándar
5. IF se configura Redis THEN el sistema SHALL proporcionar implementación por defecto para caché

### Requirement 8

**User Story:** Como desarrollador, quiero tareas programadas y cómputo distribuido, para que pueda ejecutar trabajos en segundo plano de manera confiable.

#### Acceptance Criteria

1. WHEN se registra una tarea programada THEN el sistema SHALL proporcionar scheduler con soporte para cron jobs
2. WHEN múltiples instancias ejecutan tareas THEN el sistema SHALL usar elección de líder para evitar duplicación
3. WHEN se define un job distribuido THEN el sistema SHALL proporcionar framework con reintentos y back-off exponencial
4. IF se usa Redis o PostgreSQL THEN el sistema SHALL proporcionar implementaciones por defecto para job queue

### Requirement 9

**User Story:** Como desarrollador consciente de la seguridad, quiero autenticación y autorización integradas, para que pueda proteger mis servicios sin implementación compleja.

#### Acceptance Criteria

1. WHEN se valida autenticación THEN el sistema SHALL proporcionar middleware para tokens JWT compatible con OIDC
2. WHEN se configura JWKS THEN el sistema SHALL validar tokens usando el endpoint configurado
3. WHEN se extrae información del token THEN las claims SHALL inyectarse en el context.Context
4. WHEN se integra autorización THEN el sistema SHALL proporcionar hooks para sistemas como OPA
5. IF se gestionan secretos THEN el sistema SHALL proporcionar interfaces para Vault o KMS

### Requirement 10

**User Story:** Como desarrollador, quiero herramientas de desarrollo y scaffolding, para que pueda crear servicios rápidamente siguiendo las mejores prácticas.

#### Acceptance Criteria

1. WHEN se crea un nuevo servicio THEN el CLI SHALL generar estructura completa usando `microlib-cli new service`
2. WHEN se necesitan ejemplos THEN la documentación SHALL incluir boilerplates para casos de uso comunes
3. WHEN se usa la librería THEN el sistema SHALL requerir Go 1.22 o superior
4. WHEN se desarrolla THEN el sistema SHALL seguir convenciones idiomáticas de Go con interfaces y manejo explícito de errores
5. IF se extiende funcionalidad THEN el sistema SHALL permitir inyección de dependencias con herramientas como wire o fx