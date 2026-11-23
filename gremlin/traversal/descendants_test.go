package traversal

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/stretchr/testify/assert"
)

// fakeHistoryGraph simulates a history backend (Elasticsearch) with several
// revisions per node and per edge. Edges are kept in the backend sort order
// (UpdatedAt). Every search returns at most maxHits documents.
type fakeHistoryGraph struct {
	ctx      graph.Context
	nodes    map[graph.Identifier][]*graph.Node // revisions in Revision order
	edges    []*graph.Edge
	maxHits  int
	searches int
	lookups  int
}

func (f *fakeHistoryGraph) GetContext() graph.Context {
	return f.ctx
}

// LookupChildren mimics graph.LookupChildren over the Elasticsearch backend:
// one GetNodeEdges search, then GetNode(child) for each edge of the node as parent
func (f *fakeHistoryGraph) LookupChildren(n *graph.Node, fm graph.ElementMatcher, em graph.ElementMatcher) (nodes []*graph.Node) {
	f.lookups++
	f.searches++
	var edges []*graph.Edge
	for _, e := range f.edges {
		if e.Parent == n.ID || e.Child == n.ID {
			edges = append(edges, e)
		}
	}
	if len(edges) > f.maxHits {
		edges = edges[:f.maxHits]
	}
	for _, e := range edges {
		if e.Parent == n.ID {
			f.searches += 2 // GetNode(parent) and GetNode(child)
			nodes = append(nodes, f.nodes[e.Child]...)
		}
	}
	return nodes
}

func (f *fakeHistoryGraph) GetNodesEdges(ns []*graph.Node, m graph.ElementMatcher) (edges []*graph.Edge) {
	f.searches++
	ids := make(map[graph.Identifier]bool)
	for _, n := range ns {
		ids[n.ID] = true
	}
	for _, e := range f.edges {
		if ids[e.Parent] || ids[e.Child] {
			edges = append(edges, e)
		}
	}
	if len(edges) > f.maxHits {
		edges = edges[:f.maxHits]
	}
	return edges
}

func (f *fakeHistoryGraph) GetNodesOneRevision(ids []graph.Identifier) ([]*graph.Node, bool) {
	f.searches++
	var nodes []*graph.Node
	for _, id := range ids {
		if revs := f.nodes[id]; len(revs) > 0 {
			nodes = append(nodes, revs[0])
		}
	}
	return nodes, true
}

func newRevision(id graph.Identifier, rev int64) *graph.Node {
	n := graph.CreateNode(id, graph.Metadata{"Rev": rev}, graph.Unix(0, 0), "", "")
	n.Revision = rev
	return n
}

// newFakeHistoryGraph builds servers -> software -> sub-software, with several
// revisions per node and edge, children shared by several parents, children
// without revisions in the time slice and edges in a random backend order
func newFakeHistoryGraph(seed int64) (*fakeHistoryGraph, []*graph.Node) {
	r := rand.New(rand.NewSource(seed))
	f := &fakeHistoryGraph{
		ctx:     graph.Context{TimeSlice: graph.NewTimeSlice(0, 1000)},
		nodes:   make(map[graph.Identifier][]*graph.Node),
		maxHits: 10000,
	}

	addNode := func(id graph.Identifier) {
		revs := r.Intn(4) // 0 = no revision within the time slice
		for i := 1; i <= revs; i++ {
			f.nodes[id] = append(f.nodes[id], newRevision(id, int64(i*10+r.Intn(5))))
		}
	}
	edgeSeq := 0
	addEdge := func(parent, child graph.Identifier) {
		edgeSeq++
		for i := 1; i <= 1+r.Intn(3); i++ {
			f.edges = append(f.edges, &graph.Edge{Parent: parent, Child: child})
			f.edges[len(f.edges)-1].ID = graph.Identifier(fmt.Sprintf("e%d", edgeSeq))
			f.edges[len(f.edges)-1].Revision = int64(i)
		}
	}

	var roots []*graph.Node
	for s := 0; s < 40; s++ {
		server := graph.Identifier(fmt.Sprintf("server%d", s))
		f.nodes[server] = []*graph.Node{newRevision(server, 1), newRevision(server, 2)}
		roots = append(roots, f.nodes[server]...) // the previous step can return several revisions
		for c := 0; c < r.Intn(6); c++ {
			sw := graph.Identifier(fmt.Sprintf("sw%d-%d", s, c))
			addNode(sw)
			addEdge(server, sw)
			if r.Intn(4) == 0 { // software shared with another server
				addEdge(graph.Identifier(fmt.Sprintf("server%d", r.Intn(40))), sw)
			}
			for gc := 0; gc < r.Intn(3); gc++ {
				sub := graph.Identifier(fmt.Sprintf("sub%d-%d-%d", s, c, gc))
				addNode(sub)
				addEdge(sw, sub)
			}
		}
		// an edge where the server is the child: never a descendant
		addEdge(graph.Identifier(fmt.Sprintf("rack%d", s%3)), server)
	}
	r.Shuffle(len(f.edges), func(i, j int) { f.edges[i], f.edges[j] = f.edges[j], f.edges[i] })
	r.Shuffle(len(roots), func(i, j int) { roots[i], roots[j] = roots[j], roots[i] })

	return f, roots
}

