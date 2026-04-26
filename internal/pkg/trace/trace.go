package trace

import (
	"context"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	service = "soap"
)

//nolint:gochecknoglobals
var Version = "local"

type Config struct {
	Endpoint        string        `env:"TRACE_ENDPOINT"         env-default:"jaeger:4317" yaml:"endpoint"`
	Ratio           float64       `env:"TRACE_RATIO"            env-default:"1.0"         yaml:"ratio"`
	GracefulTimeout time.Duration `env:"TRACE_GRACEFUL_TIMEOUT" env-default:"10s"         yaml:"graceful_timeout"`
}

// SetupTracing returns an OpenTelemetry TracerProvider configured to use
// the OTLP/gRPC exporter that will send spans to the provided endpoint
// (host:port). Modern Jaeger accepts OTLP natively on port 4317. The
// returned TracerProvider also uses a Resource configured with information
// about the application.
func SetupTracing(ctx context.Context, cfg Config) (*tracesdk.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}
	resources, err := resource.New(ctx,
		resource.WithProcessRuntimeDescription(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(service),
			semconv.ServiceVersion(Version),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resources),
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(cfg.Ratio))),
	)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, // W3C Trace Context format; https://www.w3.org/TR/trace-context/
		),
	)

	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error(context.Background(), "Got otel error", "err", err)
	}))

	return tp, nil
}
