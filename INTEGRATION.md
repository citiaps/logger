# Integration Guide

Esta guía describe cómo integrar `github.com/citiaps/logger` en cualquier servicio Go. El objetivo es que cada servicio emita logs JSON por `stdout` con campos consistentes para Docker/Nomad, OpenTelemetry y VictoriaLogs.

## 1. Dependencia

Instalar la versión publicada:

```bash
go get github.com/citiaps/logger@v0.2.0
go mod tidy
```

Durante desarrollo local se puede usar temporalmente:

```go
replace github.com/citiaps/logger => ../logger
```

No dejar `replace` en ramas de release/deploy.

## 2. Variables De Entorno

Cada servicio debería definir al menos:

```bash
SERVICE_NAME=<stable-service-name>
ENV=prod
LOG_LEVEL=info
LOG_FORMAT=json
```

Recomendado:

```bash
SERVICE_VERSION=<tag-or-git-sha>
OTEL_SERVICE_NAME=<stable-service-name>
```

Resolución de nombre de servicio:

```text
SERVICE_NAME -> OTEL_SERVICE_NAME -> unknown-service
```

Resolución de ambiente:

```text
ENV -> GO_REST_ENV -> APP_ENV -> GIN_MODE
```

Resolución de versión:

```text
SERVICE_VERSION -> VERSION -> GIT_SHA -> COMMIT_SHA
```

Ejemplos para los servicios conversados:

| Proyecto | `SERVICE_NAME` sugerido |
|---|---|
| E-Firma-Backend | `e-firma-backend` |
| E-Firma-Servicios-Backend | `e-firma-servicios-backend` |
| Middleware-Dspace | `dspace-middleware` |
| Middleware-Dspace/auth-service | `auth-service` |
| contraloria-honorarios-backend | `contraloria-honorarios-backend` |

## 3. Inicialización

En `main.go`, inicializar una sola vez al comienzo del proceso:

```go
logger.InitFromEnv()
```

Hacerlo antes de inicializar DB, OpenTelemetry, Gin, workers, cronjobs u otros componentes que puedan loggear.

## 4. Uso Con Gin

No usar `gin.Default()`, porque agrega logger/recovery de Gin en texto plano.

Usar:

```go
app := gin.New()
app.Use(logger.GinRecovery(nil))
app.Use(otelgin.Middleware(serviceName)) // si el servicio usa OpenTelemetry
app.Use(logger.GinLogger(nil))
```

Después agregar los middlewares propios del servicio:

```go
app.Use(middleware.CorsMiddleware())
app.Use(middleware.AuthMiddleware())
```

Orden recomendado:

```text
GinRecovery -> OTel middleware -> GinLogger -> middlewares de app -> routes
```

`GinLogger` emite `event=http.request` con campos HTTP estándar:

```text
method path route status latency_ms client_ip user_agent request_id body_size
```

Cuando el handler conoce el resultado de dominio, debe anotar la request para que el log HTTP final no quede solo como `http.request`:

```go
logger.SetRequestEvent(c, "dspace.document.download.failed")
logger.SetRequestError(c, err,
    logger.ErrorKind("external_api"),
    logger.Operation("download"),
    slog.Int("external_status", 502),
    slog.String("external_item_ref", itemRef),
    slog.String("external_doc_ref", docRef),
)
```

Con eso, el log HTTP final conserva los campos HTTP y queda consultable por dominio:

```json
{
  "event": "dspace.document.download.failed",
  "http_event": "http.request",
  "status": 500,
  "route": "/api/v1/signatures/download/:request_id",
  "error": "bad gateway",
  "error_kind": "external_api",
  "operation": "download",
  "external_status": 502
}
```

Regla: para errores 4xx/5xx conocidos por la aplicación, preferir anotar `SetRequestEvent` con un evento de dominio. `event=http.request` debería quedar para requests sin resultado de dominio específico.

`GinRecovery` emite `event=http.panic` con:

```text
error error_kind=panic stack method path route status request_id
```

## 5. Eliminar Salidas No JSON

Migrar cualquier escritura directa a `stdout`/`stderr` o logger de texto plano.

| Patrón | Reemplazo |
|---|---|
| `fmt.Println(...)` | `logger.Info(ctx, "message", ...)` |
| `fmt.Printf(...)` | `logger.Infof(ctx, ...)` temporalmente |
| `log.Printf(...)` | `logger.Infof(ctx, ...)` temporalmente |
| `log.Fatal(...)` | `logger.Fatal(ctx, "message", ...)` |
| `panic(err)` operacional | `logger.Fatal(ctx, "message", logger.WithError(err))` |
| `gin.Default()` | `gin.New()` + `GinLogger` + `GinRecovery` |
| logs a archivo local | eliminar; usar `stdout` para Docker/Nomad |

