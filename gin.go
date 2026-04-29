package logger

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

const requestIDHeader = "X-Request-ID"

func GinLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery
		requestID := ensureRequestID(c)

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
			slog.String("event", "http.request"),
			slog.Int("status", statusCode),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("route", c.FullPath()),
			slog.String("query", rawQuery),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.String("request_id", requestID),
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

		logger.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

func GinRecovery(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestID := ensureRequestID(c)
				attrs := []slog.Attr{
					slog.String("event", "http.panic"),
					slog.String("error", panicToString(recovered)),
					slog.String("error_kind", "panic"),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.String("route", c.FullPath()),
					slog.String("client_ip", c.ClientIP()),
					slog.String("user_agent", c.Request.UserAgent()),
					slog.String("request_id", requestID),
					slog.Int("status", http.StatusInternalServerError),
					slog.String("stack", string(debug.Stack())),
				}

				if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
					attrs = append(attrs,
						slog.String("trace_id", span.SpanContext().TraceID().String()),
						slog.String("span_id", span.SpanContext().SpanID().String()),
					)
				}

				logger.LogAttrs(c.Request.Context(), slog.LevelError, "panic recovered", attrs...)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		c.Next()
	}
}

func ensureRequestID(c *gin.Context) string {
	requestID := c.GetHeader(requestIDHeader)
	if requestID == "" {
		requestID = newRequestID()
	}
	c.Set("request_id", requestID)
	c.Writer.Header().Set(requestIDHeader, requestID)
	return requestID
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

func panicToString(v any) string {
	if err, ok := v.(error); ok {
		return err.Error()
	}
	return slog.AnyValue(v).String()
}
