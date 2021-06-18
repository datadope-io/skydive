package server

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("graffiti.api.server")
