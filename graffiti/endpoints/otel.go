package endpoints

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("graffiti.endpoints")
