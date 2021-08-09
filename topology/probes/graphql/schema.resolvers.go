package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/graphql/generated"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
)

func (r *mutationResolver) NetAddDevice(ctx context.Context, input model.NetDeviceInput) (*model.NetDevicePayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	node, nodeCreated, nodeUpdated, err := r.netAddDevice(
		input.Name,
		input.Type,
		input.Model,
		input.Vendor,
		input.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	payload := model.NetDevicePayload{
		Device: &model.NetNodePayload{
			ID:          string(node.ID),
			NodeCreated: nodeCreated,
			NodeUpdated: nodeUpdated,
		},
	}
	return &payload, nil
}

func (r *mutationResolver) NetAddInterface(ctx context.Context, input model.NetAddInterfaceInput) (*model.NetInterfacePayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	// Get device node where the interface should be added
	device, err := r.netGetNodeFromSelector(input.DeviceSelector)
	if err != nil {
		return nil, fmt.Errorf("add interface selector: %v", err)
	}

	// Create and link the physical interface node to the device
	ifaceNode, nodeCreated, nodeUpdated, err := r.netAddInterfaceToDevice(
		device,
		input.Interface,
		MetadataSubtypePhysicalInterface,
		input.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add interface: %v", err)
	}

	payload := model.NetInterfacePayload{
		Interface: &model.NetNodePayload{
			ID:          string(ifaceNode.ID),
			NodeCreated: nodeCreated,
			NodeUpdated: nodeUpdated,
		},
	}

	// If aggregation has been defined, link it with the interface.
	// Create it if it does not exists
	if input.Aggregation != nil {
		aggInterface, err := r.netGetOrCreateAggInterface(device, *input.Aggregation, input.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("get or create aggregation interface: %v", err)
		}

		// Link aggregation to interface
		_, _, _, err = r.createOrUpdateEdge(aggInterface, ifaceNode, graph.Metadata{MetaKeyRelationType: MetadataSubtypeAggregationInterface}, input.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("unable to link aggregation interface to physical interface: %v", err)
		}

		payload.Aggregation = &model.NetNodePayload{
			ID: string(aggInterface.ID),
		}
	}

	return &payload, nil
}

func (r *mutationResolver) NetAddAggregation(ctx context.Context, input model.NetAddAggregationInput) (*model.NetAggregationPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	// Get device node where the interface should be added
	device, err := r.netGetNodeFromSelector(input.DeviceSelector)
	if err != nil {
		return nil, fmt.Errorf("add aggregation selector: %v", err)
	}

	// Create and link the physical interface node to the device
	aggNode, nodeCreated, nodeUpdated, err := r.netAddInterfaceToDevice(
		device,
		input.Aggregation,
		MetadataSubtypeAggregationInterface,
		input.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add aggregation: %v", err)
	}

	payload := model.NetAggregationPayload{
		Aggregation: &model.NetNodePayload{
			ID:          string(aggNode.ID),
			NodeCreated: nodeCreated,
			NodeUpdated: nodeUpdated,
		},
	}
	return &payload, nil
}

func (r *mutationResolver) NetAddLogicalInterface(ctx context.Context, input model.NetAddLogicalInterfaceInput) (*model.NetLogicalInterfacePayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	// Get interface node where the logical interface should be linked
	physicalInterface, err := r.netGetNodeFromSelector(input.InterfaceSelector)
	if err != nil {
		return nil, fmt.Errorf("add logical interface selector: %v", err)
	}

	// Create and link the logical interface node to the interface/aggregation
	logicalNode, nodeCreated, nodeUpdated, err := r.netAddLogicalToInterface(
		physicalInterface,
		input.Interface,
		input.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add logical interface: %v", err)
	}
	payload := model.NetLogicalInterfacePayload{
		LogicalInterface: &model.NetNodePayload{
			ID:          string(logicalNode.ID),
			NodeCreated: nodeCreated,
			NodeUpdated: nodeUpdated,
		},
	}
	return &payload, nil
}

func (r *mutationResolver) NetAddIf2IfLink(ctx context.Context, input model.NetIf2IfLinkInput) (*model.NetAddIf2IfLinkPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	srcIface, err := r.netGetNodeFromSelector(input.SrcInterface)
	if err != nil {
		return nil, fmt.Errorf("add link between interfaces, source selector: %v", err)
	}
	dstIface, err := r.netGetNodeFromSelector(input.DstInterface)
	if err != nil {
		return nil, fmt.Errorf("add link between interfaces, destination selector: %v", err)
	}

	edge, created, updated, err := r.createOrUpdateEdge(srcIface, dstIface, graph.Metadata{
		MetaKeyRelationType: RelationConnect,
	}, input.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("add link between interfaces, edge: %v", err)
	}

	payload := model.NetAddIf2IfLinkPayload{
		InterfaceLink: &model.NetEdgePayload{
			ID:          string(edge.ID),
			EdgeCreated: created,
			EdgeUpdated: updated,
		},
	}
	return &payload, nil
}

func (r *mutationResolver) EventAdd(ctx context.Context, input model.EventAddInput) (*model.EventAddPayload, error) {
	nodeID, err := r.createEvent(
		input.ID,
		input.GremlinQuery,
		input.Level,
		input.TTL,
		input.Payload,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	return &model.EventAddPayload{
		ID: string(nodeID),
	}, nil
}

func (r *mutationResolver) EventAddMultiple(ctx context.Context, input []*model.EventAddInput) (*model.EventAddMultiplePayload, error) {
	var errors []string

	for i, event := range input {
		_, err := r.createEvent(
			event.ID,
			event.GremlinQuery,
			event.Level,
			event.TTL,
			event.Payload,
			time.Now(),
		)
		if err != nil {
			errors = append(errors,
				fmt.Sprintf("Event %d (Query: '%s', ID: '%s'): %v", i, event.GremlinQuery, event.ID, err.Error()))
		}
	}

	payload := model.EventAddMultiplePayload{
		Ok:     len(input) - len(errors),
		Fail:   len(errors),
		Errors: errors,
	}

	if len(input) == 1 && len(errors) == 1 {
		return &payload, fmt.Errorf(errors[0])
	}
	return &payload, nil
}

func (r *mutationResolver) EventClose(ctx context.Context, input model.EventCloseInput) (*model.EventClosePayload, error) {
	node, err := r.closeEvent(input)
	if err != nil {
		return nil, err
	}

	payload := model.EventClosePayload{
		ID: string(node.ID),
	}
	return &payload, nil
}

func (r *queryResolver) Version(ctx context.Context) (string, error) {
	return "beta1", nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
