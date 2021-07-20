package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"
	"time"

	"github.com/skydive-project/skydive/topology/probes/graphql/generated"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
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

func (r *mutationResolver) AddEvents(ctx context.Context, input []*model.AddEventInput) (*model.AddEventPayload, error) {
	r.Graph.Lock()
	defer r.Graph.Unlock()

	var errors []error

	for _, event := range input {
		err := r.createEvent(event, time.Now())
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		msg := fmt.Sprintf("%v", errors)
		e := fmt.Errorf(msg)

		return &model.AddEventPayload{
			Ok:    false,
			Error: &msg,
		}, e
	}

	payload := model.AddEventPayload{
		Ok:    true,
		Error: nil,
	}
	return &payload, nil
}

func (r *mutationResolver) CloseEvent(ctx context.Context, input model.CloseEventInput) (*model.CloseEventPayload, error) {
	err := r.closeEvent(input)
	if err != nil {
		errorMessage := err.Error()
		return &model.CloseEventPayload{
			Ok:    false,
			Error: &errorMessage,
		}, err
	}

	return &model.CloseEventPayload{
		Ok: true,
	}, nil
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
