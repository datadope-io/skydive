package graphql

import (
	"fmt"
	"strings"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
)

const (
	// PrefixOriginName defines the prefix set in nodes/edges Origin field
	PrefixOriginName = "graphql."
)

// newNode create a new node in Skydive with a custom createdAt and origin field
func (r *Resolver) newNode(id graph.Identifier, m graph.Metadata, createdAt *time.Time) (*graph.Node, error) {
	var timestamp graph.Time
	if createdAt != nil {
		timestamp = graph.Time(*createdAt)
	} else {
		timestamp = graph.TimeUTC()
	}

	n := graph.CreateNode(id, m, timestamp, r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

// newEdge create a new edge in Skydive with a custom createdAt and origin field
func (r *Resolver) newEdge(id graph.Identifier, srcNode, dstNode *graph.Node, m graph.Metadata, createdAt *time.Time) (*graph.Edge, error) {
	var timestamp graph.Time
	if createdAt != nil {
		timestamp = graph.Time(*createdAt)
	} else {
		timestamp = graph.TimeUTC()
	}
	e := graph.CreateEdge(id, srcNode, dstNode, m, timestamp, r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// getNodeFromGremlinQuery return a single node from a gremlin query
func (r *Resolver) getNodeFromGremlinQuery(query string) (*graph.Node, error) {
	tr := traversal.NewGremlinTraversalParser()

	// Add gremlin extensions, to be able to use the following steps:
	//   - Descendants
	//   - Ascendants
	//   - Neighbors
	//   - Building
	// As seen in analyzer/server.go
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())
	tr.AddTraversalExtension(ge.NewAscendantsTraversalExtension())
	tr.AddTraversalExtension(ge.NewNeighborsTraversalExtension())
	tr.AddTraversalExtension(ge.NewBuildingTraversalExtension())

	ts, err := tr.Parse(strings.NewReader(query))
	if err != nil {
		return nil, fmt.Errorf("parsing gremlin query: %v", err)
	}

	// Execute the gremlin query
	res, err := ts.Exec(r.Graph, false)
	if err != nil {
		return nil, fmt.Errorf("executing gremlin query: %v", err)
	}

	// Assume the gremlin query is outputting a list of nodes Fail otherwise
	tv, ok := res.(*traversal.GraphTraversalV)
	if !ok {
		return nil, fmt.Errorf("invalid graph output, expecting nodes")
	}

	// Only one node is expected
	nodes := tv.GetNodes()
	if len(nodes) != 1 {
		return nil, fmt.Errorf("expecting only one node, gremlin query returned %d", len(nodes))
	}

	return nodes[0], nil
}

// createOrUpdateNode create the node if it does not exists. Otherwise, update node's metadata.
// createdAt is used to define the creation date of the node.
// Return the node created or updated.
// Return created true if a new node has been created.
// Return updated true if the node already exists and its metadata has been updated.
// Return error if node could not be created or an error happens updating metadata.
func (r *Resolver) createOrUpdateNode(
	id graph.Identifier,
	metadata graph.Metadata,
	createdAt *time.Time,
) (node *graph.Node, created bool, updated bool, err error) {
	node = r.Graph.GetNode(id)
	if node == nil {
		node, err = r.newNode(id, metadata, createdAt)
		if err != nil {
			return node, created, updated, fmt.Errorf("unable to create node (id: %v, metadata: %+v): %v", id, metadata, err)
		}
		created = true
		return node, created, updated, nil
	}

	// Node already exists. Update metadata
	// Do not update device if we are supposed to update CreatedAt value
	if createdAt != nil && !createdAt.Equal(time.Time(node.CreatedAt)) {
		return node, created, updated, fmt.Errorf("forbidden to modify CreatedAt value of existing node")
	}

	// Store revision to know if SetMetadata has modified node's metadata
	revisionPre := node.Revision
	err = r.Graph.SetMetadata(node, metadata)
	if err != nil {
		return node, created, updated, fmt.Errorf("unable to update metadata of node %+v: %v", node, err)
	}
	if revisionPre != node.Revision {
		updated = true
	}

	return node, created, updated, nil
}

// createOrUpdateEdge create the edge if it does not exists. Otherwise, update edge's metadata.
// An edge is considered the same if it has the same source and destination nodes and RelationType.
// createdAt is used to define the creation date of the node.
// Return the edge created or updated.
// Return created true if a new edge has been created.
// Return updated true if the edge already exists and its metadata has been updated.
// Return error if edge could not be created or an error happens updating metadata.
func (r *Resolver) createOrUpdateEdge(
	srcNode *graph.Node,
	dstNode *graph.Node,
	metadata graph.Metadata,
	createdAt *time.Time,
) (edge *graph.Edge, created bool, updated bool, err error) {
	relationType, err := metadata.GetFieldString(MetaKeyRelationType)
	if err != nil {
		return edge, created, updated, fmt.Errorf("missing RelationType: %v", err)
	}

	if srcNode == nil {
		return edge, created, updated, fmt.Errorf("invalid source node")
	}

	if dstNode == nil {
		return edge, created, updated, fmt.Errorf("invalid destination node")
	}

	// Generate the edge interface based on node's ID and Metadata.RelationType
	id := graph.GenID(string(srcNode.ID), string(dstNode.ID), relationType)
	edge = r.Graph.GetEdge(id)
	if edge == nil {
		edge, err = r.newEdge(id, srcNode, dstNode, metadata, createdAt)
		if err != nil {
			return edge, created, updated, fmt.Errorf("unable to create edge type %s, between '%s' and '%s': %v", relationType, srcNode.ID, dstNode.ID, err)
		}
		created = true
		return edge, created, updated, nil
	}

	// Edge already exists. Update metadata
	// Store revision to know if SetMetadata has modified edge's metadata
	revisionPre := edge.Revision
	err = r.Graph.SetMetadata(edge, metadata)
	if err != nil {
		return edge, created, updated, fmt.Errorf("unable to update metadata of edge %+v: %v", edge, err)
	}
	if revisionPre != edge.Revision {
		updated = true
	}

	if createdAt != nil && !createdAt.Equal(time.Time(edge.CreatedAt)) {
		return edge, created, updated, fmt.Errorf("forbidden to modify CreatedAt value of existing edge")
	}
	return edge, created, updated, nil
}
