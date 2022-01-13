package graphql

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
	"github.com/stretchr/testify/assert"
)

func TestDeleteExpiredEvents(t *testing.T) {
	tests := []struct {
		desc                 string
		nodeID               graph.Identifier
		thresholdTime        time.Time
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
	}{
		{
			desc:          "error if Metadata.Events have the wrong format",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(0, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": map[string]string{},
			})},
			expectedError: "unable to mark expired events: invalid Metadata.Events format",
		},
		{
			desc:          "events with TTL 0 should not be deleted",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(0, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       0,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(100, 0),
						UpdatedAt: graph.Unix(100, 0),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       0,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(100, 0),
						UpdatedAt: graph.Unix(100, 0),
					},
				},
			},
		},
		{
			desc:          "event older than threshold but less than event+ttl should not be deleted",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(100, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       50,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(80, 0),
						UpdatedAt: graph.Unix(80, 0),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"Prefix_someId": Event{
						TTL:       50,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(80, 0),
						UpdatedAt: graph.Unix(80, 0),
					},
				},
			},
		},
		{
			desc:          "event + ttl older than threshold should be deleted",
			nodeID:        graph.Identifier("a"),
			thresholdTime: time.Unix(100, 0),
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"should_be_deleted": Event{
						TTL:       50,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(30, 0),
						UpdatedAt: graph.Unix(30, 0),
					},
					"should_not_be_deleted": Event{
						TTL:       100,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(30, 0),
						UpdatedAt: graph.Unix(30, 0),
					},
				},
			})},
			expectedNodeMetadata: graph.Metadata{
				"Name": "a",
				"Type": "host",
				"Events": &Events{
					"should_not_be_deleted": Event{
						TTL:       100,
						Level:     "INFO",
						Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
						CreatedAt: graph.Unix(30, 0),
						UpdatedAt: graph.Unix(30, 0),
					},
				},
			},
		},
	}

	for _, tC := range tests {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				err := g.AddNode(n)
				assert.NoError(t, err)
			}

			p := Probe{graph: g}
			err := p.deleteExpiredEvents(tC.nodeID, tC.thresholdTime)

			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			} else {
				// Check the node has been modified
				modNode := g.GetNode(tC.nodeID)
				assert.Equal(t, tC.expectedNodeMetadata, modNode.Metadata)
			}
		})
	}
}

// TestCleanNodesEvents delete old events, do not delete recent events, do nothing with
// node without Events
func TestCleanNodesEvents(t *testing.T) {
	g := newGraph(t)
	p := Probe{graph: g}

	err := g.AddNode(createNode("a", graph.Metadata{
		"Name": "a",
		"Type": "host",
		"Events": &Events{
			"should_be_deleted": Event{
				TTL:       100,
				Level:     "INFO",
				Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
				CreatedAt: graph.Unix(0, 0),
				UpdatedAt: graph.Unix(0, 0),
			},
			"should_not_be_deleted": Event{
				TTL:       200,
				Level:     "INFO",
				Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
				CreatedAt: graph.Unix(0, 0),
				UpdatedAt: graph.Unix(0, 0),
			},
		},
	}))
	assert.NoError(t, err)
	err = g.AddNode(createNode("b", graph.Metadata{
		"Name": "b",
		"Type": "host",
	}))
	assert.NoError(t, err)

	p.cleanNodesEvents(time.Unix(150, 0))

	assert.Equal(t, graph.Metadata{
		"Name": "a",
		"Type": "host",
		"Events": &Events{
			"should_not_be_deleted": Event{
				TTL:       200,
				Level:     "INFO",
				Payload:   []PayloadValues{{Key: "foo", Value: "bar"}},
				CreatedAt: graph.Unix(0, 0),
				UpdatedAt: graph.Unix(0, 0),
			},
		},
	}, g.GetNode("a").Metadata)
	assert.Equal(t, graph.Metadata{"Name": "b", "Type": "host"}, g.GetNode("b").Metadata)
}