func revisionsOf(nodes []*graph.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, fmt.Sprintf("%s@%d", n.ID, n.Revision))
	}
	return out
}

func runDescendants(f *fakeHistoryGraph, roots []*graph.Node, maxDepth int64, batched bool) []*graph.Node {
	d := &DescendantsGremlinTraversalStep{maxDepth: maxDepth}
	lookup := func(parent *graph.Node) []*graph.Node { return f.LookupChildren(parent, nil, nil) }
	if batched {
		lookup = d.childrenLookupFunc(f, roots)
	}
	var descendants []*graph.Node
	getDescendants(lookup, roots, &descendants, 0, maxDepth, make(map[graph.Identifier]bool))
	return descendants
}

// TestDescendantsBatchedSameResult checks the batched history path returns
// exactly the same nodes, revisions and order as one LookupChildren per node
func TestDescendantsBatchedSameResult(t *testing.T) {
	for _, maxHits := range []int{10000, 7} {
		for _, maxDepth := range []int64{1, 2, 0} {
			for seed := int64(1); seed <= 20; seed++ {
				name := fmt.Sprintf("maxHits=%d/depth=%d/seed=%d", maxHits, maxDepth, seed)

				f, roots := newFakeHistoryGraph(seed)
				f.maxHits = maxHits
				saved := descendantsMaxSearchHits
				descendantsMaxSearchHits = maxHits

				expected := revisionsOf(runDescendants(f, roots, maxDepth, false))
				directSearches := f.searches
				f.searches, f.lookups = 0, 0
				got := revisionsOf(runDescendants(f, roots, maxDepth, true))

				descendantsMaxSearchHits = saved

				assert.Equal(t, expected, got, name)
				assert.Equal(t, 0, f.lookups, name+": the batched path must not call LookupChildren")
				if maxHits == 10000 {
					assert.Less(t, f.searches, directSearches/10, name+": far fewer backend searches")
				}
			}
		}
	}
}

// TestDescendantsDirectLookupOutsideTimeSlice checks live queries and time
// points keep using LookupChildren
func TestDescendantsDirectLookupOutsideTimeSlice(t *testing.T) {
	for _, ctx := range []graph.Context{
		{},
		{TimeSlice: graph.NewTimeSlice(1000, 1000), TimePoint: true},
	} {
		f, roots := newFakeHistoryGraph(1)
		f.ctx = ctx
		expected := revisionsOf(runDescendants(f, roots, 1, false))
		f.lookups = 0
		got := revisionsOf(runDescendants(f, roots, 1, true))
		assert.Equal(t, expected, got)
		assert.Greater(t, f.lookups, 0)
	}
}

// noOneRevisionGraph is a backend that can not return one revision per node
type noOneRevisionGraph struct {
	*fakeHistoryGraph
}

func (f *noOneRevisionGraph) GetNodesOneRevision(ids []graph.Identifier) ([]*graph.Node, bool) {
	return nil, false
}

// TestDescendantsFallbackWithoutOneRevision checks the step falls back to
// LookupChildren when the backend does not support GetNodesOneRevision
func TestDescendantsFallbackWithoutOneRevision(t *testing.T) {
	f, roots := newFakeHistoryGraph(3)
	expected := revisionsOf(runDescendants(f, roots, 1, false))

	g := &noOneRevisionGraph{f}
	d := &DescendantsGremlinTraversalStep{maxDepth: 1}
	f.lookups = 0
	var descendants []*graph.Node
	getDescendants(d.childrenLookupFunc(g, roots), roots, &descendants, 0, 1, make(map[graph.Identifier]bool))

	assert.Equal(t, expected, revisionsOf(descendants))
	assert.Greater(t, f.lookups, 0)
}
