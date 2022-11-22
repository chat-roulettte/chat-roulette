package o11y

import (
	"context"
	"fmt"
	"runtime"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc/credentials"

	"github.com/chat-roulettte/chat-roulette/internal/config"
	"github.com/chat-roulettte/chat-roulette/internal/version"
)

const (
	ServiceName = "chat-roulette"

	HoneycombEndpoint          = "api.honeycomb.io:443"
	HTTPHeaderHoneycombTeam    = "x-honeycomb-team"
	HTTPHeaderHoneycombDataset = "x-honeycomb-dataset"
)

// NewTracerProvider creates an OpenTelemetry TracerProvider.
func NewTracerProvider(cfg *config.TracingConfig) (*sdktrace.TracerProvider, error) {
	var tp *sdktrace.TracerProvider

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(ServiceName),
			semconv.ProcessRuntimeNameKey.String(getRuntimeName()),
			semconv.ProcessRuntimeVersionKey.String(runtime.Version()),
			attribute.String("service.commit_sha", version.TruncatedCommitSha()),
			attribute.String("service.build_date", version.BuildDate),
		),
		resource.WithHost(),
		resource.WithOSType(),
	)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create tracing resource")
	}

	switch cfg.Exporter {
	case config.TracingExporterJaeger:
		// Configure the Jaeger exporter
		exp, err := jaeger.New(
			jaeger.WithCollectorEndpoint(
				jaeger.WithEndpoint(cfg.Jaeger.Endpoint)),
		)
		if err != nil {
			return nil, err
		}

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)

	case config.TracingExporterHoneycomb:
		// Configure the Honeycomb exporter
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(HoneycombEndpoint),
			otlptracegrpc.WithHeaders(map[string]string{
				HTTPHeaderHoneycombTeam:    cfg.Honeycomb.Team,
				HTTPHeaderHoneycombDataset: cfg.Honeycomb.Dataset,
			}),
			otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")),
		}

		client := otlptracegrpc.NewClient(opts...)

		exp, err := otlptrace.New(context.Background(), client)
		if err != nil {
			return nil, err
		}

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
		)
	default:
		return nil, fmt.Errorf("unsupported tracing exporter")
	}

	// Register TracerProvider globally
	otel.SetTracerProvider(tp)

	// Use the W3C trace context and baggage propagators for compatibility with most vendors
	// https://www.w3.org/TR/trace-context/
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}

func ShutdownTracer(ctx context.Context, logger hclog.Logger, tp *sdktrace.TracerProvider) {
	if err := tp.ForceFlush(ctx); err != nil {
		logger.Warn("failed to flush tracer")
	}

	if err := tp.Shutdown(ctx); err != nil {
		logger.Warn("failed to shutdown tracer")
	}
}

// getRuntimeName returns the name of the runtime
// See: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/process.md#go-runtimes
func getRuntimeName() string {
	if runtime.Compiler == "gc" {
		return "go"
	}
	return runtime.Compiler
}
