package traversal

import (
	"errors"
	"strings"
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
)

// failingHistoryBackend is a backend with history whose persistent requests
// fail, as Elasticsearch does when it is down or overloaded
type failingHistoryBackend struct {
	*graph.MemoryBackend
}

func (b *failingHistoryBackend) IsHistorySupported() bool {
	return true
}

func (b *failingHistoryBackend) GetNodes(t graph.Context, m graph.ElementMatcher) []*graph.Node {
	if t.TimeSlice != nil {
		t.Errors.Add(errors.New("elasticsearch: connection refused"))
		return nil
	}
	return b.MemoryBackend.GetNodes(t, m)
}

func execGremlin(t *testing.T, g *graph.Graph, query string) (GraphTraversalStep, error) {
	ts, err := NewGremlinTraversalParser().Parse(strings.NewReader(query))
	if err != nil {
		t.Fatalf("parse %s: %v", query, err)
	}
	return ts.Exec(g, false)
}

// TestHistoryBackendErrorFailsQuery checks a history query fails when a
// persistent backend request fails, instead of returning an empty result
func TestHistoryBackendErrorFailsQuery(t *testing.T) {
	memory, err := graph.NewMemoryBackend()
	if err != nil {
		t.Fatal(err)
	}
	g := graph.NewGraph("host1", &failingHistoryBackend{memory}, "test")
	g.NewNode(graph.GenID(), graph.Metadata{"Type": "Server"}, "host1")

	_, err = execGremlin(t, g, "G.At(1479899809,3600).V().Has('Type','Server').Count()")
	if !errors.Is(err, graph.ErrBackendQuery) {
		t.Fatalf("expected ErrBackendQuery, got %v", err)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("the error must include the backend error: %v", err)
	}

	// live queries do not use the persistent backend
	res, err := execGremlin(t, g, "G.V().Has('Type','Server').Count()")
	if err != nil {
		t.Fatalf("live query: %v", err)
	}
	if v := res.Values(); len(v) != 1 || v[0] != 1 {
		t.Fatalf("live query: expected 1, got %v", v)
	}
}
