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
	"sync"

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

// getNeighbors given a list of nodes, get its neighbors nodes for "maxDepth" depth relationships.
// Edges between nodes must fulfill "edgeFilter" filter.
// Nodes passed to this function will always be in the response.
// This funcion will execute calls to the graph concurrently, to increase response time when a persistent backend
// is involved.
func getNeighbors(g *graph.Graph, nodes []*graph.Node, maxDepth int64, edgeFilter graph.ElementMatcher) []*graph.Node {
	wg := sync.WaitGroup{}
	ndnLock := sync.Mutex{}

	// Limit the number of goroutines querying the backend at the same time
	// A high number of concurrent queries makes ElasticSearch slower.
	backendLimitConcurrentCalls := make(chan struct{}, 40)

	// visitedNodes store neighors and avoid visiting twice the same node
	visitedNodes := map[graph.Identifier]interface{}{}
	visitedNodesLock := sync.Mutex{}

	// currentDepthNodes slice with the nodes being processed in each depth.
	currentDepthNodes := []graph.Identifier{}
	// nextDepthNodes slice were next depth nodes are being stored.
	// Initializated with the list of origin nodes where it should start from.
	nextDepthNodes := []graph.Identifier{}

	// Neighbor step will return also the origin nodes
	for _, n := range nodes {
		visitedNodes[n.ID] = struct{}{}
		nextDepthNodes = append(nextDepthNodes, n.ID)
	}

	// DFS
	// BFS must not be used because could lead to ignore some servers in this case:
	//   A -> B
	//   B -> C
	//   C -> D
	//   A -> C
	//   With depth=2, BFS will return A,B,C (C is visited in A->B->C, si ignored in A->C->D)
	//   DFS will return, the correct, A,B,C,D
	// Walk concurrently all nodes in the same depth.
	// Once finished, walk the next depth.
	for i := 0; i < int(maxDepth); i++ {
		// Copy values from nextDepthNodes to currentDepthNodes
		currentDepthNodes = make([]graph.Identifier, len(nextDepthNodes))
		copy(currentDepthNodes, nextDepthNodes)

		nextDepthNodes = nextDepthNodes[:0] // Clean slice, keeping capacity
		for _, n := range currentDepthNodes {
			wg.Add(1)
			go func(nodeID graph.Identifier) {
				defer wg.Done()

				// Get edges for node with id "nodeID"
				backendLimitConcurrentCalls <- struct{}{}
				edges := g.GetNodeEdges(graph.CreateNode(nodeID, nil, graph.Time{}, "", ""), edgeFilter)
				<-backendLimitConcurrentCalls

				for _, e := range edges {
					// Get nodeID of the other side of the edge
					var neighborID graph.Identifier
					if nodeID == e.Parent {
						neighborID = e.Child
					} else {
						neighborID = e.Parent
					}

					// Store neighbors
					visitedNodesLock.Lock()
					_, ok := visitedNodes[neighborID]
					if !ok {
						visitedNodes[neighborID] = struct{}{}
					}
					visitedNodesLock.Unlock()

					// Do not walk nodes already processed
					if ok {
						continue
					}

					// Append this node ID in the list of nodes to be visited in the next depth
					ndnLock.Lock()
					nextDepthNodes = append(nextDepthNodes, neighborID)
					ndnLock.Unlock()
				}
			}(n)
		}
		wg.Wait()
	}

	// Get concurrentl all nodes for the list of neighbors ids
	neighbors := make([]*graph.Node, 0, len(visitedNodes))
	neighborsLock := sync.Mutex{}

	for n := range visitedNodes {
		wg.Add(1)
		go func(nodeID graph.Identifier) {
			defer wg.Done()

			backendLimitConcurrentCalls <- struct{}{}
			node := g.GetNode(nodeID)
			<-backendLimitConcurrentCalls

			neighborsLock.Lock()
			neighbors = append(neighbors, node)
			neighborsLock.Unlock()

		}(n)
	}
	wg.Wait()

	return neighbors
}

// Exec Neighbors step
func (d *NeighborsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		neighbors := getNeighbors(tv.GraphTraversal.Graph, tv.GetNodes(), d.maxDepth, d.edgeFilter)
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
