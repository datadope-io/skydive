//go:generate sh -c "go run github.com/gomatic/renderizer --name=node --resource=node --type=Node --title=Node --article=a ../../../api/server/swagger_operations.tmpl > node_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=node --resource=node --type=Node --title=Node ../../../api/server/swagger_definitions.tmpl > node_swagger.json"

/*
 * Copyright (C) 2020 Sylvain Baubeau
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package server

import (
	"context"
	"time"

	"github.com/skydive-project/skydive/graffiti/api/rest"
	"github.com/skydive-project/skydive/graffiti/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
)

const apiOrigin = "api"

// NodeResourceHandler aims to creates and manage a new Alert.
type NodeResourceHandler struct {
	rest.ResourceHandler
}

// NodeAPIHandler aims to exposes the Alert API.
type NodeAPIHandler struct {
	g *graph.Graph
}

// New creates a new node
func (h *NodeAPIHandler) New(ctx context.Context) rest.Resource {
	return &types.Node{}
}

// Name returns resource name "node"
func (h *NodeAPIHandler) Name() string {
	return "node"
}

// Index returns the list of existing nodes
func (h *NodeAPIHandler) Index(ctx context.Context) map[string]rest.Resource {
	ctx, span := tracer.Start(ctx, "NodeAPIHandler.Index")
	defer span.End()

	h.g.RLock()
	nodes := h.g.GetNodes(ctx, nil)
	nodeMap := make(map[string]rest.Resource, len(nodes))
	for _, node := range nodes {
		n := types.Node(*node)
		nodeMap[string(node.ID)] = &n
	}
	h.g.RUnlock()
	return nodeMap
}

// Get returns a node with the specified id
func (h *NodeAPIHandler) Get(ctx context.Context, id string) (rest.Resource, bool) {
	h.g.RLock()
	defer h.g.RUnlock()

	n := h.g.GetNode(ctx, graph.Identifier(id))
	if n == nil {
		return nil, false
	}
	node := (*types.Node)(n)
	return node, true
}

// Decorate the specified node
func (h *NodeAPIHandler) Decorate(ctx context.Context, resource rest.Resource) {
}

// Create adds the specified node to the graph
func (h *NodeAPIHandler) Create(ctx context.Context, resource rest.Resource, createOpts *rest.CreateOptions) error {
	node := resource.(*types.Node)
	graphNode := graph.Node(*node)
	if graphNode.CreatedAt.IsZero() {
		graphNode.CreatedAt = graph.Time(time.Now())
	}
	if graphNode.UpdatedAt.IsZero() {
		graphNode.UpdatedAt = graphNode.CreatedAt
	}
	if graphNode.Origin == "" {
		graphNode.Origin = graph.Origin(h.g.GetHost(), apiOrigin)
	}
	if graphNode.Metadata == nil {
		graphNode.Metadata = graph.Metadata{}
	}

	h.g.Lock()
	err := h.g.AddNode(ctx, &graphNode)
	h.g.Unlock()
	return err
}

// Delete the node with the specified id from the graph
func (h *NodeAPIHandler) Delete(ctx context.Context, id string) error {
	h.g.Lock()
	defer h.g.Unlock()

	node := h.g.GetNode(ctx, graph.Identifier(id))
	if node == nil {
		return rest.ErrNotFound
	}

	return h.g.DelNode(ctx, node)
}

// Update a node metadata
func (h *NodeAPIHandler) Update(ctx context.Context, id string, resource rest.Resource) (rest.Resource, bool, error) {
	h.g.Lock()
	defer h.g.Unlock()

	// Current node, to be updated
	n := h.g.GetNode(ctx, graph.Identifier(id))
	if n == nil {
		return nil, false, rest.ErrNotFound
	}

	// Node containing the metadata updated
	patchedNode := resource.(*types.Node)

	// Do not modify/replace Metadata.(TID|Name|Type), use actual node values
	if actualTID, _ := n.Metadata.GetFieldString("TID"); actualTID != "" {
		patchedNode.Metadata.SetField("TID", actualTID)
	}
	actualName, _ := n.Metadata.GetFieldString("Name")
	patchedNode.Metadata.SetField("Name", actualName)
	actualType, _ := n.Metadata.GetFieldString("Type")
	patchedNode.Metadata.SetField("Type", actualType)

	// Update actual node Metadata with new patched node
	previousRevision := n.Revision
	if err := h.g.SetMetadata(ctx, n, patchedNode.Metadata); err != nil {
		return nil, false, err
	}

	return (*types.Node)(n), n.Revision != previousRevision, nil
}

// RegisterNodeAPI registers the node API
func RegisterNodeAPI(apiServer *Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) *NodeAPIHandler {
	nodeAPIHandler := &NodeAPIHandler{
		g: g,
	}
	apiServer.RegisterAPIHandler(nodeAPIHandler, authBackend)
	return nodeAPIHandler
}
