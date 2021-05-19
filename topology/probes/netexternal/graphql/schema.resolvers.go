package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
//
// TODO:
//  - eliminar cosas si desaparecen de la mutation, ejemplo:
//    - VID que se quitan de una iface
//    - interfaces que se quitan enteras
//    - se quita una IP de una interfaz
//    - etc

import (
	"context"
	"net"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/netexternal/graphql/generated"
	"github.com/skydive-project/skydive/topology/probes/netexternal/graphql/model"
)

func (r *mutationResolver) AddVlan(ctx context.Context, input model.VLANInput) (*model.AddVLANPayload, error) {
	node, err := r.createVLAN(input.Vid, *input.Name)
	if err != nil {
		return nil, err
	}

	payload := model.AddVLANPayload{
		ID: string(node.ID),
	}
	return &payload, nil
}

func (r *mutationResolver) AddSwitch(ctx context.Context, input model.SwitchInput) (*model.AddSwitchPayload, error) {
	hw, err := net.ParseMAC(input.Mac)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		MetadataNameKey:   input.Name,
		MetadataTypeKey:   TypeSwitch,
		MetadataMACKey:    hw.String(),
		MetadataVendorKey: input.Vendor,
		MetadataModelKey:  input.Model,
	}

	nodePKeyFilter := graph.Metadata{
		MetadataTypeKey: TypeSwitch,
		MetadataNameKey: input.Name,
	}

	node, updated, interfaceUpdated, err := r.addNodeWithInterfaces(metadata, input.Interfaces, nodePKeyFilter)
	if err != nil {
		return nil, err
	}

	payload := model.AddSwitchPayload{
		ID:               string(node.ID),
		Updated:          updated,
		InterfaceUpdated: interfaceUpdated,
	}
	return &payload, nil
}

func (r *mutationResolver) AddRouter(ctx context.Context, input model.RouterInput) (*model.AddRouterPayload, error) {
	metadata := map[string]interface{}{
		MetadataNameKey:   input.Name,
		MetadataTypeKey:   TypeRouter,
		MetadataVendorKey: input.Vendor,
		MetadataModelKey:  input.Model,
	}

	nodePKeyFilter := graph.Metadata{
		MetadataTypeKey: TypeRouter,
		MetadataNameKey: input.Name,
	}

	node, updated, interfaceUpdated, err := r.addNodeWithInterfaces(metadata, input.Interfaces, nodePKeyFilter)
	if err != nil {
		return nil, err
	}

	payload := model.AddRouterPayload{
		ID:               string(node.ID),
		Updated:          updated,
		InterfaceUpdated: interfaceUpdated,
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
