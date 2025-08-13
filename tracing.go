package jsondb

import (
	"context"

	"azugo.io/opentelemetry"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func Tracing(ctx context.Context, tracer oteltrace.Tracer, propagator propagation.TextMapPropagator, spfmt opentelemetry.InstrumentationSpanNameFormatter, op string, args ...any) (func(err error), bool) {
	method, ok := InstrExec(op, args...)
	if !ok {
		return nil, false
	}

	spanName := spfmt(ctx, op, args...)
	if spanName == "" {
		spanName = "CALL " + method
	}

	opts := []oteltrace.SpanStartOption{
		oteltrace.WithAttributes(
			semconv.DBSystemPostgreSQL,
		),
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	}

	_, span := tracer.Start(opentelemetry.FromContext(ctx), spanName, opts...)

	return func(err error) {
		if err != nil {
			span.SetStatus(codes.Error, err.Error())

			span.RecordError(err, oteltrace.WithStackTrace(true))
		}

		span.End()
	}, true
}
