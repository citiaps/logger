package logger

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

func GinLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		level := slog.LevelInfo
		if statusCode >= 500 {
			level = slog.LevelError
		} else if statusCode >= 400 {
			level = slog.LevelWarn
		}

		attrs := []slog.Attr{
			slog.Int("status", statusCode),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", rawQuery),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Float64("latency_ms", float64(latency.Microseconds())/1000),
			slog.Int("body_size", c.Writer.Size()),
		}

		if len(c.Errors) > 0 {
			errStrs := make([]string, len(c.Errors))
			for i, e := range c.Errors {
				errStrs[i] = e.Err.Error()
			}
			attrs = append(attrs, slog.Any("gin_errors", errStrs))
		}

		if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
			attrs = append(attrs,
				slog.String("trace_id", span.SpanContext().TraceID().String()),
				slog.String("span_id", span.SpanContext().SpanID().String()),
			)
		}

		logger.LogAttrs(c.Request.Context(), level, "request", attrs...)
	}
}
