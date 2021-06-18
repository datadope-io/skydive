package graph

import (
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("graffiti.graph")

// addFilterAttribute add a string representation of the filter as an span attribute, only if filter is defined
func addFilterAttribute(span trace.Span, m ElementMatcher, attrName string) {
	if m != nil {
		f, err := m.Filter()
		if err == nil {
			span.SetAttributes(attribute.Key(attrName).String(f.String()))
		}
	}
}

func addTimeFilterAttribute(span trace.Span, ctx Context) {
	if ctx.TimeSlice != nil {
		start := time.Unix(ctx.TimeSlice.Start/1000, ctx.TimeSlice.Start%1000)
		last := time.Unix(ctx.TimeSlice.Last/1000, ctx.TimeSlice.Last%1000)

		span.SetAttributes(attribute.Key("time.slice.start").String(start.Format(time.RFC3339)))
		span.SetAttributes(attribute.Key("time.slice.last").String(last.Format(time.RFC3339)))
		span.SetAttributes(attribute.Key("time.point").Bool(ctx.TimePoint))
	}
}
