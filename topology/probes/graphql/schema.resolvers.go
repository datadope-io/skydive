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

func (r *mutationResolver) NetAddRegion(ctx context.Context, input model.NetRegionInput) (*model.NetRegionPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	node, created, updated, err := r.netAddLocation(input.Name, MetadataTypeRegion, input.Location, nil, input.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &model.NetRegionPayload{
		ID:          string(node.ID),
		NodeCreated: created,
		NodeUpdated: updated,
	}, nil
}

func (r *mutationResolver) NetAddProvince(ctx context.Context, input model.NetProvinceInput) (*model.NetProvincePayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	node, created, updated, err := r.netAddLocation(input.Name, MetadataTypeProvince, input.Location, nil, input.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Link the Province to the Region
	Region := r.Graph.GetNodes(graph.Metadata{MetaKeyType: MetadataTypeRegion, MetaKeyName: *&input.Region})
	if len(Region) == 0 {
		return nil, fmt.Errorf("Region %s not found", input.Region)
	} else if len(Region) > 1 {
		return nil, fmt.Errorf("multiple Region %s found: %+v", input.Region, Region)
	}

	_, _, _, err = r.createOrUpdateEdge(Region[0], node, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership}, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to link Region (%v) to province (%v): %v", Region[0], node, err)
	}

	return &model.NetProvincePayload{
		ID:          string(node.ID),
		NodeCreated: created,
		NodeUpdated: updated,
	}, nil
}

func (r *mutationResolver) NetAddBuilding(ctx context.Context, input model.NetBuildingInput) (*model.NetBuildingPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	node, created, updated, err := r.netAddLocation(input.Name, MetadataTypeBuilding, input.Location, input.Class, input.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Link the building to the province
	province := r.Graph.GetNodes(graph.Metadata{MetaKeyType: MetadataTypeProvince, MetaKeyName: *&input.Province})
	if len(province) == 0 {
		return nil, fmt.Errorf("province %s not found", input.Province)
	} else if len(province) > 1 {
		return nil, fmt.Errorf("multiple province %s found: %+v", input.Province, province)
	}

	_, _, _, err = r.createOrUpdateEdge(province[0], node, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership}, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to link province (%v) to building (%v): %v", province[0], node, err)
	}

	return &model.NetBuildingPayload{
		Building: &model.NetNodePayload{
			ID:          string(node.ID),
			NodeCreated: created,
			NodeUpdated: updated,
		},
	}, nil
}

func (r *mutationResolver) NetAddDevice(ctx context.Context, input model.NetDeviceInput) (*model.NetDevicePayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	node, nodeCreated, nodeUpdated, err := r.netAddDevice(
		input.Name,
		input.DeviceID,
		input.Type,
		input.Model,
		input.Vendor,
		input.BuildingName,
		input.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// If building is defined, link device to building and set the device coordinates to building coordinates
	if input.Building != nil {
		building := r.Graph.GetNodes(graph.Metadata{MetaKeyType: MetadataTypeBuilding, MetaKeyName: *input.Building})
		if len(building) == 0 {
			return nil, fmt.Errorf("building %s not found", *input.Building)
		} else if len(building) > 1 {
			return nil, fmt.Errorf("multiple building %s found: %+v", *input.Building, building)
		}

		// Link building to device
		_, _, _, err = r.createOrUpdateEdge(building[0], node, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership}, input.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("unable to link building (%v) to device (%v): %v", building[0], node, err)
		}
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

func (r *mutationResolver) NetDelInterface(ctx context.Context, input model.NetDelInterfaceInput) (*model.NetInterfacePayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	// Get device node where the interface should be added
	iface, err := r.netGetNodeFromSelector(input.InterfaceSelector)
	if err != nil {
		return nil, fmt.Errorf("del interface selector: %v", err)
	}

	err = r.Graph.DelNode(iface)
	if err != nil {
		return nil, fmt.Errorf("del interface: %w", err)
	}

	payload := model.NetInterfacePayload{
		Interface: &model.NetNodePayload{
			ID: string(iface.ID),
		},
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
	baseInterface, err := r.netGetNodeFromSelector(input.InterfaceSelector)
	if err != nil {
		return nil, fmt.Errorf("add logical interface selector: %v", err)
	}

	// Create and link the logical interface node to the interface/aggregation
	logicalNode, nodeCreated, nodeUpdated, err := r.netAddLogicalToInterface(
		baseInterface,
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

func (r *mutationResolver) NetAddIf2IfLink(ctx context.Context, input model.NetAddIf2IfLinkInput) (*model.NetIf2IfLinkPayload, error) {
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

	// Store all the edges created in this array to return them in the payload
	interfaceLinks := []*model.NetEdgePayload{}

	// Create the link between the interfaces
	edge, created, updated, err := r.createOrUpdateEdge(srcIface, dstIface, graph.Metadata{
		MetaKeyRelationType: RelationConnect,
	}, input.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("add link between interfaces, edge: %v", err)
	}

	interfaceLinks = append(interfaceLinks, &model.NetEdgePayload{
		ID:          string(edge.ID),
		EdgeCreated: created,
		EdgeUpdated: updated,
	})

	// Create links between interfaces all the way up to the regions, creating also links between the different levels.
	// This is done to allow to have links between different levels without having to get all the data down to the interface.
	// TODO: que tipo de edge le ponemos? Como gestionamos el borrado?
	// TODO: como la vista de mapa solo llega hasta building, y es en esta vista donde necesitamos todas estas conex duplicadas,
	// no generar conex duplicadas entre devices
	//   srcBuilding -> dstBuilding, dstProvince, dstRegion
	//   srcProvince -> dstBuilding, dstProvince, dstRegion
	//   srcRegion   -> dstBuilding, dstProvince, dstRegion
	for _, srcNode := range r.getHierarchyNodesFromNode(srcIface) {
		srcType, err := srcNode.GetFieldString(MetaKeyType)
		if err != nil {
			return nil, fmt.Errorf("missing Metadata.Type on node %+v: %v", srcNode, err)
		}
		if srcType == MetadataTypeNetworkInterface || srcType == MetadataTypeNetworkDevice {
			continue
		}

		for _, dstNode := range r.getHierarchyNodesFromNode(dstIface) {
			dstType, err := dstNode.GetFieldString(MetaKeyType)
			if err != nil {
				return nil, fmt.Errorf("missing Metadata.Type on node %+v: %v", dstNode, err)
			}
			if dstType == MetadataTypeNetworkInterface || dstType == MetadataTypeNetworkDevice {
				continue
			}

			edge, created, updated, err = r.createOrUpdateEdge(srcNode, dstNode, graph.Metadata{
				MetaKeyRelationType: RelationConnect,
			}, input.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("add link between interfaces, edge: %v", err)
			}

			interfaceLinks = append(interfaceLinks, &model.NetEdgePayload{
				ID:          string(edge.ID),
				EdgeCreated: created,
				EdgeUpdated: updated,
			})
		}
	}

	payload := model.NetIf2IfLinkPayload{InterfaceLinks: interfaceLinks}
	return &payload, nil
}

func (r *mutationResolver) NetDelIf2IfLink(ctx context.Context, input model.NetDelIf2IfLinkInput) (*model.NetIf2IfLinkPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	srcIface, err := r.netGetNodeFromSelector(input.SrcInterface)
	if err != nil {
		return nil, fmt.Errorf("del link between interfaces, source selector: %v", err)
	}
	dstIface, err := r.netGetNodeFromSelector(input.DstInterface)
	if err != nil {
		return nil, fmt.Errorf("del link between interfaces, destination selector: %v", err)
	}

	lldpEdge := r.Graph.GetFirstLink(srcIface, dstIface, graph.Metadata{
		MetaKeyRelationType: RelationConnect,
	})

	if lldpEdge == nil {
		return nil, fmt.Errorf("edge not found")
	}

	// Delete the edge
	err = r.Graph.DelEdge(lldpEdge)
	if err != nil {
		return nil, fmt.Errorf("unable to delete edge (%+v): %w", lldpEdge, err)
	}

	return &model.NetIf2IfLinkPayload{
		InterfaceLinks: []*model.NetEdgePayload{{
			ID: string(lldpEdge.ID),
		}},
	}, nil
}

func (r *mutationResolver) EventAdd(ctx context.Context, input model.EventAddInput) (*model.EventAddPayload, error) {
	propagate := false
	if input.Propagate != nil {
		propagate = *input.Propagate
	}

	nodeID, err := r.createEvent(
		input.ID,
		input.GremlinQuery,
		input.Level,
		input.TTL,
		input.Payload,
		time.Now(),
		propagate,
		[]string{},
		graph.Identifier(""),
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
		propagate := false
		if event.Propagate != nil {
			propagate = *event.Propagate
		}

		_, err := r.createEvent(
			event.ID,
			event.GremlinQuery,
			event.Level,
			event.TTL,
			event.Payload,
			time.Now(),
			propagate,
			[]string{},
			graph.Identifier(""),
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
	r.Graph.Lock()
	defer r.Graph.Unlock()

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

type (
	mutationResolver struct{ *Resolver }
	queryResolver    struct{ *Resolver }
)
