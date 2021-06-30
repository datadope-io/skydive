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

	var deviceType *string
	if input.Type == nil {
		str := "host"
		deviceType = &str
	} else {
		deviceType = input.Type
	}

	metadata := map[string]interface{}{
		MetadataNameKey:   input.Name,
		MetadataVendorKey: input.Vendor,
		MetadataModelKey:  input.Model,
		MetadataTypeKey:   deviceType,
	}

	node, updated, _, err := r.addNodeWithInterfaces(input.Name, metadata,
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

func (r *mutationResolver) AddEvent(ctx context.Context, input model.EventInput) (*model.EventPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	metadata := map[string]interface{}{
		MetadataNameKey: input.Name,
		MetadataTypeKey: "host",
		"foo":           "bar",
	}

	var ok bool = true
	var err_msg string
	_, err := r.updateNode(input.Name, metadata, input.Time)
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
