//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/netexternal
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE
package netexternal

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
)

// easyjson:json
// gendecoder
type AlarmsML map[string]AlarmsMLEvent

// easyjson:json
// gendecoder
type AlarmsMLEvent struct {
	Id          string
	Span        int64 
	Function    string
	Score       string
	Probability string
	Field       string
	CreatedAt   graph.Time
	UpdatedAt   graph.Time
	DeletedAt   graph.Time
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var a AlarmsML
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("unable to unmarshal AlarmsML metadata %s: %s", string(raw), err)
	}

	return &a, nil
}
