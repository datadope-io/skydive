package traversal

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

// BuildingTraversalExtension describes a new extension to enhance the topology
type BuildingTraversalExtension struct {
	BuildingToken traversal.Token
}

// BuildingGremlinTraversalStep is a ad-hoc step to show devices belonging to a building, with
// its interfaces and the nodes connected to them.
// The model used for this step should be:
// [building]----(ownership)----[device]----(ownership)----[interface]----(ownership)----[logical_interface]
// [interface|logical_interface]----(connect/pseudowire)----[interface|logical_interface]
// This step should be used directly after the "G" step. Eg.: G.Building("building_name")
type BuildingGremlinTraversalStep struct {
	context  traversal.GremlinTraversalContext
	building string
	// edgeFilter list of RelationType to move between interfaces of different buildings
	edgeFilter []string
	// nextStepOnlyIDs is set to true if the next step only needs node IDs and not the whole node info
	nextStepOnlyIDs bool
}

// NewBuildingTraversalExtension returns a new graph traversal extension
func NewBuildingTraversalExtension() *BuildingTraversalExtension {
	return &BuildingTraversalExtension{
		BuildingToken: traversalBuildingToken,
	}
}

// ScanIdent returns an associated graph token
func (e *BuildingTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "BUILDING":
		return e.BuildingToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses Building step
func (e *BuildingTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.BuildingToken:
	default:
		return nil, nil
	}

	if len(p.Params) != 2 {
		return nil, fmt.Errorf("Building step requires 2 parameters, name and within filter")
	}

	building, ok := p.Params[0].(string)
	if !ok {
		return nil, errors.New("Building first argument must be the building name as a string")
	}

	edgeFilterRaw, ok := p.Params[1].(*traversal.WithinElementMatcher)
	if !ok {
		return nil, errors.New("Building second argument must be the a within filter with the RelationType to move between interfaces of different buildings")
	}

	// Convert an slice of interface{} to a slice of string
	edgeFilter := make([]string, 0, len(edgeFilterRaw.List))
	for _, e := range edgeFilterRaw.List {
		eStr, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("edge filter must be a list of strings")
		}
		edgeFilter = append(edgeFilter, eStr)
	}

	return &BuildingGremlinTraversalStep{
		context:    p,
		building:   building,
		edgeFilter: edgeFilter,
	}, nil
}

// getChildIDsFromNode returns the IDs of the children of the given node (nodes connected with an ownership edge)
func (d *BuildingGremlinTraversalStep) getChildIDsFromNode(g *graph.Graph, n *graph.Node) []graph.Identifier {
	ids := []graph.Identifier{}

	edges := g.GetNodeEdges(n, graph.Metadata{"RelationType": "ownership"})
	for _, e := range edges {
		// Only append if the node is the parent of the edge
		if e.Parent == n.ID {
			ids = append(ids, e.Child)
		}
	}

	return ids
}

// getChildIDsFromIDs given a list of node ids, returns the list of node ids of the children of those nodes
func (d *BuildingGremlinTraversalStep) getChildIDsFromIDs(g *graph.Graph, sourceIDs []graph.Identifier) []graph.Identifier {
	// Store source IDs to filter edges and only keep the ones that are connected to the source nodes as parents
	sourceIDsMap := make(map[graph.Identifier]interface{})

	// Convert the list of node ids to a list of nodes. Needed by g.GetNodeEdges. This is a bit of a hack.
	// g.GetNodeEdges only accepts a list of nodes, not a list of ids, but it only uses the id.
	nodes := make([]*graph.Node, 0, len(sourceIDs))
	for _, nID := range sourceIDs {
		sourceIDsMap[nID] = nil
		nodes = append(nodes, graph.CreateNode(nID, graph.Metadata{}, graph.Unix(0, 0), "", ""))
	}

	ids := []graph.Identifier{}
	edges := g.GetNodesEdges(nodes, graph.Metadata{"RelationType": "ownership"})
	for _, e := range edges {
		// Only append if the node is the parent of the edge
		if _, ok := sourceIDsMap[e.Parent]; ok {
			ids = append(ids, e.Child)
		}
	}

	return ids
}

// getParentIDsFromIDs given a list of node ids, returns the list of node ids of the parents of those nodes
func (d *BuildingGremlinTraversalStep) getParentIDsFromIDs(g *graph.Graph, sourceIDs []graph.Identifier) []graph.Identifier {
	// Store source IDs to filter edges and only keep the ones that are connected to the source nodes as children
	sourceIDsMap := make(map[graph.Identifier]interface{})

	// Convert the list of node ids to a list of nodes. Needed by g.GetNodeEdges. This is a bit of a hack.
	// g.GetNodeEdges only accepts a list of nodes, not a list of ids, but it only uses the id.
	nodes := make([]*graph.Node, 0, len(sourceIDs))
	for _, nID := range sourceIDs {
		sourceIDsMap[nID] = nil
		nodes = append(nodes, graph.CreateNode(nID, graph.Metadata{}, graph.Unix(0, 0), "", ""))
	}

	ids := []graph.Identifier{}
	edges := g.GetNodesEdges(nodes, graph.Metadata{"RelationType": "ownership"})
	for _, e := range edges {
		// Only append if the node is the child of the edge and the parent is not in the source list
		if _, ok := sourceIDsMap[e.Child]; ok {
			if _, ok := sourceIDsMap[e.Parent]; !ok {
				ids = append(ids, e.Parent)
			}
		}
	}

	return ids
}

