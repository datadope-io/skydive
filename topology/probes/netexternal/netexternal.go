/*
 * Copyright (C) 2016 Red Hat, Inc.
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

// Package netexternal provides an API so external programs could create network
// elements (nodes) using the internal go structures.
//
// This API is implemented with GraphQL to be able to define clearly the format that
// has to be used.
//
package netexternal

import (
	"net/http"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"

	"github.com/skydive-project/skydive/topology/probes/netexternal/graphql"
	"github.com/skydive-project/skydive/topology/probes/netexternal/graphql/generated"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
)

// Probe describes this probe
type Probe struct {
	graph *graph.Graph
}

// Start initilizates the netexternal probe, starting a web server serving a GraphQL API
func (p *Probe) Start() error {
	listenEndpoint := config.GetString("analyzer.topology.netexternal.listen")

	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: &graphql.Resolver{
			Graph: p.graph,
		},
	}))

	http.Handle("/", playground.Handler("netexternal GraphQL playground", "/query"))
	http.Handle("/query", srv)

	go http.ListenAndServe(listenEndpoint, nil)

	logging.GetLogger().Infof("Listening for network elements on %v", listenEndpoint)

	return nil
}

// Stop the probe
func (p *Probe) Stop() {
}

// NewProbe initialize the probe with the parameters from the config file
func NewProbe(g *graph.Graph) (*Probe, error) {
	probe := &Probe{
		graph: g,
	}

	return probe, nil
}

// Register called at initialization to register metadata decoders
func Register() {
}
