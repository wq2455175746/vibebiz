package observability

import (
	"context"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type tracingSetup struct {
	tracerProvider trace.TracerProvider
	shutdown       func(context.Context) error
}

func setupTracing(ctx context.Context, serviceName, plane string) (tracingSetup, error) {
	res := resource.NewSchemaless(
		attribute.String("service.name", serviceName),
		attribute.String("service.namespace", "vibebiz"),
		attribute.String("deployment.environment", deploymentEnvironment()),
		attribute.String("plane", plane),
	)

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerFromEnv()),
	}
	tp := sdktrace.NewTracerProvider(opts...)
	if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" {
		exporter, exportErr := newSpanExporter(ctx)
		if exportErr != nil {
			return tracingSetup{}, exportErr
		}
		tp = sdktrace.NewTracerProvider(append(opts, sdktrace.WithBatcher(exporter))...)
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tracingSetup{tracerProvider: tp, shutdown: tp.Shutdown}, nil
}

func newSpanExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))) {
	case "grpc":
		return otlptracegrpc.New(ctx)
	default:
		return otlptracehttp.New(ctx)
	}
}

func samplerFromEnv() sdktrace.Sampler {
	ratio := samplerRatioFromEnv()
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER"))) {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

func samplerRatioFromEnv() float64 {
	raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	if raw == "" {
		return 1
	}
	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 1
	}
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func deploymentEnvironment() string {
	for _, pair := range strings.Split(os.Getenv("OTEL_RESOURCE_ATTRIBUTES"), ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 && kv[0] == "deployment.environment" && kv[1] != "" {
			return kv[1]
		}
	}
	return "local"
}
