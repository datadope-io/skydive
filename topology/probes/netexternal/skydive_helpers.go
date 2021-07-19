package netexternal

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/netexternal/model"
)

const (
	// PrefixOriginName defines the prefix set in nodes/edges Origin field
	PrefixOriginName = "netexternal."

	TypeRouter    = "router"
	TypeSwitch    = "switch"
	TypeInterface = "interface"
	TypeServer    = "server"

	MetaKeyRelationType   = "RelationType"
	RelationTypeOwnership = "ownership"
	RelationConnectsTo    = "ConnectsTo"

	MetaKeyType        = "Type"
	MetaKeyName        = "Name"
	MetaKeyVendor      = "Vendor"
	MetaKeyModel       = "Model"
	MetaKeyAggregation = "Aggregation"
	MetaKeyEvents      = "Events"
)

// newEdge create a new edge in Skydive with a custom Origin field
func (r *Resolver) newEdge(n *graph.Node, c *graph.Node, m graph.Metadata, createdAt *time.Time) (*graph.Edge, error) {
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(n.ID+c.ID))
	i := graph.Identifier(u.String())

	var timestamp graph.Time
	if createdAt != nil {
		timestamp = graph.Time(*createdAt)
	} else {
		timestamp = graph.TimeUTC()
	}
	e := graph.CreateEdge(i, n, c, m, timestamp, r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// newNode create a new edge in Skydive with a custom Origin field
func (r *Resolver) newNode(id graph.Identifier, m graph.Metadata, createdAt *time.Time) (*graph.Node, error) {
	var timestamp graph.Time
	if createdAt != nil {
		timestamp = graph.Time(*createdAt)
	} else {
		timestamp = graph.TimeUTC()
	}
	n := graph.CreateNode(id, m, timestamp, r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

// getNodeFromGremlinQuery return a single node from a gremlin query
func (r *Resolver) getNodeFromGremlinQuery(query string) (*graph.Node, error) {
	tr := traversal.NewGremlinTraversalParser()
	ts, err := tr.Parse(strings.NewReader(query))
	if err != nil {
		return nil, fmt.Errorf("parsing gremlin query: %v", err)
	}

	// Execute the gremlin query
	res, err := ts.Exec(r.Graph, false)
	if err != nil {
		return nil, fmt.Errorf("executing gremlin query: %v", err)
	}

	// Assume the gremlin query is outputting a list of nodes Fail otherwise
	tv, ok := res.(*traversal.GraphTraversalV)
	if !ok {
		return nil, fmt.Errorf("invalid graph output, expecting nodes")
	}

	// Only one node is expected
	nodes := tv.GetNodes()
	if len(nodes) != 1 {
		return nil, fmt.Errorf("expecting only one node, gremlin query returned %d", len(nodes))
	}

	return nodes[0], nil
}

// closeEvent remove an event from a node specified by a gremlin query
func (r *Resolver) closeEvent(event model.CloseEventInput) error {
	node, err := r.getNodeFromGremlinQuery(event.GremlinQuery)
	if err != nil {
		return err
	}

	var updateErr error

	err = r.Graph.UpdateMetadata(node, MetaKeyEvents, func(field interface{}) bool {
		nodeEvents, ok := field.(*Events)
		if !ok {
			logging.GetLogger().Warningf("Unable to convert Metadata.%s (type %T) to *Events", MetaKeyEvents, field)
			updateErr = fmt.Errorf("invalid Metadata.%s format", MetaKeyEvents)
			return false
		}

		if _, eventExists := (*nodeEvents)[event.ID]; !eventExists {
			// event does not exists in the node
			updateErr = fmt.Errorf("event ID '%s' does not exists in node %v", event.ID, node.ID)
			return false
		}
		delete(*nodeEvents, event.ID)
		return true
	})
	if err != nil {
		fmt.Errorf("unable to delete event: %v", err)
	}
	if updateErr != nil {
		fmt.Errorf("unable to delete event: %v", updateErr)
	}

	return nil
}

// createEvent exec the gremlin query to get the node where the event should be added
func (r *Resolver) createEvent(event *model.AddEventInput) error {
	node, err := r.getNodeFromGremlinQuery(event.GremlinQuery)
	if err != nil {
		return err
	}

	// Parse the gremlin query
	// Map the GraphQL event data, to the data stored in Metadata.Events
	e := Event{
		Level:     event.Level.String(),
		CreatedAt: graph.Time(time.Now()),
		UpdatedAt: graph.Time(time.Now()),
	}

	// Set TTL if defined
	if event.TTL != nil {
		e.TTL = int64(*event.TTL)
	}

	// Set payload if defined
	if len(event.Payload) > 0 {
		payloadValues := make([]PayloadValues, 0, len(event.Payload))
		for _, v := range event.Payload {
			payloadValues = append(payloadValues, PayloadValues{
				Key:   v.Key,
				Value: v.Value,
			})
		}
		e.Payload = payloadValues
	}

	// Add the event to that node obtained from the gremlin query.
	// All events are stored into Metadata.Events
	// Previous event is substituted by the new one, except CreatedAt value, that remains.
	err = r.Graph.UpdateMetadata(node, MetaKeyEvents, func(field interface{}) bool {
		nodeEvents, ok := field.(*Events)
		if !ok {
			logging.GetLogger().Warningf("Unable to convert Metadata.%s (type %T) to *Events", MetaKeyEvents, field)
			return false
		}

		if nodeEvent, eventExists := (*nodeEvents)[event.ID]; eventExists {
			// Event already exists in the node metadata
			e.CreatedAt = nodeEvent.CreatedAt
		}
		(*nodeEvents)[event.ID] = e
		return true
	})
	if err == getter.ErrFieldNotFound {
		// Metadata.Events does not exists. Create it.
		events := Events{event.ID: e}
		err = r.Graph.AddMetadata(node, MetaKeyEvents, &events)
		if err != nil {
			return fmt.Errorf("adding new Events key in Metadata")
		}
	} else if err != nil {
		return fmt.Errorf("unable to add event: %v", err)
	}

	return nil
}

func (r *Resolver) createInterfaces(
	device *graph.Node,
	interfaces []*model.InterfaceInput,
	createdAt *time.Time,
) (updated bool, err error) {
	// Iterate over user defined interfaces, creating or updating while needed
	for _, iface := range interfaces {
		nodeName := device.Metadata[MetaKeyName]

		ifaceMetadata := map[string]interface{}{
			MetaKeyName: iface.Name,
			"Device":    nodeName,
			MetaKeyType: "interface",
		}

		// Generate ID: sha256("device__ifName")
		s := fmt.Sprintf("%s__%s", nodeName, iface.Name)
		ifID := str2GraphID(s)

		ifNode := r.Graph.GetNode(ifID)
		if ifNode == nil {
			// Create the interface and assign the node to "iface" var
			ifNode, err = r.newNode(ifID, ifaceMetadata, createdAt)
			if err != nil {
				logging.GetLogger().Errorf("unable to create interface %+v: %v", ifaceMetadata, err)
				return updated, fmt.Errorf("unable to create interface: %v", err)
			}
		} else {
			revisionPreUpdateIface := ifNode.Revision
			errS := r.Graph.SetMetadata(ifNode, ifaceMetadata)
			if errS != nil {
				logging.GetLogger().Errorf("unable to update interface metadata %+v: %v", ifNode, err)
				return updated, fmt.Errorf("unable to update interface metadata %+v: %v", ifNode, err)
			}
			if revisionPreUpdateIface != ifNode.Revision {
				updated = true
			}
		}

		if iface.Aggregation != nil && *iface.Aggregation != "" {
			aggregation := *iface.Aggregation
			ifaceMetadata[MetaKeyAggregation] = aggregation
			r.createAggrIface(aggregation, device, ifNode, createdAt)
		} else {
			aggregation := iface.Name + "__NoAggregation"
			ifaceMetadata[MetaKeyAggregation] = aggregation
			r.createAggrIface(aggregation, device, ifNode, createdAt)
		}
	}

	return updated, nil
}

func (r *Resolver) createAggrIface(
	aggregation string,
	device *graph.Node,
	ifNode *graph.Node,
	createdAt *time.Time,
) error {
	var err error
	nodeName := device.Metadata[MetaKeyName]

	aggMetadata := map[string]interface{}{
		MetaKeyName: aggregation,
		"Device":    nodeName,
		MetaKeyType: "aggregation",
	}

	s := fmt.Sprintf("%s__%s", nodeName, aggregation)
	aggID := str2GraphID(s)

	aggNode := r.Graph.GetNode(aggID)
	if aggNode == nil {
		// Create the interface and assign the node to "iface" var
		aggNode, err = r.newNode(aggID, aggMetadata, createdAt)
		if err != nil {
			fmt.Printf("unable to create aggregate interface %+v: %v\n", aggMetadata, err)
			return fmt.Errorf("unable to create aggregate interface: %v", err)
		}
	}

	// Link device with aggregate interface
	if !r.Graph.AreLinked(device, aggNode, nil) {
		_, err = r.newEdge(device, aggNode, map[string]interface{}{
			MetaKeyRelationType: RelationTypeOwnership,
		}, createdAt)
		if err != nil {
			fmt.Printf("unable to link device (%+v) to aggregation (%+v): %v\n", device, aggNode, err)
			return fmt.Errorf("unable to link device (%+v) to aggregation (%+v): %v", device, aggNode, err)
		}
	}

	// Link interface with aggregate interface
	if !r.Graph.AreLinked(aggNode, ifNode, nil) {
		_, err = r.newEdge(aggNode, ifNode, map[string]interface{}{
			MetaKeyRelationType: RelationTypeOwnership,
		}, createdAt)
		if err != nil {
			fmt.Printf("unable to link interface (%+v) to aggregation (%+v): %v\n", ifNode, aggNode, err)
			return fmt.Errorf("unable to link interface (%+v) to aggregation (%+v): %v", ifNode, aggNode, err)
		}
	}

	return nil
}

func (r *Resolver) createIf2IfEdge(
	srcDevice string,
	srcInterface string,
	dstDevice string,
	dstInterface string,
	createdAt *time.Time,
) (edge *graph.Edge, err error) {
	// Get source node
	s := fmt.Sprintf("%s__%s", srcDevice, srcInterface)
	srcNode := r.Graph.GetNode(str2GraphID(s))

	// Get destination node
	s = fmt.Sprintf("%s__%s", dstDevice, dstInterface)
	dstNode := r.Graph.GetNode(str2GraphID(s))

	edge, err = r.newEdge(srcNode, dstNode, map[string]interface{}{
		MetaKeyRelationType: RelationConnectsTo,
	}, createdAt)
	if err != nil {
		return nil, fmt.Errorf("unable to link (%+v) with (%+v): %v", srcNode, dstNode, err)
	}

	return edge, nil
}

func str2GraphID(input string) graph.Identifier {
	hash := sha256.Sum256([]byte(input))
	return graph.Identifier(fmt.Sprintf("%x", hash))
}

func (r *Resolver) addDeviceWithInterfaces(
	name string,
	metadata graph.Metadata,
	interfaces []*model.InterfaceInput,
	createdAt *time.Time,
) (node *graph.Node, updated bool, interfaceUpdated bool, err error) {

	id := str2GraphID(name)
	node = r.Graph.GetNode(id)

	// Create, if it does not exists.
	// Return internal ID if it exists.
	if node == nil {
		node, err = r.newNode(id, metadata, createdAt)
		if err != nil {
			return node, updated, interfaceUpdated, err
		}
	} else {
		// Update metadata if needed
		revisionPreUpdate := node.Revision

		err = r.Graph.SetMetadata(node, metadata)
		if err != nil {
			logging.GetLogger().Errorf("unable to update node %+v: %v", node, err)
			return node, updated, interfaceUpdated, fmt.Errorf("error updating node: %v", err)
		}
		// If SetMetadata modifies metadata, it will increase revision number.
		// We use this to know if switch metadata has been modified and notify the user.
		if revisionPreUpdate != node.Revision {
			updated = true
		}
	}

	// Interfaces
	interfaceUpdated, err = r.createInterfaces(node, interfaces, createdAt)
	if err != nil {
		logging.GetLogger().Errorf("creating interfaces for node %+v: %v", node, err)
		return nil, false, false, fmt.Errorf("creating interfaces: %v", err)
	}

	return node, updated, interfaceUpdated, nil
}
