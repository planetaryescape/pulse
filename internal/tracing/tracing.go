package tracing

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type CollectingExporter struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (e *CollectingExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	e.mu.Lock()
	e.spans = append(e.spans, spans...)
	e.mu.Unlock()
	return nil
}

func (e *CollectingExporter) Shutdown(_ context.Context) error { return nil }

func (e *CollectingExporter) Spans() []sdktrace.ReadOnlySpan {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), e.spans...)
}

func Init(enabled bool) (*CollectingExporter, func(context.Context) error) {
	if !enabled {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return nil, func(context.Context) error { return nil }
	}

	exp := &CollectingExporter{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exp)),
	)
	otel.SetTracerProvider(tp)
	return exp, tp.Shutdown
}

func Tracer() trace.Tracer {
	return otel.Tracer("pulse")
}
