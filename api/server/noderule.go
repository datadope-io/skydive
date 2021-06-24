//go:generate sh -c "go run github.com/gomatic/renderizer --name='node rule' --resource=noderule --type=NodeRule --title='Node rule' --article=a swagger_operations.tmpl > noderule_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name='node rule' --resource=noderule --type=NodeRule --title='Node rule' swagger_definitions.tmpl > noderule_swagger.json"

/*
 * Copyright (C) 2018 Red Hat, Inc.
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

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
)

// NodeRuleResourceHandler describes a node resource handler
type NodeRuleResourceHandler struct {
	rest.ResourceHandler
}

// NodeRuleAPI based on BasicAPIHandler
type NodeRuleAPI struct {
	rest.BasicAPIHandler
	Graph *graph.Graph
}

// Name returns resource name "noderule"
func (nrh *NodeRuleResourceHandler) Name() string {
	return "noderule"
}

// New creates a new node rule
func (nrh *NodeRuleResourceHandler) New() rest.Resource {
	return &types.NodeRule{}
}

// Update an edge rule
func (a *NodeRuleAPI) Update(ctx context.Context, id string, resource rest.Resource) (rest.Resource, bool, error) {
	return nil, false, rest.ErrNotUpdatable
}

// RegisterNodeRuleAPI register a new node rule api handler
func RegisterNodeRuleAPI(apiServer *api.Server, g *graph.Graph, authBackend shttp.AuthenticationBackend) *NodeRuleAPI {
	nra := &NodeRuleAPI{
		BasicAPIHandler: rest.BasicAPIHandler{
			ResourceHandler: &NodeRuleResourceHandler{},
			EtcdClient:      apiServer.EtcdClient,
		},
		Graph: g,
	}
	apiServer.RegisterAPIHandler(nra, authBackend)
	return nra
}
