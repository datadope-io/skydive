package netexternal

import (
	"testing"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/stretchr/testify/assert"
)

// TODO test enviar una query incorrecta (que falle el parser)
// TODO enviar query incorrecta (que falle el exec)
// TODO query que devuelve edges u otra cosa, debe dar error
// TODO query que devuelve 0 nodos. Query que devuelve 2 nodos
// TODO error intentando actualizar (que falla UpdateMetadata)
// TODO intentar añadir un evento a ua key que no existe
// TODO comprobar las fechas al crear nuevos elementos y modificarlos
// TODO al modificar elementos se borran todos los valores anteriores y se pisan con los nuevos
// TODO borrar un evento que no existe devuelve un error

func TestGetNodeFromGremlinQuery(t *testing.T) {
	testCases := []struct {
		desc          string
		query         string
		nodes         []*graph.Node
		expectedNode  *graph.Node
		expectedError string
	}{
		{
			desc:          "invalid gremlin query return an error",
			query:         "G.B.C",
			expectedError: "parsing gremlin query: Expected step function, got: B",
		},
		{
			desc:          "query should return nodes",
			query:         "G.E()",
			expectedError: "invalid graph output, expecting nodes",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			g := newGraph(t)
			for _, n := range tC.nodes {
				g.AddNode(n)
			}

			r := Resolver{Graph: g}

			n, err := r.getNodeFromGremlinQuery(tC.query)
			if tC.expectedError != "" {
				assert.EqualError(t, err, tC.expectedError)
			}

			assert.Equal(t, n, tC.expectedNode)
		})
	}
}

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return graph.NewGraph("testhost", b, "analyzer.testhost")
}

// createNode func to create nodes for testing
func createNode(id string, m graph.Metadata) *graph.Node {
	return graph.CreateNode(graph.Identifier(id), m, graph.TimeUTC(), "host", "origin")
}
