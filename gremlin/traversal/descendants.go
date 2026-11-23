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

// DescendantsTraversalExtension describes a new extension to enhance the topology
type DescendantsTraversalExtension struct {
	DescendantsToken traversal.Token
}

// DescendantsGremlinTraversalStep rawpackets step
type DescendantsGremlinTraversalStep struct {
	context    traversal.GremlinTraversalContext
	maxDepth   int64
	edgeFilter graph.ElementMatcher
}

// NewDescendantsTraversalExtension returns a new graph traversal extension
func NewDescendantsTraversalExtension() *DescendantsTraversalExtension {
	return &DescendantsTraversalExtension{
		DescendantsToken: traversalDescendantsToken,
	}
}

// ScanIdent returns an associated graph token
func (e *DescendantsTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "DESCENDANTS":
		return e.DescendantsToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses descendants step
func (e *DescendantsTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.DescendantsToken:
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
			return nil, errors.Wrap(err, "Descendants accepts an optional number of key/value tuples and an optional depth")
		}
		edgeFilter = filter

		if i == len(p.Params) {
			break
		}

		fallthrough
	case 1:
		depth, ok := p.Params[len(p.Params)-1].(int64)
		if !ok {
			return nil, errors.New("Descendants last argument must be the maximum depth specified as an integer")
		}
		maxDepth = depth
	}

	return &DescendantsGremlinTraversalStep{context: p, maxDepth: maxDepth, edgeFilter: graph.NewElementFilter(edgeFilter)}, nil
}

const (
	// descendantsEdgesBatchSize is the number of parent nodes per edges request.
	// It is kept below the batch size of the Elasticsearch backend
	// (maxClauseCount/2 = 256 nodes) so each request is a single search, and a
	// result reaching descendantsMaxSearchHits means that search was truncated.
	descendantsEdgesBatchSize = 200
	// descendantsNodesBatchSize is the number of node IDs per nodes request
	descendantsNodesBatchSize = 500
)

// descendantsMaxSearchHits is the maximum number of hits of a single backend
// search (Elasticsearch client Search: Size 10000). A variable for the tests.
var descendantsMaxSearchHits = 10000

// descendantsGraph is the part of the graph used by the Descendants step.
// *graph.Graph implements it.
type descendantsGraph interface {
	GetContext() graph.Context
	LookupChildren(n *graph.Node, f graph.ElementMatcher, em graph.ElementMatcher) []*graph.Node
	GetNodesEdges(n []*graph.Node, m graph.ElementMatcher) []*graph.Edge
	GetNodesOneRevision(i []graph.Identifier) ([]*graph.Node, bool)
}

// childrenLookup returns the children of a node
type childrenLookup func(parent *graph.Node) []*graph.Node

func getDescendants(lookup childrenLookup, parents []*graph.Node, descendants *[]*graph.Node, currDepth, maxDepth int64, visited map[graph.Identifier]bool) {
	var ld []*graph.Node
	for _, parent := range parents {
		if _, ok := visited[parent.ID]; !ok {
			ld = append(ld, parent)
			visited[parent.ID] = true
		}
	}
	*descendants = append(*descendants, ld...)

	if maxDepth == 0 || currDepth < maxDepth {
		for _, parent := range parents {
			children := lookup(parent)
			getDescendants(lookup, children, descendants, currDepth+1, maxDepth, visited)
		}
	}
}

// childrenLookupFunc returns the function used to get the children of a node.
//
// Live queries and time points use the graph LookupChildren, as before.
// History queries over a time slice (G.At(t, d)) fetch beforehand, in batches,
// the children of every node the walk can reach. LookupChildren would make
// one backend request per node plus two per edge, sequentially: with ~180
// servers that took over 60s against a remote Elasticsearch.
//
// The result is the same: getDescendants keeps the first revision of each node
// it finds (visited is by ID), that is, for each child, the first revision
// returned by the backend GetNode, which is what GetNodesOneRevision returns.
// Edges keep the backend order per parent. Only one revision of each child is
// transferred, instead of all of them.
func (d *DescendantsGremlinTraversalStep) childrenLookupFunc(g descendantsGraph, roots []*graph.Node) childrenLookup {
	direct := func(parent *graph.Node) []*graph.Node {
		return g.LookupChildren(parent, nil, d.edgeFilter)
	}

	ctx := g.GetContext()
	if ctx.TimeSlice == nil || ctx.TimePoint {
		return direct
	}

	children, ok := d.prefetchChildren(g, roots)
	if !ok {
		return direct
	}

	return func(parent *graph.Node) []*graph.Node {
		if c, found := children[parent.ID]; found {
			return c
		}
		return direct(parent)
	}
}

