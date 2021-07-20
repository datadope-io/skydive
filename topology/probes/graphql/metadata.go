//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/graphql
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE
package graphql

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// easyjson:json
// gendecoder
type Events map[string]Event

// easyjson:json
// gendecoder
type Event struct {
	// TTL is the duration, in seconds, when this event should be closed after UpdatedAt date
	TTL       int64
	Level     string
	Payload   []PayloadValues
	CreatedAt graph.Time
	UpdatedAt graph.Time
}

// PayloadValues extra metadata that could be added to the events
// easyjson:json
// gendecoder
type PayloadValues struct {
	Key   string
	Value string
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var a Events
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Events metadata %s: %s", string(raw), err)
	}

	return &a, nil
}
