package traversal

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("gremlin.traversal")
