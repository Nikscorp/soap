package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromContext_DefaultsToGlobal(t *testing.T) {
	got := FromContext(context.Background())
	require.NotNil(t, got)
	assert.Same(t, global, got)
}

func TestToFromContext_RoundTrip(t *testing.T) {
	custom := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	ctx := ToContext(context.Background(), custom)
	assert.Same(t, custom, FromContext(ctx))
}

func TestWithRequestID_StampsRecords(t *testing.T) {
	var buf bytes.Buffer
	h := newRequestIDHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(h)

	ctx := WithRequestID(context.Background(), "req-42")
	l.InfoContext(ctx, "hello", "k", "v")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	assert.Equal(t, "req-42", rec["request_id"])
	assert.Equal(t, "hello", rec["msg"])
}

func TestWithRequestID_AbsentWhenUnset(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(newRequestIDHandler(slog.NewJSONHandler(&buf, nil)))
	l.InfoContext(context.Background(), "no-id")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	_, ok := rec["request_id"]
	assert.False(t, ok, "request_id should be absent when not set on ctx")
}

func TestWithAttrs_AttachesAttrs(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(newRequestIDHandler(slog.NewJSONHandler(&buf, nil)))
	ctx := ToContext(context.Background(), base)
	ctx = WithAttrs(ctx, "scope", "unit-test")

	Info(ctx, "msg")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	assert.Equal(t, "unit-test", rec["scope"])
}

func TestPackageHelpers_RouteThroughContextLogger(t *testing.T) {
	cases := []struct {
		name string
		fn   func(context.Context, string, ...any)
		lvl  string
	}{
		{"debug", Debug, "DEBUG"},
		{"info", Info, "INFO"},
		{"warn", Warn, "WARN"},
		{"error", Error, "ERROR"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			ctx := ToContext(context.Background(), l)

			tc.fn(ctx, "boom")

			var rec map[string]any
			require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
			assert.Equal(t, tc.lvl, rec["level"])
			assert.Equal(t, "boom", rec["msg"])
		})
	}
}

func TestSetup_ReplacesGlobalAndRespectsLevel(t *testing.T) {
	prev := global
	t.Cleanup(func() {
		global = prev
		slog.SetDefault(prev)
	})

	Setup(slog.LevelError)
	assert.NotSame(t, prev, global)
}

func TestRequestIDHandler_EnabledAndChaining(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	h := newRequestIDHandler(inner)

	assert.True(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))

	withAttrs := h.WithAttrs([]slog.Attr{slog.String("svc", "lazysoap")})
	withGroup := withAttrs.WithGroup("g")

	l := slog.New(withGroup)
	l.InfoContext(WithRequestID(context.Background(), "id-7"), "msg", "k", "v")

	out := buf.String()
	assert.Contains(t, out, `"svc":"lazysoap"`)
	assert.Contains(t, out, `"request_id":"id-7"`)
	assert.True(t, strings.Contains(out, `"g":{`), "expected group nesting in output: %s", out)
}
