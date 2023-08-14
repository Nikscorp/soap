//nolint:ireturn
package logger

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/slog"
)

type CustomHandler struct {
	inner slog.Handler
}

func NewCustomHandler(inner slog.Handler) *CustomHandler {
	return &CustomHandler{inner: inner}
}

func (cs *CustomHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return cs.inner.Enabled(ctx, level)
}

func (cs *CustomHandler) Handle(ctx context.Context, rec slog.Record) error {
	span := trace.SpanFromContext(ctx)
	spanContext := span.SpanContext()
	if spanContext.IsValid() {
		if rec.Level == slog.LevelError {
			span.SetStatus(codes.Error, rec.Message)
		} else {
			span.SetStatus(codes.Ok, rec.Message)
			span.AddEvent(rec.Message)
		}

		rec.Attrs(func(a slog.Attr) bool {
			span.SetAttributes(attribute.String("slog."+a.Key, fmt.Sprint(a.Value)))
			return true
		})

		rec.AddAttrs(
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}

	return cs.inner.Handle(ctx, rec)
}

func (cs *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CustomHandler{inner: cs.inner.WithAttrs(attrs)}
}

func (cs *CustomHandler) WithGroup(name string) slog.Handler {
	return &CustomHandler{inner: cs.inner.WithGroup(name)}
}
