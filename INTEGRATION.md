# Instrucciones Para Cada Proyecto

Estas son las acciones que debe aplicar cada backend Go para adoptar la base semántica del logger.

## 1. Dependencia

Cuando exista tag publicado:

```bash
go get github.com/citiaps/logger@v0.2.0
```

Durante desarrollo local se puede usar temporalmente:

```go
replace github.com/citiaps/logger => ../logger
```

Regla: no dejar `replace` en ramas de release/deploy.

## 2. Variables de entorno obligatorias

Cada servicio debe definir al menos:

```bash
SERVICE_NAME=<nombre-estable>
ENV=prod
LOG_LEVEL=info
LOG_FORMAT=json
```

Recomendado:

```bash
SERVICE_VERSION=<tag-o-git-sha>
OTEL_SERVICE_NAME=<mismo-service-name>
```

Nombres propuestos:

| Proyecto | `SERVICE_NAME` |
|---|---|
| E-Firma-Backend | `e-firma-backend` |
| E-Firma-Servicios-Backend | `e-firma-servicios-backend` |
| Middleware-Dspace | `dspace-middleware` |
| Middleware-Dspace/auth-service | `auth-service` |
| contraloria-honorarios-backend | `contraloria-honorarios-backend` |

## 3. Inicialización

En `main.go`, llamar una vez al inicio:

```go
logger.InitFromEnv()
```

Idealmente antes de inicializar DB, OTel, Gin u otros componentes que puedan loggear.

## 4. Gin sin logs de texto plano

Reemplazar:

```go
app := gin.Default()
```

por:

```go
app := gin.New()
app.Use(logger.GinLogger(nil))
app.Use(logger.GinRecovery(nil))
```

Luego mantener los middlewares propios:

```go
app.Use(middleware.CorsMiddleware())
app.Use(otelgin.Middleware(config.OpenTelemetryServiceName()))
```

Motivo: `gin.Default()` agrega logger/recovery en texto plano y rompe el contrato JSON por línea.

## 5. Reemplazar salidas no JSON

Buscar y migrar:

| Patrón | Reemplazo |
|---|---|
| `fmt.Println(...)` | `logger.Info(ctx, "message", ...)` |
| `fmt.Printf(...)` | `logger.Infof(ctx, ...)` temporalmente |
| `log.Printf(...)` | `logger.Infof(ctx, ...)` temporalmente |
| `panic(err)` operacional | `logger.Fatal(ctx, "...", logger.WithError(err))` |
| `gin.Default()` | `gin.New()` + `GinLogger` + `GinRecovery` |
| `lumberjack` / logs a archivo | eliminar; Docker/Nomad maneja stdout |

## 6. Estándar de errores

Correcto:

```go
logger.Error(ctx, "failed to create user",
    logger.Event("user.create.failed"),
    logger.WithError(err),
    logger.ErrorKind("db"),
    logger.UserID(userID),
)
```

Incorrecto:

```go
logger.Infof(ctx, "Error al crear usuario: %v", err)
logger.Error(ctx, "Error: "+err.Error())
logger.Error(ctx, "failed", "err", err)
```

Usar siempre:

| Campo | Uso |
|---|---|
| `error` | Error real (`logger.WithError(err)`) |
| `error_kind` | Categoría: `db`, `validation`, `external_api`, `auth`, `timeout`, `panic` |
| `error_code` | Código interno/externo si existe |
| `retryable` | `true` si corresponde reintento |

## 7. Eventos recomendados

Eventos Auth:

```text
auth.login.success
auth.login.failed
auth.token.refresh.success
auth.token.refresh.failed
auth.jwt.validation.failed
auth.api_key.validation.failed
```

Eventos DSpace:

```text
dspace.login.success
dspace.login.failed
dspace.document.download.success
dspace.document.download.failed
dspace.document.upload.success
dspace.document.upload.failed
dspace.metadata.patch.failed
```

Eventos Firma:

```text
signature.request.created
signature.request.failed
signature.document.signed
signature.document.uploaded
signature.batch.started
signature.batch.completed
signature.batch.failed
```

Eventos Contraloría:

```text
contract.created
contract.updated
contract.status.changed
contract.status.change.failed
contract.document.uploaded
contract.document.deleted
workflow.transition.success
workflow.transition.failed
```

## 8. Campos de dominio recomendados

Firma:

```text
signature_request_id
batch_request_id
document_id
document_uuid
signature_type
document_status
external_item_ref
external_doc_ref
```

DSpace:

```text
dspace_uuid
collection_uuid
workspace_item_id
bitstream_uuid
operation
external_status
```

Contraloría:

```text
contract_id
num_convenio
num_presupuesto
workflow_id
workflow_status
old_status
new_status
cost_center_id
```

Auth:

```text
auth_method
token_type
login_result
failure_reason
user_id
role
system_id
application_id
```

## 9. Compatibilidad temporal

Si el proyecto tiene muchas llamadas antiguas tipo `utils.LogError(...)`, se permite una capa temporal `utils/logger_compat.go`, pero debe cumplir:

- `LogError` emite nivel `ERROR`.
- Si recibe un `error`, lo entrega como campo `error`.
- No debe escribir a archivos ni usar colores ANSI en producción.

Ejemplo mínimo:

```go
func LogError(format string, args ...any) {
    logger.Errorf(context.TODO(), format, args...)
}
```

Objetivo final: reemplazar compat por llamadas estructuradas con `logger.Event`, `logger.WithError`, etc.

## 10. Checklist de aceptación

Antes de mergear en cada proyecto:

```bash
go build ./...
```

Buscar que no queden:

```bash
rg 'gin\.Default\('
rg 'fmt\.Print|fmt\.Printf|fmt\.Println'
rg 'panic\('
rg 'log\.Print|log\.Fatal|log\.Panic'
rg 'lumberjack'
```

Validar en ejecución que cada línea de stdout sea JSON válido y contenga al menos:

```text
time level service env msg
```

Para requests HTTP debe contener:

```text
event=http.request method route status latency_ms request_id
```
