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
type NetworkInfo struct {
	Ethernet   *EthernetInfo   `json:",omitempty"`
	IP         []string        `json:",omitempty"`
	PseudoWire *PseudoWireInfo `json:",omitempty"`
}

// easyjson:json
// gendecoder
type PseudoWireInfo struct {
	ID       string
	Neighbor string
}

// easyjson:json
// gendecoder
type EthernetInfo struct {
	VLAN *VLANInfo `json:",omitempty"`
}

// easyjson:json
// gendecoder
type VLANInfo struct {
	Access *VLANAccessInfo `json:",omitempty"`
	Trunk  *VLANTrunkInfo  `json:",omitempty"`
}

// easyjson:json
// gendecoder
type VLANAccessInfo struct {
	AcceptedVIDs []int
	// VID=0 is not valid, so it's safe to omit this field if value is zero
	NativeVLANVID int64 `json:",omitempty"`
}

// easyjson:json
// gendecoder
type VLANTrunkInfo struct {
	VID int
}

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
	DeletedAt graph.Time
	// ChildChain contains the list of node names to reach the originator node of the event.
	// This is used when an event is propagated to its parents.
	// Helps users to identify the origin of the event.
	ChildChain []string
	// ChildID contains the originator node ID of the event.
	// This is used when an event is propagated to its parents.
	// Helps to delete the event when its original event is deleted.
	ChildID graph.Identifier
}

// PayloadValues extra metadata that could be added to the events
// easyjson:json
// gendecoder
type PayloadValues struct {
	Key   string
	Value string
}

// NetworNetworkInfoMetadataDecoder implements a json message raw decoder for network info
func NetworkInfoMetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var a NetworkInfo
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("unable to unmarshal NetworkInfo metadata %s: %s", string(raw), err)
	}

	return &a, nil
}

// EventsMetadataDecoder implements a json message raw decoder for events
func EventsMetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var a Events
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("unable to unmarshal Events metadata %s: %s", string(raw), err)
	}

	return &a, nil
}
