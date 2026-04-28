package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

type Config struct {
	ServiceName string
	Level       slog.Level
	Format      Format
	Output      io.Writer
}

func (c *Config) defaults() {
	if c.Level == 0 {
		c.Level = slog.LevelInfo
	}
	if c.Format == "" {
		c.Format = FormatJSON
	}
	if c.Output == nil {
		c.Output = os.Stdout
	}
}

func Init(cfg Config) *slog.Logger {
	cfg.defaults()

	opts := &slog.HandlerOptions{
		Level: cfg.Level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(time.Now().UTC().Format(time.RFC3339Nano))
			}
			return a
		},
	}

	var handler slog.Handler
	if cfg.Format == FormatText {
		handler = slog.NewTextHandler(cfg.Output, opts)
	} else {
		opts.AddSource = true
		handler = slog.NewJSONHandler(cfg.Output, opts)
	}

	logger := slog.New(handler).With(slog.String("service", cfg.ServiceName))
	slog.SetDefault(logger)
	return logger
}

func InitFromEnv() *slog.Logger {
	cfg := Config{
		ServiceName: os.Getenv("SERVICE_NAME"),
		Level:       parseLevel(os.Getenv("LOG_LEVEL")),
		Format:      Format(strings.ToLower(os.Getenv("LOG_FORMAT"))),
	}
	return Init(cfg)
}

func parseLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func FromContext(ctx context.Context) *slog.Logger {
	logger := slog.Default()
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		return logger.With(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return logger
}

func WithError(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.String("error", err.Error())
}

func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).InfoContext(ctx, msg, args...)
}

func Infof(ctx context.Context, format string, args ...any) {
	logf(ctx, slog.LevelInfo, format, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WarnContext(ctx, msg, args...)
}

func Warnf(ctx context.Context, format string, args ...any) {
	logf(ctx, slog.LevelWarn, format, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).ErrorContext(ctx, msg, args...)
}

func Errorf(ctx context.Context, format string, args ...any) {
	logf(ctx, slog.LevelError, format, args...)
}

func Debug(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).DebugContext(ctx, msg, args...)
}

func Debugf(ctx context.Context, format string, args ...any) {
	logf(ctx, slog.LevelDebug, format, args...)
}

func Fatal(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).ErrorContext(ctx, msg, args...)
	os.Exit(1)
}

func Fatalf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])
	r := slog.NewRecord(time.Now(), slog.LevelError, msg, pcs[0])
	_ = slog.Default().Handler().Handle(ctx, r)
	os.Exit(1)
}

func logf(ctx context.Context, level slog.Level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	_ = FromContext(ctx).Handler().Handle(ctx, r)
}
