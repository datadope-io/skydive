package packetinjector

import (
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("packetinjector")
