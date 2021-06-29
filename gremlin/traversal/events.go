package traversal

import (
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// EventsTraversalExtension describes a new extension to enhance the topology
type EventsTraversalExtension struct {
	EventsToken traversal.Token
}

// EventsGremlinTraversalStep step aggregates events from different revisions of the nodes into a new metadata key.
// This step should be used with a presistant backend, so it can access previous revisions of the nodes.
// To use this step we should select a metadata key (first parameter), where the events will be read from.
// Inside this Metadata.Key events should have the format map[string]interface{}.
// The second parameter is the metadata key where all the events will be aggregated.
// The aggregation will with the format: map[string][]interface{}.
// All events with the same key in the map will be joined in an slice.
// To use this step we can use a graph with a time period context, eg: G.At(1479899809,3600).V().Events('A','B').
// Or we can define the time period in the step: G.V().Events('A','B',1500000000,1500099999).
// Note that in this case we define the start and end time, while in "At" is start time and duration.
// In both cases, Events step will use the nodes given by the previous step.
type EventsGremlinTraversalStep struct {
	traversal.GremlinTraversalContext
	EventKey    string
	EventAggKey string
	StartTime   time.Time
	EndTime     time.Time
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
	var startTime, endTime time.Time
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
	case 4:
		eventKey, ok = p.Params[0].(string)
		if !ok {
			return nil, errors.New("Events first parameter have to be a string")
		}
		eventAggKey, ok = p.Params[1].(string)
		if !ok {
			return nil, errors.New("Events second parameter have to be a string")
		}
		startTimeUnixEpoch, ok := p.Params[2].(int64)
		if !ok {
			return nil, errors.New("Events third parameter have to be a int (unix epoch time)")
		}
		startTime = time.Unix(startTimeUnixEpoch, 0)
		endTimeUnixEpoch, ok := p.Params[3].(int64)
		if !ok {
			return nil, errors.New("Events fourth parameter have to be a int (unix epoch time)")
		}
		endTime = time.Unix(endTimeUnixEpoch, 0)
	default:
		return nil, errors.New("Events parameter must have two or four parameters (OriginKey, DestinationKey, StartTime, EndTime)")
	}

	return &EventsGremlinTraversalStep{
		GremlinTraversalContext: p,
		EventKey:                eventKey,
		EventAggKey:             eventAggKey,
		StartTime:               startTime,
		EndTime:                 endTime,
	}, nil
}

// Exec executes the events step
func (s *EventsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		return s.InterfaceEvents(tv)

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
func (s *EventsGremlinTraversalStep) InterfaceEvents(tv *traversal.GraphTraversalV) (traversal.GraphTraversalStep, error) {
	it := s.StepContext.PaginationRange.Iterator()

	// If user has defined start/end time in the parameters, use that values instead of the ones comming with the graph
	if !s.StartTime.IsZero() && !s.EndTime.IsZero() {
		timeSlice := graph.NewTimeSlice(
			graph.Time(s.StartTime).UnixMilli(),
			graph.Time(s.EndTime).UnixMilli(),
		)
		userTimeSliceCtx := graph.Context{
			TimeSlice: timeSlice,
			TimePoint: true,
		}

		newGraph, err := tv.GraphTraversal.Graph.CloneWithContext(userTimeSliceCtx)
		if err != nil {
			return nil, err
		}
		tv.GraphTraversal.Graph = newGraph
	}

	tv.GraphTraversal.RLock()
	defer tv.GraphTraversal.RUnlock()

	// uniqNodes store the latest node for each node identifier
	uniqNodes := map[graph.Identifier]*graph.Node{}
	uniqNodesLock := sync.Mutex{}

	// events accumulate the events for each node id
	events := map[graph.Identifier]map[string][]interface{}{}
	eventsLock := sync.Mutex{}

	// Limit the number of goroutines querying the backend at the same time
	// A high number of concurrent queries makes ElasticSearch slower.
	backendLimitConcurrentCalls := make(chan struct{}, 40)

	wg := sync.WaitGroup{}

	for _, node := range tv.GetNodes() {
		// TODO afecta la paginación a esta función? Cuando se usa esta paginación?
		if it.Done() {
			break
		}

		// Spawn a different goroutine for each node to speed up this function
		// Elasticsearch client is thread safe.
		wg.Add(1)
		go func(nodeID graph.Identifier) {
			defer wg.Done()

			// Get all revisions for this node
			backendLimitConcurrentCalls <- struct{}{}
			revisionNodes := tv.GraphTraversal.Graph.GetNodeAll(nodeID)
			<-backendLimitConcurrentCalls

			// Store only the most recent nodes
			for _, rNode := range revisionNodes {
				uniqNodesLock.Lock()
				storedNode, ok := uniqNodes[rNode.ID]
				if !ok {
					uniqNodes[rNode.ID] = rNode
				} else {
					if storedNode.Revision < rNode.Revision {
						uniqNodes[rNode.ID] = rNode
					}
				}
				uniqNodesLock.Unlock()

				// Store events from all revisions into the "events" variable
				eventsLock.Lock()
				events[rNode.ID] = mergeEvents(rNode, s.EventKey, events[rNode.ID])
				eventsLock.Unlock()
			}
		}(node.ID)
	}

	wg.Wait()

	// Move the nodes from the uniqNodes map to an slice required by TraversalV
	nodes := []*graph.Node{}
	for id, n := range uniqNodes {
		e, ok := events[id]
		if ok {
			// Set the stored node with the merge of Events from previous and current node
			metadataSet := uniqNodes[id].Metadata.SetField(s.EventAggKey, e)
			if !metadataSet {
				logging.GetLogger().Errorf("Unable to set events metadata for host %v", id)
			}
		}

		nodes = append(nodes, n)
	}

	return traversal.NewGraphTraversalV(tv.GraphTraversal, nodes), nil
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