Los panics verdaderamente no recuperables pueden existir, pero deberían ser excepcionales. En HTTP, `GinRecovery` los convierte en JSON.

## 6. Logging Estructurado

Preferir logs con campos key/value:

```go
logger.Info(ctx, "resource updated",
    logger.Event("resource.updated"),
    logger.UserID(userID),
    "resource_id", resourceID,
    "old_status", oldStatus,
    "new_status", newStatus,
)
```

Usar `Infof`, `Warnf`, `Errorf` y `Fatalf` solo como compatibilidad temporal al migrar desde `log.Printf`.

## 7. Estándar De Errores

Correcto:

```go
logger.Error(ctx, "failed to create resource",
    logger.Event("resource.create.failed"),
    logger.WithError(err),
    logger.ErrorKind("db"),
    logger.UserID(userID),
)
```

Incorrecto:

```go
logger.Infof(ctx, "Error creando recurso: %v", err)
logger.Error(ctx, "Error: "+err.Error())
logger.Error(ctx, "failed", "err", err)
```

Campos estándar de error:

| Campo | Uso |
|---|---|
| `error` | Error real, usando `logger.WithError(err)` |
| `error_kind` | Categoría general: `db`, `validation`, `external_api`, `auth`, `timeout`, `panic` |
| `error_code` | Código interno/externo si existe |
| `retryable` | `true` si corresponde reintento |

Regla: `msg` es humano y corto; el error va en `error`.

## 8. Eventos

`event` es un nombre estable para buscar, medir y alertar. No depende del idioma ni del texto de `msg`.

Formato recomendado:

```text
<domain>.<action>.<result>
```

Ejemplos genéricos:

```text
auth.login.success
auth.login.failed
resource.created
resource.updated
resource.deleted
resource.create.failed
external_api.request.failed
job.started
job.completed
job.failed
workflow.transition.success
workflow.transition.failed
```

Ejemplos específicos para los servicios conversados:

```text
dspace.document.upload.failed
dspace.document.download.failed
signature.request.created
signature.batch.completed
contract.status.changed
contract.status.change.failed
```

No todos los logs necesitan `event`. Usarlo cuando el log represente algo que se quiera buscar, medir o alertar.

## 9. Campos De Dominio

Cada servicio puede agregar campos propios, pero debe mantener nombres estables y consistentes.

Campos comunes recomendados:

```text
user_id
role
system_id
application_id
request_id
correlation_id
operation
external_status
old_status
new_status
```

Ejemplos específicos para los servicios conversados:

```text
signature_request_id
batch_request_id
document_id
document_uuid
signature_type
external_item_ref
external_doc_ref
dspace_uuid
collection_uuid
workspace_item_id
bitstream_uuid
contract_id
num_convenio
num_presupuesto
workflow_id
cost_center_id
```

Evitar variantes para el mismo concepto:

```text
userId / userid / id_user
err / error_msg / error_details
route_path / endpoint / url_path
```

## 10. Compatibilidad Temporal

Si un servicio tiene muchas llamadas antiguas tipo `utils.LogError(...)`, se permite una capa temporal `utils/logger_compat.go`.

Debe cumplir:

- `LogError` emite nivel `ERROR`.
- `LogWarn` emite nivel `WARN`.
- Si recibe un `error`, debería pasarlo como campo `error` cuando sea posible.
- No debe escribir a archivos.
- No debe usar colores ANSI en producción.

Ejemplo mínimo:

```go
func LogError(format string, args ...any) {
    logger.Errorf(context.TODO(), format, args...)
}
```

Objetivo final: reemplazar compat por llamadas estructuradas con `logger.Event`, `logger.WithError`, `logger.ErrorKind`, etc.

## 11. Checklist De Aceptación

Antes de mergear en cada proyecto:

```bash
go build ./...
```

Buscar que no queden fuentes comunes de logs no JSON:

```bash
rg 'gin\.Default\('
rg 'fmt\.Print|fmt\.Printf|fmt\.Println'
rg 'panic\('
rg 'log\.Print|log\.Fatal|log\.Panic'
rg 'lumberjack'
```

Validar en ejecución que cada línea de stdout sea JSON válido y contenga al menos:

```text
time level service msg
```

Si se define ambiente, también debe aparecer:

```text
env
```

Para requests HTTP debe contener:

```text
event=http.request method route status latency_ms request_id
```
