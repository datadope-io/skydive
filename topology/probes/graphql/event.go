package graphql

import (
	"errors"
	"fmt"
	"time"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
)

const (
	// MetaKeyEvents key name where all events are stored inside of Metadata
	MetaKeyEvents = "Events"
	// SecondToMilli to convert seconds to milliseconds
	SecondToMilli = 1000
)

// ErrEventNotFound is returned when an event is not found
var ErrEventNotFound = errors.New("event not found")

// createEvent add, or update, an event to the node selected by the "gremlinQuery", using
// the parameter "t" as the event CreatedAt and UpdatedAt values (except for updates, where
// CreatedAt remains the original one).
// "id" is the event identifier, to be able to modify/update a previous sent event.
// "level" is the criticity of the event.
// "ttl" is the "time to live" of the event, when it should be automatically deleted, if defined.
// "payload" is extra metadata added by the user to identify the event.
// "propagate" is a boolean to indicate if the event should be copied to all node parents.
// "childChain" contains the list of nodes names to reach the node originator of the event.
// If it is empty, the event is original one.
// "childID" is the identifier of the node originator of the event.
func (r *Resolver) createEvent(
	id string,
	gremlinQuery string,
	level model.EventLevel,
	ttl *int,
	payload []*model.EventKeyValue,
	t time.Time,
	propagate bool,
	childChain []string,
	childID graph.Identifier,
) (gid graph.Identifier, err error) {
	if id == "" {
		return gid, fmt.Errorf("ID should not be empty")
	}

	r.Graph.Lock()
	node, err := r.getNodeFromGremlinQuery(gremlinQuery)
	if err != nil {
		return gid, err
	}

	// Parse the gremlin query
	// Map the GraphQL event data, to the data stored in Metadata.Events
	e := Event{
		Level:     level.String(),
		CreatedAt: graph.Time(t),
		UpdatedAt: graph.Time(t),
	}

	// Set TTL if defined
	if ttl != nil {
		e.TTL = int64(*ttl)
	}

	// Set payload if defined
	payloadValues := make([]PayloadValues, 0, len(payload))
	for _, v := range payload {
		payloadValues = append(payloadValues, PayloadValues{
			Key:   v.Key,
			Value: v.Value,
		})
	}
	e.Payload = payloadValues

	// If we are propagating the event and this node if one of its parents,
	// add the child info.
	if propagate && len(childChain) > 0 {
		e.ChildChain = childChain
		e.ChildID = childID
	}

	// Save errors produced inside of the updating function
	var updateErr error

	// Add the event to that node obtained from the gremlin query.
	// All events are stored into Metadata.Events
	// Previous event is substituted by the new one, except CreatedAt value, that remains.
	err = r.Graph.UpdateMetadata(node, MetaKeyEvents, func(field interface{}) bool {
		nodeEvents, ok := field.(*Events)
		if !ok {
			logging.GetLogger().Warningf("Unable to convert Metadata.%s (type %T) to *Events", MetaKeyEvents, field)
			updateErr = fmt.Errorf("invalid Metadata.%s format", MetaKeyEvents)
			return false
		}

		if nodeEvent, eventExists := (*nodeEvents)[id]; eventExists {
			// Event already exists in the node metadata
			e.CreatedAt = nodeEvent.CreatedAt
		}
		(*nodeEvents)[id] = e
		return true
	})
	if err == getter.ErrFieldNotFound {
		// Metadata.Events does not exists. Create it.
		events := Events{id: e}
		err = r.Graph.AddMetadata(node, MetaKeyEvents, &events)
		if err != nil {
			return node.ID, fmt.Errorf("adding new Events key in Metadata")
		}
	} else if err != nil {
		return node.ID, fmt.Errorf("unable to add event: %v", err)
	} else if updateErr != nil {
		return node.ID, fmt.Errorf("unable to add event: %v", updateErr)
	}
	// Release lock before a new createEvent call could be done with propagation
	r.Graph.Unlock()

	// Propagate event to all parents
	if propagate {
		// Get the node parents
		parents := r.Graph.LookupParents(node, nil, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership})
		for _, parent := range parents {
			// Create the event in the parent node
			nodeName, err := node.Metadata.GetFieldString(MetaKeyName)
			if err != nil {
				return node.ID, fmt.Errorf("unable to get node name: %v", err)
			}

			// Create the chain of nodes names to reach the node originator of the event
			cc := append(childChain, nodeName)

			// Set the originator node ID, if not already set
			if childID == "" {
				childID = node.ID
			}

			_, err = r.createEvent(id, fmt.Sprintf("G.V('%v')", string(parent.ID)), level, ttl, payload, t, true, cc, childID)
			if err != nil {
				return node.ID, fmt.Errorf("unable to create event in parent node: %v", err)
			}
		}
	}

	return node.ID, nil
}

