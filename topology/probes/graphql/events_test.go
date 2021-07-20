package graphql

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
	"github.com/stretchr/testify/assert"
)

func TestCloseEvent(t *testing.T) {
	createEventTime := time.Unix(100, 100)
	testCases := []struct {
		desc                 string
		event                model.CloseEventInput
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
	}{
		{
			desc: "trying to delete an event from a node without Metadata.Event produces an error",
			event: model.CloseEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "unable to delete event: Field not found",
		},
		{
			desc: "trying to delete an event from a node with Metadata.Event empty produces an error",
			event: model.CloseEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedError: "unable to delete event: event ID 'Prefix_someId' does not exists in node a",
		},
		{
			desc: "trying to delete an event with empty ID produces an error",
			event: model.CloseEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedError: "ID should not be empty",
		},
		{
			desc: "trying to delete an event with empty query produces an error",
			event: model.CloseEventInput{
				GremlinQuery: "",
				ID:           "Prefix_someId",
			},
			expectedError: "parsing gremlin query: found \"\", expected `G`",
		},
		{
			desc: "deleting an event",
			event: model.CloseEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			},
		},
		{
			desc: "deleting an event from a node with two events",
			event: model.CloseEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
					"Prefix_otherId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_otherId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				g.AddNode(n)
			}

			r := Resolver{Graph: g}

			err := r.closeEvent(tC.event)
			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			} else {
				// Check the node has been modified
				modNode := g.GetNode(graph.Identifier("a"))
				assert.Equal(t, tC.expectedNodeMetadata, modNode.Metadata)
			}
		})
	}
}

func TestCreateEvent(t *testing.T) {
	ttl200 := 200
	ttl400 := 400
	createEventTime := time.Unix(100, 100)

	testCases := []struct {
		desc                 string
		event                model.AddEventInput
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
	}{
		// evento con ID vacío ("")
		// evento con query vacía
		{
			desc: "adding an event with empty query produces an error",
			event: model.AddEventInput{
				GremlinQuery: "",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.KeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "parsing gremlin query: found \"\", expected `G`",
		},
		{
			desc: "adding an event with empty ID produces an error",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "",
				Level:        model.EventLevelInfo,
				Payload:      []*model.KeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "ID should not be empty",
		},
		{
			desc: "adding an event to a non existant node produces an error",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','z')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.KeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "expecting only one node, gremlin query returned 0",
		},
		{
			desc: "adding an event to a node without Metadata.Event",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.KeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "adding an event to a node with Metadata.Event empty",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.KeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "adding an event with nil Payload",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				TTL:          &ttl200,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "adding an event with nil TTL",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelCritical,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       0,
						Level:     "CRITICAL",
						Payload:   []PayloadValues{},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "updating an event, new event define all values except CreatedAt",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.KeyValue{{Key: "new", Value: "value"}},
				TTL:          &ttl400,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "original", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(time.Unix(0, 0)),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       400,
						Level:     "WARNING",
						Payload:   []PayloadValues{{Key: "new", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "add a second event",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.KeyValue{{Key: "new", Value: "value"}},
				TTL:          &ttl400,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "original", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(time.Unix(0, 0)),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       200,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "original", Value: "value"}},
						CreatedAt: graph.Time(time.Unix(0, 0)),
						UpdatedAt: graph.Time(time.Unix(0, 0)),
					},
					"Prefix_otherId": Event{
						TTL:       400,
						Level:     "WARNING",
						Payload:   []PayloadValues{{Key: "new", Value: "value"}},
						CreatedAt: graph.Time(createEventTime),
						UpdatedAt: graph.Time(createEventTime),
					},
				},
			},
		},
		{
			desc: "error if Metadata.Event does not have the proper type",
			event: model.AddEventInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.KeyValue{{Key: "new", Value: "value"}},
				TTL:          &ttl400,
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": map[string]string{},
			})},
			expectedError: "unable to add event: invalid Metadata.Events format",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				g.AddNode(n)
			}

			r := Resolver{Graph: g}

			err := r.createEvent(&tC.event, createEventTime)
			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			} else {
				// Check the node has been modified
				modNode := g.GetNode(graph.Identifier("a"))
				assert.Equal(t, tC.expectedNodeMetadata, modNode.Metadata)
			}
		})
	}
}