// getConnectedNodesFromIDs given a list of node ids, returns the list of node ids of the nodes connected to those nodes,
// but only the edges which have a RelationType on the relationTypes list.
func (d *BuildingGremlinTraversalStep) getConnectedNodesFromIDs(g *graph.Graph, sourceIDs []graph.Identifier, relationTypes []string) []graph.Identifier {
	// Store source IDs to filter edges and only keep the ones that are not in this list
	sourceIDsMap := make(map[graph.Identifier]interface{})

	// Convert the list of node ids to a list of nodes. Needed by g.GetNodeEdges. This is a bit of a hack.
	// g.GetNodeEdges only accepts a list of nodes, not a list of ids, but it only uses the id.
	nodes := make([]*graph.Node, 0, len(sourceIDs))
	for _, nID := range sourceIDs {
		sourceIDsMap[nID] = nil
		nodes = append(nodes, graph.CreateNode(nID, graph.Metadata{}, graph.Unix(0, 0), "", ""))
	}

	relationTypeFilter := []*filters.Filter{}
	for _, rt := range relationTypes {
		relationTypeFilter = append(relationTypeFilter, filters.NewTermStringFilter("RelationType", rt))
	}

	ids := []graph.Identifier{}
	edges := g.GetNodesEdges(nodes, graph.NewElementFilter(filters.NewOrFilter(relationTypeFilter...)))
	for _, e := range edges {
		// Only append the remote node
		if _, ok := sourceIDsMap[e.Parent]; ok {
			ids = append(ids, e.Child)
		} else if _, ok := sourceIDsMap[e.Child]; ok {
			ids = append(ids, e.Parent)
		}
	}

	return ids
}

// getBuilding return devices and interfaces connected to the given building.
// Also return devices and interfaces connected to those in the building.
func (d *BuildingGremlinTraversalStep) getBuilding(g *graph.Graph) ([]*graph.Node, error) {
	// Get the building node
	buildings := g.GetNodes(graph.Metadata{"Name": d.building})
	if len(buildings) == 0 {
		return nil, nil
	}

	building := buildings[0]

	// Get the ownership edges from the building node (devices in that building)
	devicesIDs := d.getChildIDsFromNode(g, building)

	// Get the interfaces of all devices in the building
	interfacesIDs := d.getChildIDsFromIDs(g, devicesIDs)

	// Get also the logical interfaces (those are connected to the interfaces)
	interfacesIDs = append(interfacesIDs, d.getChildIDsFromIDs(g, interfacesIDs)...)

	// Get the nodes connected to the interfaces, only those connected with 'connect' or 'pseudowire' edges
	remoteNodesIDs := d.getConnectedNodesFromIDs(g, interfacesIDs, d.edgeFilter)

	// Get the interfaces or devices of those remote IDs.
	// We don't know if we have reached a physical/aggregation interface or a logical one.
	// Physical/aggregation are connected directly to devices, so one step "up" is enough.
	// But logical interfaces are connected to interfaces, so we need to go two steps "up".
	// Going two steps up from a phy/agg interface will reach the building.
	remoteNodesIDs = append(remoteNodesIDs, d.getParentIDsFromIDs(g, remoteNodesIDs)...)
	remoteNodesIDs = append(remoteNodesIDs, d.getParentIDsFromIDs(g, remoteNodesIDs)...)

	// Now resolve all IDs to nodes
	// Append devices, interfaces and remoteNodesIDs into the same list
	nodesIDs := append(devicesIDs, interfacesIDs...)
	nodesIDs = append(nodesIDs, remoteNodesIDs...)

	nodes := []*graph.Node{}

	// If the next step only needs the nodes IDs, create fake nodes just with that ID.
	// Otherwise, make a call to get the actual nodes from the graph.
	if d.nextStepOnlyIDs {
		for _, id := range nodesIDs {
			nodes = append(nodes, graph.CreateNode(id, graph.Metadata{}, graph.Unix(0, 0), "", ""))
		}
	} else {
		for _, n := range g.GetNodesFromIDs(nodesIDs) {
			if n.Metadata["Type"] == "device" || n.Metadata["Type"] == "interface" {
				nodes = append(nodes, n)
			}
		}
	}

	return nodes, nil
}

// Exec Building step
func (d *BuildingGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	tv, ok := last.(*traversal.GraphTraversal)
	if !ok {
		return nil, traversal.ErrExecutionError
	}

	tv.RLock()
	nodes, err := d.getBuilding(tv.Graph)
	tv.RUnlock()

	if err != nil {
		return nil, err
	}

	return traversal.NewGraphTraversalV(tv, nodes), nil
}

// Reduce Descendants step
func (d *BuildingGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	// Merge step only needs the ids of nodes. In that case, do not resolve node IDs to actual nodes.
	if _, ok := next.(*MergeGremlinTraversalStep); ok {
		d.nextStepOnlyIDs = true
	}
	return next, nil
}

// Context Building step
func (d *BuildingGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &d.context
}
