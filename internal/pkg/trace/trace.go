package trace

import (
	"context"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
)

const (
	service = "soap"
)

//nolint:gochecknoglobals
var Version = "local"

type Config struct {
	Endpoint        string        `env:"TRACE_ENDPOINT"         env-default:"http://jaeger:14268/api/traces" yaml:"endpoint"`
	Ratio           float64       `env:"TRACE_RATIO"            env-default:"1.0"                            yaml:"ratio"`
	GracefulTimeout time.Duration `env:"TRACE_GRACEFUL_TIMEOUT" env-default:"10s"                            yaml:"graceful_timeout"`
}

// SetupTracing returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func SetupTracing(cfg Config) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(
		jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.Endpoint)),
	)
	if err != nil {
		return nil, err
	}
	resources, err := resource.New(context.Background(),
		resource.WithProcessRuntimeDescription(), // This option configures a set of Detectors that discover process information
		resource.WithOS(),                        // This option configures a set of Detectors that discover OS information
		resource.WithContainer(),                 // This option configures a set of Detectors that discover container information
		resource.WithHost(),                      // This option configures a set of Detectors that discover host information
		resource.WithAttributes(
			semconv.ServiceName(service),
			semconv.ServiceVersion(Version),
		), // Or specify resource attributes directly
	)
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
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
