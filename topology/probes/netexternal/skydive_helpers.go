package netexternal

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

const (
	// PrefixOriginName defines the prefix set in nodes/edges Origin field
	PrefixOriginName = "netexternal."
)

// newEdge create a new edge in Skydive with a custom Origin field
func (r *Resolver) newEdge(n *graph.Node, c *graph.Node, m graph.Metadata, createdAt *time.Time) (*graph.Edge, error) {
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(n.ID+c.ID))
	i := graph.Identifier(u.String())

	var timestamp graph.Time
	if createdAt != nil {
		timestamp = graph.Time(*createdAt)
	} else {
		timestamp = graph.TimeUTC()
	}
	e := graph.CreateEdge(i, n, c, m, timestamp, r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// newNode create a new edge in Skydive with a custom Origin field
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

// getNodeFromGremlinQuery return a single node from a gremlin query
func (r *Resolver) getNodeFromGremlinQuery(query string) (*graph.Node, error) {
	tr := traversal.NewGremlinTraversalParser()
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

func str2GraphID(input string) graph.Identifier {
	hash := sha256.Sum256([]byte(input))
	return graph.Identifier(fmt.Sprintf("%x", hash))
}
