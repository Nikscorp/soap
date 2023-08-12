package rest

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

func Ping(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(strings.ToLower(r.URL.Path), "/ping") {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("pong"))
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func Version(version string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasSuffix(strings.ToLower(r.URL.Path), "/version") {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(version))
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// TraceIDToOutHeader is a middleware that appends trace id to the response header.
func TraceIDToOutHeader(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		if span != nil {
			w.Header().Set("x-trace-id", span.SpanContext().TraceID().String())
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func LogRequest(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		route := routePattern(r)
		if strings.HasPrefix(route, "/debug") || strings.HasPrefix(route, "/metrics") {
			next.ServeHTTP(w, r)
			return
		}

		ctx := logger.ContextWithAttrs(r.Context(), "route", route)
		r = r.WithContext(ctx)

		logger.Info(ctx, "New incoming request",
			"method", r.Method,
			"url", maskedURL(r.URL, nil),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"proto", r.Proto,
		)

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		defer func() {
			logger.Info(ctx, "Request completed",
				"status", ww.Status(),
				"size", ww.BytesWritten(),
				"duration", time.Since(start).String(),
			)
		}()
		next.ServeHTTP(ww, r)
	}

	return http.HandlerFunc(fn)
}

func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if pattern := rctx.RoutePattern(); pattern != "" {
		// Pattern is already available
		return pattern
	}

	routePath := r.URL.Path
	if r.URL.RawPath != "" {
		routePath = r.URL.RawPath
	}

	tctx := chi.NewRouteContext()
	if !rctx.Routes.Match(tctx, r.Method, routePath) {
		// No matching pattern, so just return the request path.
		// Depending on your use case, it might make sense to
		// return an empty string or error here instead
		return routePath
	}

	// tctx has the updated pattern, since Match mutates it
	return tctx.RoutePattern()
}

func maskedURL(u *url.URL, keysToHide map[string]struct{}) string {
	// it is save to do shallow copy, cause all inner pointers are immutable
	resURL := *u

	values := resURL.Query()

	for k, v := range values {
		if _, ok := keysToHide[strings.ToLower(k)]; !ok {
			continue
		}
		for i := range v {
			v[i] = "***"
		}
	}

	resURL.RawQuery = values.Encode()
	res := resURL.String()
	if v, err := url.QueryUnescape(res); err == nil {
		res = v
	}
	return res
}
