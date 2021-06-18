package traversal

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// EventsTraversalExtension describes a new extension to enhance the topology
type EventsTraversalExtension struct {
	EventsToken traversal.Token
}

// EventsGremlinTraversalStep describes the Events gremlin traversal step
type EventsGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
	EventKey    string
	EventAggKey string
}

// NewEventsTraversalExtension returns a new graph traversal extension
func NewEventsTraversalExtension() *EventsTraversalExtension {
	return &EventsTraversalExtension{
		EventsToken: traversalEventsToken,
	}
}

// ScanIdent recognise the word associated with this step (in uppercase) and return a token
// which represents it. Return true if it have found a match
func (e *EventsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "EVENTS":
		return e.EventsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep generate a step for a given token, having in 'p' context and params
func (e *EventsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.EventsToken:
	default:
		return nil, nil
	}

	var eventKey, eventAggKey string
	var ok bool

	switch len(p.Params) {
	case 2:
		eventKey, ok = p.Params[0].(string)
		if !ok {
			return nil, errors.New("Events first parameter have to be a string")
		}
		eventAggKey, ok = p.Params[1].(string)
		if !ok {
			return nil, errors.New("Events second parameter have to be a string")
		}
	default:
		return nil, errors.New("Events parameter must have two parameters")
	}

	return &EventsGremlinTraversalStep{
		GremlinTraversalContext: p,
		EventKey:                eventKey,
		EventAggKey:             eventAggKey,
	}, nil
}

// Exec executes the events step
func (s *EventsGremlinTraversalStep) Exec(ctx context.Context, last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		return InterfaceEvents(ctx, s.StepContext, tv, s.EventKey, s.EventAggKey), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce events step
func (s *EventsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context events step
func (s *EventsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &s.GremlinTraversalContext
}

// InterfaceEvents for each node id, group all the events stored in Metadata.key from the
// input nodes and put them into the newest node for each id into Metadata.aggKey.
// Events are groupped based on its key. See mergedEvents for an example.
// All output nodes will have Metadata.aggKey defined (empty or not).
func InterfaceEvents(gCtx context.Context, ctx traversal.StepContext, tv *traversal.GraphTraversalV, key, aggKey string) traversal.GraphTraversalStep {
	it := ctx.PaginationRange.Iterator()
	//gslice := tv.GraphTraversal.Graph.GetContext().TimeSlice

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	uniqNodes := map[graph.Identifier]*graph.Node{}
	// events accumulate the events for each node id
	events := map[graph.Identifier]map[string][]interface{}{}

	for _, node := range tv.GetNodes() {
		// TODO afecta la paginación a esta función? Cuando se usa esta paginación?
		if it.Done() {
			break
		}

		// Get all revisions for this node
		revisionNodes := tv.GraphTraversal.Graph.GetNodeAll(gCtx, node.ID)

		// Store only the most recent nodes
		for _, rNode := range revisionNodes {
			storedNode, ok := uniqNodes[rNode.ID]
			if !ok {
				uniqNodes[rNode.ID] = rNode
			} else {
				if storedNode.Revision < rNode.Revision {
					uniqNodes[rNode.ID] = rNode
				}
			}

			// Store events from all revisions into the "events" variable
			events[rNode.ID] = mergeEvents(rNode, key, events[rNode.ID])
		}
	}

	// Move the nodes from the uniqNodes map to an slice required by TraversalV
	nodes := []*graph.Node{}
	for id, n := range uniqNodes {
		e, ok := events[id]
		if ok {
			// Set the stored node with the merge of Alarms from previous and current node
			metadataSet := uniqNodes[id].Metadata.SetField(aggKey, e)
			if !metadataSet {
				logging.GetLogger().Errorf("Unable to set events metadata for host %v", id)
			}
		}

		nodes = append(nodes, n)
	}

	return traversal.NewGraphTraversalV(tv.GraphTraversal, nodes)
}

// mergeEvents return the merge of node.Key events with the ones already stored in nodeEvents
// Eg.:
//   node:       Metadata.key: {"a":{x}, "b":{y}}
//   nodeEvents:               {"a":{z}}
//   return:     Metadata.key: {"a":[{x},{z}], "b":[{y}]}
//
// Ignore if Metadata.key has an invalid format.
func mergeEvents(node *graph.Node, key string, nodeEvents map[string][]interface{}) map[string][]interface{} {
	if nodeEvents == nil {
		nodeEvents = map[string][]interface{}{}
	}

	n1EventsIface, n1Err := node.GetField(key)
	if n1Err == nil {
		// Ignore Metadata.key values with not a valid format
		n1Events, _ := n1EventsIface.(map[string]interface{})
	NODE_EVENTS:
		for k, v := range n1Events {
			// Do not append if the same event already exists
			for _, storedEvent := range nodeEvents[k] {
				if reflect.DeepEqual(storedEvent, v) {
					continue NODE_EVENTS
				}
			}

			nodeEvents[k] = append(nodeEvents[k], v)
		}
	}

	return nodeEvents
}
