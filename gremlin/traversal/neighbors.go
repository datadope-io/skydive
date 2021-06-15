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

package traversal

import (
	"github.com/pkg/errors"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology"
)

// NeighborsTraversalExtension describes a new extension to enhance the topology
type NeighborsTraversalExtension struct {
	NeighborsToken traversal.Token
}

// NeighborsGremlinTraversalStep rawpackets step
type NeighborsGremlinTraversalStep struct {
	context    traversal.GremlinTraversalContext
	maxDepth   int64
	edgeFilter graph.ElementMatcher
}

// NewNeighborsTraversalExtension returns a new graph traversal extension
func NewNeighborsTraversalExtension() *NeighborsTraversalExtension {
	return &NeighborsTraversalExtension{
		NeighborsToken: traversalNeighborsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *NeighborsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "NEIGHBORS":
		return e.NeighborsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses neighbors step
func (e *NeighborsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.NeighborsToken:
	default:
		return nil, nil
	}

	maxDepth := int64(1)
	edgeFilter, _ := topology.OwnershipMetadata().Filter()

	switch len(p.Params) {
	case 0:
	default:
		i := len(p.Params) / 2 * 2
		filter, err := traversal.ParamsToFilter(filters.BoolFilterOp_OR, p.Params[:i]...)
		if err != nil {
			return nil, errors.Wrap(err, "Neighbors accepts an optional number of key/value tuples and an optional depth")
		}
		edgeFilter = filter

		if i == len(p.Params) {
			break
		}

		fallthrough
	case 1:
		depth, ok := p.Params[len(p.Params)-1].(int64)
		if !ok {
			return nil, errors.New("Neighbors last argument must be the maximum depth specified as an integer")
		}
		maxDepth = depth
	}

	return &NeighborsGremlinTraversalStep{context: p, maxDepth: maxDepth, edgeFilter: graph.NewElementFilter(edgeFilter)}, nil
}

// getNeighbors given a list of nodes, add them to the neighbors list, get the neighbors of that nodes and call this function with those new nodes
func getNeighbors(g *graph.Graph, nodes []*graph.Node, neighbors *[]*graph.Node, currDepth, maxDepth int64, edgeFilter graph.ElementMatcher, visited map[graph.Identifier]bool) {
	var newNodes []*graph.Node
	for _, node := range nodes {
		if _, ok := visited[node.ID]; !ok {
			newNodes = append(newNodes, node)
			visited[node.ID] = true
		}
	}
	*neighbors = append(*neighbors, newNodes...)

	if maxDepth == 0 || currDepth < maxDepth {
		// For each node get its edges and nodes
		for _, node := range newNodes {
			// Get neighbors nodes, ignoring the ones already visited
			parents := g.LookupNeighborIgnoreVisited(node, nil, edgeFilter, visited)
			getNeighbors(g, parents, neighbors, currDepth+1, maxDepth, edgeFilter, visited)
		}
	}
}

// Exec Neighbors step
func (d *NeighborsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var neighbors []*graph.Node

	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		getNeighbors(tv.GraphTraversal.Graph, tv.GetNodes(), &neighbors, 0, d.maxDepth, d.edgeFilter, make(map[graph.Identifier]bool))
		tv.GraphTraversal.RUnlock()

		return traversal.NewGraphTraversalV(tv.GraphTraversal, neighbors), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Neighbors step
func (d *NeighborsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context Neighbors step
func (d *NeighborsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &d.context
}
