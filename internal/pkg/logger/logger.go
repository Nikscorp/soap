// Package logger wraps log/slog with two small conveniences:
//   - a context-attached *slog.Logger, so handlers and helpers can pass
//     ctx around and pick up the current scope's attrs automatically;
//   - a request-id-aware handler that stamps every record with the
//     request id stored in ctx by the HTTP middleware.
//
// Setup must be called once early in main; until then a JSON logger
// targeting stdout is installed by default.
package logger

import (
	"context"
	"log/slog"
	"os"
)

//nolint:gochecknoglobals
var global = slog.New(newRequestIDHandler(slog.NewJSONHandler(os.Stdout, nil)))

//nolint:gochecknoinits
func init() {
	slog.SetDefault(global)
}

// Setup configures the package-level logger. It is intended to be called
// once during program start-up; subsequent calls replace the global.
func Setup(level slog.Leveler) {
	global = slog.New(newRequestIDHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))
	slog.SetDefault(global)
}

type loggerCtxKey struct{}

type requestIDCtxKey struct{}

// ToContext stores l on ctx so FromContext can retrieve it.
func ToContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// FromContext returns the logger stored on ctx by ToContext, or the
// package-level default if none is present.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return l
	}
	return global
}

// WithAttrs returns a context whose logger has the given attrs attached.
// Subsequent FromContext / Debug / Info / Warn / Error calls will include
// those attrs on every record.
func WithAttrs(ctx context.Context, args ...any) context.Context {
	return ToContext(ctx, FromContext(ctx).With(args...))
}

// WithRequestID stores id on ctx so the request-id handler can stamp it
// onto every record produced from this ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey{}, id)
}

// requestIDFromContext returns the request id stored by WithRequestID,
// or the empty string if none is present.
func requestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return id
	}
	return ""
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

// requestIDHandler is a slog.Handler that adds a request_id attr to every
// record whose context carries one (set via WithRequestID).
type requestIDHandler struct {
	inner slog.Handler
}

func newRequestIDHandler(inner slog.Handler) *requestIDHandler {
	return &requestIDHandler{inner: inner}
}

func (h *requestIDHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *requestIDHandler) Handle(ctx context.Context, rec slog.Record) error {
	if id := requestIDFromContext(ctx); id != "" {
		rec.AddAttrs(slog.String("request_id", id))
	}
	return h.inner.Handle(ctx, rec)
}

func (h *requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &requestIDHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *requestIDHandler) WithGroup(name string) slog.Handler {
	return &requestIDHandler{inner: h.inner.WithGroup(name)}
}
