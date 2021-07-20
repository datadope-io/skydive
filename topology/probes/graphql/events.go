package graphql

import (
	"fmt"
	"time"

	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
)

const (
	// MetaKeyEvents key name where all events are stored inside of Metadata
	MetaKeyEvents = "Events"
)

// createEvent add, or update, an event to the node selected by the gremlin query, using
// the parameter "t" as the event CreatedAt and UpdatedAt values (except for updates, where
// CreatedAt remains the original one).
func (r *Resolver) createEvent(event *model.AddEventInput, t time.Time) error {
	if event.ID == "" {
		return fmt.Errorf("ID should not be empty")
	}

	node, err := r.getNodeFromGremlinQuery(event.GremlinQuery)
	if err != nil {
		return err
	}

	// Parse the gremlin query
	// Map the GraphQL event data, to the data stored in Metadata.Events
	e := Event{
		Level:     event.Level.String(),
		CreatedAt: graph.Time(t),
		UpdatedAt: graph.Time(t),
	}

	// Set TTL if defined
	if event.TTL != nil {
		e.TTL = int64(*event.TTL)
	}

	// Set payload if defined
	payloadValues := make([]PayloadValues, 0, len(event.Payload))
	for _, v := range event.Payload {
		payloadValues = append(payloadValues, PayloadValues{
			Key:   v.Key,
			Value: v.Value,
		})
	}
	e.Payload = payloadValues

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
	} else if updateErr != nil {
		return fmt.Errorf("unable to add event: %v", updateErr)
	}

	return nil
}

// closeEvent remove an event from a node specified by a gremlin query
func (r *Resolver) closeEvent(event model.CloseEventInput) error {
	if event.ID == "" {
		return fmt.Errorf("ID should not be empty")
	}

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
		return fmt.Errorf("unable to delete event: %v", err)
	}
	if updateErr != nil {
		return fmt.Errorf("unable to delete event: %v", updateErr)
	}

	return nil
}