// closeEvent remove an event from a node specified by a gremlin query.
// Return node selected by the input gremlin query.
func (r *Resolver) closeEvent(event model.EventCloseInput) (node *graph.Node, err error) {
	if event.ID == "" {
		return node, fmt.Errorf("ID should not be empty")
	}

	node, err = r.getNodeFromGremlinQuery(event.GremlinQuery)
	if err != nil {
		return node, err
	}

	var updateErr error

	// Mark the event as deleted. This archives the node containing the event with the DeletedAt field unset.
	// And index in the new node with the event with DeletedAt defined.
	err = r.Graph.UpdateMetadata(node, MetaKeyEvents, func(field interface{}) bool {
		nodeEvents, ok := field.(*Events)
		if !ok {
			logging.GetLogger().Warningf("Unable to convert Metadata.%s (type %T) to *Events", MetaKeyEvents, field)
			updateErr = fmt.Errorf("invalid Metadata.%s format", MetaKeyEvents)
			return false
		}

		if storedEvent, eventExists := (*nodeEvents)[event.ID]; !eventExists {
			// event does not exists in the node
			updateErr = fmt.Errorf("event ID '%s' in node %v: %w", event.ID, node.ID, ErrEventNotFound)
			return false
		} else {
			storedEvent.DeletedAt = graph.Time(time.Now())
			(*nodeEvents)[event.ID] = storedEvent
		}
		return true
	})
	if err != nil {
		return node, fmt.Errorf("unable to mark event as deleted: %w", err)
	}
	if updateErr != nil {
		return node, fmt.Errorf("unable to mark event as deleted: %w", updateErr)
	}

	// Delete the event from the metadata.
	// The node archived contains the event with DeletedAt set.
	err = r.Graph.UpdateMetadata(node, MetaKeyEvents, func(field interface{}) bool {
		nodeEvents, ok := field.(*Events)
		if !ok {
			logging.GetLogger().Warningf("Unable to convert Metadata.%s (type %T) to *Events", MetaKeyEvents, field)
			updateErr = fmt.Errorf("invalid Metadata.%s format", MetaKeyEvents)
			return false
		}

		if _, eventExists := (*nodeEvents)[event.ID]; !eventExists {
			// event does not exists in the node
			updateErr = fmt.Errorf("event ID '%s' in node %v: %w", event.ID, node.ID, ErrEventNotFound)
			return false
		}
		delete(*nodeEvents, event.ID)
		return true
	})
	if err != nil {
		return node, fmt.Errorf("unable to delete event: %w", err)
	}
	if updateErr != nil {
		return node, fmt.Errorf("unable to delete event: %w", updateErr)
	}

	// Propagate close event to all parents
	// Will not be executed if there was an error processing the node (eg.: event does not exists)
	if event.Propagate != nil && *event.Propagate {
		// Get the node parents
		parents := r.Graph.LookupParents(node, nil, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership})
		for _, parent := range parents {
			// Close the event in the parent node
			propagate := true
			query := fmt.Sprintf("G.V('%v')", string(parent.ID))
			_, err = r.closeEvent(model.EventCloseInput{ID: event.ID, GremlinQuery: query, Propagate: &propagate})
			if err != nil {
				return node, fmt.Errorf("unable to close event in parent node: %v", err)
			}
		}
	}

	return node, nil
}

// eventGarbageCollector executes periodically functions to clean old events.
// "interval" is how often will be executed the garbageCollector.
func (p *Probe) eventGarbageCollector(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.EventGCdone:
			logging.GetLogger().Debug("Closing event garbageCollector")
			return
		case <-ticker.C:
			logging.GetLogger().Debug("Executing event garbageCollector")
			p.cleanNodesEvents(time.Now())
			logging.GetLogger().Debug("End event garbageCollector")
		}
	}
}

// cleanNodesEvents remove old events from nodes with defined Metadata.Events key older
// than "expiredEventThreshold"
func (p *Probe) cleanNodesEvents(expiredEventThreshold time.Time) {
	// To avoid a huge lock between getting all the nodes and processing them, we do:
	//   - lock
	//   - get nodes with events
	//   - unlock
	//   - loop for each node
	//   - lock
	//   - get the node again
	//   - delete expired events
	//   - unlock
	// Doing this sequence we avoid the problem of some node modified between getting
	// the list of nodes and processing each particular node

	// Get nodes with Metadata.Events defined
	nodeFilter := graph.NewElementFilter(filters.NewNotNullFilter(MetaKeyEvents))
	p.graph.RLock()
	nodes := p.graph.GetNodes(nodeFilter)
	p.graph.RUnlock()

	logging.GetLogger().Debugf("Event GarbageCollector, deleting events older than %v. Processing %v nodes", expiredEventThreshold, len(nodes))

	for _, n := range nodes {
		err := p.deleteExpiredEvents(n.ID, expiredEventThreshold)
		if err != nil {
			logging.GetLogger().Warningf("Deleting old event on node %+v: %v", n, err)
		}
	}
}

