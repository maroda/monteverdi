package monteverdi

import (
	"context"
	"fmt"

	"github.com/honeycombio/otel-config-go/otelconfig"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitOTelHNY uses the Honeycomb library to interface with OTel
func InitOTelHNY() (func(), error) {
	otelShutdown, err := otelconfig.ConfigureOpenTelemetry()
	if err != nil {
		return nil, fmt.Errorf("failed to configure OpenTelemetry: %w", err)
	}
	return func() { otelShutdown() }, nil
}

// InitOTelGRF uses the Grafana recommended configuration including Baggage for propagation
func InitOTelGRF() (*sdktrace.TracerProvider, error) {
	exporter, err := otlptrace.New(context.Background(), otlptracehttp.NewClient())
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))
	return tp, err
}