func TestCloseEvent(t *testing.T) {
	createEventTime := time.Unix(100, 100)
	testCases := []struct {
		desc                 string
		event                model.EventCloseInput
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
	}{
		{
			desc: "trying to delete an event from a node without Metadata.Event produces an error",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "unable to mark event as deleted: Field not found",
		},
		{
			desc: "trying to delete an event from a node with Metadata.Event empty produces an error",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": &Events{},
			})},
			expectedError: "unable to mark event as deleted: event ID 'Prefix_someId' in node a: event not found",
		},
		{
			desc: "error if Metadata.Event does not have the proper type",
			event: model.EventCloseInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
			},
			nodes: []*graph.Node{createNode("a", graph.Metadata{
				"Name":   "a",
				"Type":   "host",
				"Events": map[string]string{},
			})},
			expectedError: "unable to mark event as deleted: invalid Metadata.Events format",
		},
		{
			desc: "trying to delete an event with empty ID produces an error",
			event: model.EventCloseInput{
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
			event: model.EventCloseInput{
				GremlinQuery: "",
				ID:           "Prefix_someId",
			},
			expectedError: "parsing gremlin query: found \"\", expected `G`",
		},
		{
			desc: "deleting an event",
			event: model.EventCloseInput{
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
			event: model.EventCloseInput{
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
				err := g.AddNode(n)
				assert.NoError(t, err)
			}

			r := Resolver{Graph: g}

			n, err := r.closeEvent(tC.event)
			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			} else {
				assert.Equal(t, graph.Identifier("a"), n.ID)
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
		event                model.EventAddInput
		nodes                []*graph.Node
		expectedNodeMetadata graph.Metadata
		expectedError        string
		childChain           []string
		childID              graph.Identifier
	}{
		{
			desc: "adding an event with empty query produces an error",
			event: model.EventAddInput{
				GremlinQuery: "",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "parsing gremlin query: found \"\", expected `G`",
		},
		{
			desc: "adding an event with empty ID produces an error",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "ID should not be empty",
		},
		{
			desc: "adding an event to a non existant node produces an error",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','z')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
				TTL:          &ttl200,
			},
			nodes:         []*graph.Node{createNode("a", graph.Metadata{"Name": "a", "Type": "host"})},
			expectedError: "expecting only one node, gremlin query returned 0",
		},
		{
			desc: "adding an event to a node without Metadata.Event",
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
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
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelInfo,
				Payload:      []*model.EventKeyValue{{Key: "foo", Value: "bar"}},
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
			event: model.EventAddInput{
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
			event: model.EventAddInput{
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
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_someId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.EventKeyValue{{Key: "new", Value: "value"}},
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
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.EventKeyValue{{Key: "new", Value: "value"}},
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
			event: model.EventAddInput{
				GremlinQuery: "G.V().Has('Name','a')",
				ID:           "Prefix_otherId",
				Level:        model.EventLevelWarning,
				Payload:      []*model.EventKeyValue{{Key: "new", Value: "value"}},
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
				err := g.AddNode(n)
				assert.NoError(t, err)
			}

			r := Resolver{Graph: g}

			_, err := r.createEvent(
				tC.event.ID,
				tC.event.GremlinQuery,
				tC.event.Level,
				tC.event.TTL,
				tC.event.Payload,
				createEventTime,
				false,
				[]string{},
				"",
			)
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

func TestCreateEventWithPropagation(t *testing.T) {
	g := newGraph(t)

	deviceNode := createNode("device1ID", graph.Metadata{
		"Name": "device1",
		"Type": "device",
	})

	buildingNode := createNode("building1ID", graph.Metadata{
		"Name": "building1",
		"Type": "Building",
	})

	provinceNode := createNode("province1ID", graph.Metadata{
		"Name": "province1",
		"Type": "Province",
	})

	err := g.AddNode(deviceNode)
	assert.NoError(t, err)
	err = g.AddNode(buildingNode)
	assert.NoError(t, err)
	err = g.AddNode(provinceNode)
	assert.NoError(t, err)

	// Building owns device
	eBuildingNode := graph.CreateEdge(
		graph.GenID(),
		buildingNode,
		deviceNode,
		graph.Metadata{"RelationType": "ownership"},
		graph.Time(time.Now()),
		"host",
		"origin",
	)

	// Province owns building
	eProvinceBuilding := graph.CreateEdge(
		graph.GenID(),
		provinceNode,
		buildingNode,
		graph.Metadata{"RelationType": "ownership"},
		graph.Time(time.Now()),
		"host",
		"origin",
	)

	err = g.AddEdge(eBuildingNode)
	assert.NoError(t, err)
	err = g.AddEdge(eProvinceBuilding)
	assert.NoError(t, err)

	r := Resolver{Graph: g}
	_, err = r.createEvent(
		"foobar",
		"G.V().Has('Name', 'device1')",
		model.EventLevelWarning,
		nil,
		[]*model.EventKeyValue{{Key: "new", Value: "value"}},
		time.Unix(0, 0),
		true,
		[]string{},
		"",
	)
	assert.NoError(t, err)

	// Check the event has been created in the device node and propagated
	// to the building and province node
	expectedDeviceEvents := &Events{
		"foobar": Event{
			TTL:       0,
			Level:     "WARNING",
			Payload:   []PayloadValues{{Key: "new", Value: "value"}},
			CreatedAt: graph.Time(time.Unix(0, 0)),
			UpdatedAt: graph.Time(time.Unix(0, 0)),
		},
	}

	expectedBuildingEvents := &Events{
		"foobar": Event{
			TTL:       0,
			Level:     "WARNING",
			Payload:   []PayloadValues{{Key: "new", Value: "value"}},
			CreatedAt: graph.Time(time.Unix(0, 0)),
			UpdatedAt: graph.Time(time.Unix(0, 0)),
			ChildChain: []string{
				"device1",
			},
			ChildID: "device1ID",
		},
	}

	expectedProvinceEvents := &Events{
		"foobar": Event{
			TTL:       0,
			Level:     "WARNING",
			Payload:   []PayloadValues{{Key: "new", Value: "value"}},
			CreatedAt: graph.Time(time.Unix(0, 0)),
			UpdatedAt: graph.Time(time.Unix(0, 0)),
			ChildChain: []string{
				"device1",
				"building1",
			},
			ChildID: "device1ID",
		},
	}

	assert.Equal(t, expectedDeviceEvents, deviceNode.Metadata["Events"])
	assert.Equal(t, expectedBuildingEvents, buildingNode.Metadata["Events"])
	assert.Equal(t, expectedProvinceEvents, provinceNode.Metadata["Events"])
}

func TestDeleteEventWithPropagation(t *testing.T) {
	g := newGraph(t)

	deviceNode := createNode("device1ID", graph.Metadata{
		"Name": "device1",
		"Type": "device",
	})

	buildingNode := createNode("building1ID", graph.Metadata{
		"Name": "building1",
		"Type": "Building",
	})

	provinceNode := createNode("province1ID", graph.Metadata{
		"Name": "province1",
		"Type": "Province",
	})

	err := g.AddNode(deviceNode)
	assert.NoError(t, err)
	err = g.AddNode(buildingNode)
	assert.NoError(t, err)
	err = g.AddNode(provinceNode)
	assert.NoError(t, err)

	// Building owns device
	eBuildingNode := graph.CreateEdge(
		graph.GenID(),
		buildingNode,
		deviceNode,
		graph.Metadata{"RelationType": "ownership"},
		graph.Time(time.Now()),
		"host",
		"origin",
	)

	// Province owns building
	eProvinceBuilding := graph.CreateEdge(
		graph.GenID(),
		provinceNode,
		buildingNode,
		graph.Metadata{"RelationType": "ownership"},
		graph.Time(time.Now()),
		"host",
		"origin",
	)

	err = g.AddEdge(eBuildingNode)
	assert.NoError(t, err)
	err = g.AddEdge(eProvinceBuilding)
	assert.NoError(t, err)

	r := Resolver{Graph: g}
	_, err = r.createEvent(
		"foobar",
		"G.V().Has('Name', 'device1')",
		model.EventLevelWarning,
		nil,
		[]*model.EventKeyValue{{Key: "new", Value: "value"}},
		time.Unix(0, 0),
		true,
		[]string{},
		"",
	)
	assert.NoError(t, err)

	// Tras propagar el evento a los parents, forzamos un cerrado del evento y comprobamos
	// que los parents, ni el nodo, tienen el evento

	propagate := true
	r = Resolver{Graph: g}
	_, err = r.closeEvent(model.EventCloseInput{
		GremlinQuery: "G.V().Has('Name', 'device1')",
		ID:           "foobar",
		Propagate:    &propagate,
	})
	assert.NoError(t, err)

	// Check the event has been deleted from parent nodes
	assert.Equal(t, &Events{}, deviceNode.Metadata["Events"])
	assert.Equal(t, &Events{}, buildingNode.Metadata["Events"])
	assert.Equal(t, &Events{}, provinceNode.Metadata["Events"])
}

// TestDeleteEventWithPropagationWhenChildNodeIsDeleted comprobamos que si se borra el nodo que tiene un evento,
// se borran todos sus eventos propagados
func TestDeleteEventWithPropagationWhenChildNodeIsDeleted(t *testing.T) {
	g := newGraph(t)

	deviceNode := createNode("device1ID", graph.Metadata{
		"Name": "device1",
		"Type": "device",
	})

	buildingNode := createNode("building1ID", graph.Metadata{
		"Name": "building1",
		"Type": "Building",
	})

	provinceNode := createNode("province1ID", graph.Metadata{
		"Name": "province1",
		"Type": "Province",
	})

	err := g.AddNode(deviceNode)
	assert.NoError(t, err)
	err = g.AddNode(buildingNode)
	assert.NoError(t, err)
	err = g.AddNode(provinceNode)
	assert.NoError(t, err)

	// Building owns device
	eBuildingNode := graph.CreateEdge(
		graph.GenID(),
		buildingNode,
		deviceNode,
		graph.Metadata{"RelationType": "ownership"},
		graph.Time(time.Now()),
		"host",
		"origin",
	)

	// Province owns building
	eProvinceBuilding := graph.CreateEdge(
		graph.GenID(),
		provinceNode,
		buildingNode,
		graph.Metadata{"RelationType": "ownership"},
		graph.Time(time.Now()),
		"host",
		"origin",
	)

	err = g.AddEdge(eBuildingNode)
	assert.NoError(t, err)
	err = g.AddEdge(eProvinceBuilding)
	assert.NoError(t, err)

	grapgqlProbe, err := NewProbe(g)
	assert.NoError(t, err)
	grapgqlProbe.Start()

	r := Resolver{Graph: g}
	_, err = r.createEvent(
		"foobar",
		"G.V().Has('Name', 'device1')",
		model.EventLevelWarning,
		nil,
		[]*model.EventKeyValue{{Key: "new", Value: "value"}},
		time.Unix(0, 0),
		true,
		[]string{},
		"",
	)
	assert.NoError(t, err)

	// Tras propagar el evento a los parents, borramos el nodo que tenía el evento.
	err = g.DelNode(deviceNode)
	assert.NoError(t, err)

	// Check the event has been deleted from parent nodes
	events, ok := buildingNode.Metadata["Events"].(*Events)
	assert.True(t, ok)
	assert.Truef(t, len(*events) == 0, "building should have no events")

	events, ok = provinceNode.Metadata["Events"].(*Events)
	assert.True(t, ok)
	assert.Truef(t, len(*events) == 0, "province should have no events")
}
