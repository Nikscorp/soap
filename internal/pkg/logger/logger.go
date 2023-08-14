package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	//nolint:gochecknoglobals
	global *slog.Logger
	//nolint:gochecknoglobals
	once sync.Once
)

type Opts struct {
	Level slog.Leveler
}

//nolint:gochecknoinits
func init() {
	global = slog.New(NewCustomHandler(slog.NewJSONHandler(os.Stdout, nil)))
	slog.SetDefault(global)
}

func SetUpLogger(opts *Opts) {
	once.Do(func() {
		slopts := &slog.HandlerOptions{
			Level: opts.Level,
		}
		global = slog.New(NewCustomHandler(slog.NewJSONHandler(os.Stdout, slopts)))
		slog.SetDefault(global)
	})
}

func Logger() *slog.Logger {
	return global
}

type LogPrinter func(v ...interface{})

func (p LogPrinter) Print(v ...interface{}) {
	p(v...)
}

type contextKey struct{}

//nolint:gochecknoglobals
var loggerKey = contextKey{}

func ToContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return v
	}

	return global
}

func ContextWithAttrs(ctx context.Context, attrs ...any) context.Context {
	span := trace.SpanFromContext(ctx)
	spanContext := span.SpanContext()
	if spanContext.IsValid() {
		r := slog.NewRecord(time.Now(), slog.LevelInfo, "", 0)
		r.Add(attrs...)
		r.Attrs(func(a slog.Attr) bool {
			span.SetAttributes(attribute.String("slog."+a.Key, fmt.Sprint(a.Value)))
			return true
		})
	}

	return ToContext(ctx, FromContext(ctx).With(attrs...))
}

func Debug(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).DebugContext(ctx, msg, args...)
}

func Info(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).InfoContext(ctx, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).WarnContext(ctx, msg, args...)
}

func Error(ctx context.Context, msg string, args ...any) {
	FromContext(ctx).ErrorContext(ctx, msg, args...)
}
