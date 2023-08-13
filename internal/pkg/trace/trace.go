package trace

import (
	"context"
	"time"

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

var (
	//nolint:gochecknoglobals
	Version = "local"
)

type Config struct {
	Endpoint        string        `yaml:"endpoint" env:"TRACE_ENDPOINT" env-default:"http://jaeger:14268/api/traces"`
	Ratio           float64       `yaml:"ratio" env:"TRACE_RATIO" env-default:"1.0"`
	GracefulTimeout time.Duration `yaml:"graceful_timeout" env:"TRACE_GRACEFUL_TIMEOUT" env-default:"10s"`
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

	return tp, nil
}