// prefetchChildren walks the descendants level by level, up to maxDepth, and
// returns the children of each walked node ID, in the order LookupChildren
// returns them, with one revision per child. ok is false if the backend can not
// return one revision per node; the caller must use LookupChildren.
func (d *DescendantsGremlinTraversalStep) prefetchChildren(g descendantsGraph, roots []*graph.Node) (map[graph.Identifier][]*graph.Node, bool) {
	children := make(map[graph.Identifier][]*graph.Node)
	nodesByID := make(map[graph.Identifier]*graph.Node)
	requestedNodes := make(map[graph.Identifier]bool)
	queued := make(map[graph.Identifier]bool)

	var level []graph.Identifier
	for _, n := range roots {
		if !queued[n.ID] {
			queued[n.ID] = true
			level = append(level, n.ID)
		}
	}

	for depth := int64(0); len(level) > 0 && (d.maxDepth == 0 || depth < d.maxDepth); depth++ {
		edgesByParent := make(map[graph.Identifier][]*graph.Edge)
		for start := 0; start < len(level); start += descendantsEdgesBatchSize {
			end := start + descendantsEdgesBatchSize
			if end > len(level) {
				end = len(level)
			}
			d.fetchChildEdges(g, level[start:end], edgesByParent)
		}

		var missing []graph.Identifier
		for _, id := range level {
			for _, e := range edgesByParent[id] {
				if !requestedNodes[e.Child] {
					requestedNodes[e.Child] = true
					missing = append(missing, e.Child)
				}
			}
		}

		for start := 0; start < len(missing); start += descendantsNodesBatchSize {
			end := start + descendantsNodesBatchSize
			if end > len(missing) {
				end = len(missing)
			}
			nodes, ok := g.GetNodesOneRevision(missing[start:end])
			if !ok {
				return nil, false
			}
			for _, n := range nodes {
				nodesByID[n.ID] = n
			}
		}

		var next []graph.Identifier
		for _, id := range level {
			list := []*graph.Node{}
			for _, e := range edgesByParent[id] {
				child, found := nodesByID[e.Child]
				if !found {
					// no revision of the child within the time slice
					continue
				}
				list = append(list, child)
				if !queued[child.ID] {
					queued[child.ID] = true
					next = append(next, child.ID)
				}
			}
			children[id] = list
		}
		level = next
	}

	return children, true
}

// fetchChildEdges adds to edgesByParent the edges, matching the step edge
// filter, whose parent is one of ids, keeping the backend order. If the
// request may have been truncated by the backend search limit, the batch is
// split in two, down to a single node, which is the same request
// LookupChildren makes.
func (d *DescendantsGremlinTraversalStep) fetchChildEdges(g descendantsGraph, ids []graph.Identifier, edgesByParent map[graph.Identifier][]*graph.Edge) {
	nodes := make([]*graph.Node, 0, len(ids))
	inBatch := make(map[graph.Identifier]bool, len(ids))
	for _, id := range ids {
		nodes = append(nodes, graph.CreateNode(id, graph.Metadata{}, graph.Unix(0, 0), "", ""))
		inBatch[id] = true
	}

	edges := g.GetNodesEdges(nodes, d.edgeFilter)
	if len(edges) >= descendantsMaxSearchHits && len(ids) > 1 {
		half := len(ids) / 2
		d.fetchChildEdges(g, ids[:half], edgesByParent)
		d.fetchChildEdges(g, ids[half:], edgesByParent)
		return
	}

	for _, e := range edges {
		if inBatch[e.Parent] {
			edgesByParent[e.Parent] = append(edgesByParent[e.Parent], e)
		}
	}
}

// Exec Descendants step
func (d *DescendantsGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	var descendants []*graph.Node

	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		tv.GraphTraversal.RLock()
		roots := tv.GetNodes()
		lookup := d.childrenLookupFunc(tv.GraphTraversal.Graph, roots)
		getDescendants(lookup, roots, &descendants, 0, d.maxDepth, make(map[graph.Identifier]bool))
		tv.GraphTraversal.RUnlock()

		return traversal.NewGraphTraversalV(tv.GraphTraversal, descendants), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce Descendants step
func (d *DescendantsGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context Descendants step
func (d *DescendantsGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &d.context
}
