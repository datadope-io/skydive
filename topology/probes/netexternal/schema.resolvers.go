package netexternal

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"

	"github.com/skydive-project/skydive/topology/probes/netexternal/generated"
	"github.com/skydive-project/skydive/topology/probes/netexternal/model"
)

func (r *mutationResolver) AddNetworkDevice(ctx context.Context, input model.NetworkDeviceInput) (*model.AddNetworkDevicePayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	// Use "host" as the default device type
	// TODO: definir subtype (router, switch) y dejar type como host siempre.
	deviceType := "host"
	if input.Type != nil {
		deviceType = *input.Type
	}

	metadata := map[string]interface{}{
		MetaKeyName: input.Name,
		MetaKeyType: deviceType,
	}

	// Set optional metadata fields if defined
	if input.Vendor != nil {
		metadata[MetaKeyVendor] = *input.Vendor
	}

	if input.Model != nil {
		metadata[MetaKeyModel] = *input.Model
	}

	node, updated, _, err := r.addDeviceWithInterfaces(input.Name, metadata,
		input.Interfaces, input.CreatedAt)
	if err != nil {
		return nil, err
	}

	payload := model.AddNetworkDevicePayload{
		ID:      string(node.ID),
		Updated: updated,
	}
	return &payload, nil
}

func (r *mutationResolver) AddIf2IfLink(ctx context.Context, input model.If2IfLinkInput) (*model.AddIf2IfLinkPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	edge, err := r.createIf2IfEdge(input.SrcDevice, input.SrcInterface,
		input.DstDevice, input.DstInterface, input.CreatedAt)
	if err != nil {
		return nil, err
	}

	payload := model.AddIf2IfLinkPayload{
		ID: string(edge.ID),
	}
	return &payload, nil
}

func (r *mutationResolver) AddEvents(ctx context.Context, input []*model.EventInput) (*model.EventPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	var ok bool = true
	var err_msg string
	err := r.createEvents(input)
	if err != nil {
		ok = false
		err_msg = err.Error()
	}

	payload := model.EventPayload{
		Ok:    ok,
		Error: &err_msg,
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
