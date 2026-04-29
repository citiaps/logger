# citiaps/logger

Logger JSON estructurado para servicios Go de CITIAPS. La salida está pensada para `stdout`/`stderr` en Docker/Nomad y posterior ingesta en VictoriaLogs.

## Contrato base

Todo log emitido por este paquete usa estos campos base:

| Campo | Fuente | Descripción |
|---|---|---|
| `time` | logger | Timestamp UTC RFC3339Nano |
| `level` | logger | `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `service` | `SERVICE_NAME`, `OTEL_SERVICE_NAME`, fallback `unknown-service` | Nombre estable del servicio |
| `env` | `ENV`, `GO_REST_ENV`, `APP_ENV`, `GIN_MODE` | Ambiente si está definido |
| `version` | `SERVICE_VERSION`, `VERSION`, `GIT_SHA`, `COMMIT_SHA` | Versión si está definida |
| `msg` | aplicación | Mensaje humano corto |

Cuando hay contexto OpenTelemetry, `FromContext(ctx)` agrega:

| Campo | Descripción |
|---|---|
| `trace_id` | Trace ID OTel |
| `span_id` | Span ID OTel |

## Variables de entorno

```bash
SERVICE_NAME=e-firma-backend
ENV=prod
SERVICE_VERSION=1.2.3
LOG_LEVEL=info
LOG_FORMAT=json
```

`LOG_LEVEL` acepta: `debug`, `info`, `warn`, `error`.

`LOG_FORMAT=json` es el default recomendado. Usar `text` solo para desarrollo local si se necesita.

## Uso base

```go
package main

import "github.com/citiaps/logger"

func main() {
    logger.InitFromEnv()
}
```

## Gin

No usar `gin.Default()` porque agrega logging/recovery de Gin en texto plano.

Usar:

```go
app := gin.New()
app.Use(logger.GinLogger(nil))
app.Use(logger.GinRecovery(nil))
```

`GinLogger` emite:

| Campo | Descripción |
|---|---|
| `event` | Siempre `http.request` |
| `method` | Método HTTP |
| `path` | Path real recibido |
| `route` | Ruta Gin normalizada, ej. `/api/v1/users/:id` |
| `status` | Status HTTP |
| `latency_ms` | Latencia en milisegundos |
| `client_ip` | IP cliente |
| `user_agent` | User-Agent |
| `request_id` | Header `X-Request-ID` o generado automáticamente |
| `body_size` | Tamaño de respuesta |

Por defecto `event` es `http.request`. Si un handler conoce el resultado de dominio, puede anotar la request para que el log HTTP final incluya ese evento:

```go
func DownloadDocument(c *gin.Context) {
    document, err := service.Download(c.Request.Context(), id)
    if err != nil {
        logger.SetRequestEvent(c, "dspace.document.download.failed")
        logger.SetRequestError(c, err,
            logger.ErrorKind("external_api"),
            logger.Operation("download"),
            slog.Int("external_status", 502),
            slog.String("external_item_ref", itemRef),
        )
        c.JSON(500, gin.H{"message": "failed to download document"})
        return
    }
    c.JSON(200, document)
}
```

El log final de la request quedaría con `event=dspace.document.download.failed` y `http_event=http.request`, lo que permite alertar directamente por evento de dominio y conservar métricas HTTP.

`GinRecovery` emite panics como JSON:

| Campo | Descripción |
|---|---|
| `event` | Siempre `http.panic` |
| `error` | Panic/error recuperado |
| `error_kind` | Siempre `panic` |
| `stack` | Stack trace |
| `request_id` | Mismo request ID de la request |

## Semántica estándar

Preferir llamadas estructuradas:

```go
logger.Info(ctx, "contract status changed",
    logger.Event("contract.status.changed"),
    logger.UserID(userID),
    "contract_id", contractID,
    "old_status", oldStatus,
    "new_status", newStatus,
)
```

Para errores:

```go
logger.Error(ctx, "failed to upload document to dspace",
    logger.Event("dspace.document.upload.failed"),
    logger.WithError(err),
    logger.ErrorKind("external_api"),
    logger.Retryable(true),
    logger.Operation("upload"),
    "external_item_ref", itemRef,
)
```

Reglas:

- El error real va en `error`, no dentro de `msg`.
- El nombre estable del evento va en `event`.
- Usar `route` para agrupar HTTP; `path` queda como referencia exacta.
- Evitar variantes como `err`, `error_msg`, `userId`, `userid`.

## Helpers disponibles

```go
logger.Event("auth.login.failed")
logger.WithError(err)
logger.ErrorKind("db")
logger.ErrorCode("DB_TIMEOUT")
logger.Retryable(true)
logger.RequestID(requestID)
logger.CorrelationID(correlationID)
logger.UserID(userID)
logger.Route(route)
logger.Method(method)
logger.Status(status)
logger.LatencyMS(ms)
logger.Operation("upload")
```

Helpers Gin disponibles:

```go
logger.SetRequestEvent(c, "dspace.document.download.failed")
logger.SetRequestError(c, err, logger.ErrorKind("external_api"))
logger.AddRequestAttrs(c, slog.String("document_id", documentID))
```

## Migración gradual

`Infof`, `Warnf`, `Errorf` y `Fatalf` existen para migración desde `log.Printf`, pero el objetivo final es usar logs estructurados con key/value.

Temporal:

```go
logger.Errorf(ctx, "failed to create user: %v", err)
```

Final:

```go
logger.Error(ctx, "failed to create user",
    logger.Event("user.create.failed"),
    logger.WithError(err),
    logger.ErrorKind("db"),
    logger.UserID(userID),
)
```