// deleteExpiredEvents remove expired events for a particular node.
// Events are removed if they CreatedAt + TTL is less than "expiredEventThreshold", it's live time has end.
// Only events with TTL higher than 0 will be processed.
func (p *Probe) deleteExpiredEvents(nodeID graph.Identifier, thresholdTime time.Time) error {
	// Save errors produced inside of the updating function
	var removeErr error

	tt := graph.Time(thresholdTime)

	// Function to process Metadata.Events and mark expired events as deleted
	markAsDeletedExpired := func(field interface{}) (ret bool) {
		nodeEvents, ok := field.(*Events)
		if !ok {
			removeErr = fmt.Errorf("invalid Metadata.%s format", MetaKeyEvents)
			return false
		}

		for k, v := range *nodeEvents {
			if v.TTL > 0 {
				// Usamos UpdatedAt por si llegan más eventos del mismo ID. Queremos que no se cierre
				// hasta que dejen de llegar.
				if v.UpdatedAt.UnixMilli()+v.TTL*SecondToMilli < tt.UnixMilli() {
					logging.GetLogger().Debugf("Deleted expired event %s from node %s", k, nodeID)
					v.DeletedAt = graph.Time(time.Now())
					(*nodeEvents)[k] = v
					ret = true
				}
			}
		}
		return ret
	}

	// Function to process Metadata.Events and delete expired events
	removeExpired := func(field interface{}) (ret bool) {
		nodeEvents, ok := field.(*Events)
		if !ok {
			removeErr = fmt.Errorf("invalid Metadata.%s format", MetaKeyEvents)
			return false
		}

		for k, v := range *nodeEvents {
			if v.TTL > 0 {
				if v.UpdatedAt.UnixMilli()+v.TTL*SecondToMilli < tt.UnixMilli() {
					logging.GetLogger().Debugf("Deleted expired event %s from node %s", k, nodeID)
					delete(*nodeEvents, k)
					// TODO en vez de ese delete llamar a la función que cierra eventos que tambien gestiona propagados
					ret = true
				}
			}
		}
		return ret
	}

	// Get node metadata and process its events
	p.graph.Lock()
	defer p.graph.Unlock()
	node := p.graph.GetNode(nodeID)

	err := p.graph.UpdateMetadata(node, MetaKeyEvents, markAsDeletedExpired)
	if err != nil {
		if errors.Is(err, getter.ErrFieldNotFound) {
			// It is normal not to have defined that metadata key
			return nil
		}
		return fmt.Errorf("unable to mark expired events: %v", err)
	}
	if removeErr != nil {
		return fmt.Errorf("unable to mark expired events: %v", removeErr)
	}

	err = p.graph.UpdateMetadata(node, MetaKeyEvents, removeExpired)
	if err != nil {
		if errors.Is(err, getter.ErrFieldNotFound) {
			// It is normal not to have defined that metadata key
			return nil
		}
		return fmt.Errorf("unable to delete expired events: %v", err)
	}
	if removeErr != nil {
		return fmt.Errorf("unable to delete expired events: %v", removeErr)
	}
	return nil
}

// handleDeletedEdgeWithChildEvents given a removed edge and its child node, remove the events in the parent nodes
func (p *Probe) handleDeletedEdgeWithChildEvents(childNode *graph.Node, edge *graph.Edge) error {
	p.graph.Lock()
	defer p.graph.Unlock()
	propagate := true

	// Iterate over all the events and delete them
	eventsRaw, err := childNode.GetField(MetaKeyEvents)
	if err != nil {
		return err
	}

	nodeEvents, ok := eventsRaw.(*Events)
	if !ok {
		logging.GetLogger().Warningf("Unable to convert Metadata.%s (type %T) to *Events", MetaKeyEvents, eventsRaw)
		return fmt.Errorf("invalid Metadata.%s format", MetaKeyEvents)
	}

	// For each child event, try to delete it in the parent node and its parents
	for id := range *nodeEvents {
		_, err := p.resolver.closeEvent(model.EventCloseInput{
			ID:           id,
			GremlinQuery: fmt.Sprintf("G.V('%s')", edge.Parent),
			Propagate:    &propagate,
		})
		if errors.Is(err, ErrEventNotFound) {
			// Ignore if event was not found in the parent node, could be an event not propagated
			continue
		}
		if err != nil {
			return fmt.Errorf("unable to close event %s: %v", id, err)
		}
	}
	return nil
}

// StartEvents initialice event garbage collector
func (p *Probe) StartEvents() {
	p.EventGCdone = make(chan bool)

	intervalConfig := config.GetString("analyzer.topology.graphql.event.garbage_collector_interval")
	interval, err := time.ParseDuration(intervalConfig)
	if err != nil {
		logging.GetLogger().Fatalf("Invalid analyzer.topology.graphql.event.garbage_collector_interval value: %v", err)
	}

	go p.eventGarbageCollector(interval)
}

// StopEvents stop event garbage collector
func (p *Probe) StopEvents() {
	// Stop the GarbageCollector
	p.EventGCdone <- true
}
