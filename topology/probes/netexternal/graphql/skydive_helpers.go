package graphql

import (
	"fmt"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/netexternal/graphql/model"
)

const (
	// PrefixOriginName defines the prefix set in nodes/edges Origin field
	PrefixOriginName = "netexternal."
	// TypeRouter is the Metadata.Type value used for routers
	TypeRouter = "router"
	// TypeSwitch is the Metadata.Type value used for switches
	TypeSwitch = "switch"
	// TypeNetwork is the Metadata.Type value used for networks
	TypeNetwork = "network"
	// TypeVRF is the Metadata.Type value used for VRFs
	TypeVRF = "vrf"
	// TypeInterface is the Metadata.Type value used for interfaces
	TypeInterface = "interface"
	// TypeVLAN is the Metadata.Type value used for VLANs
	TypeVLAN = "vlan"
	// TypeIP is the Metadata.Type value used for IPs
	TypeIP = "ip"
	// TypeServer is the Metadata.Type value used for server
	TypeServer = "server"

	// RelationTypeLayer2 is the edge type between interfaces and switches/routers and interfaces
	RelationTypeLayer2 = "layer2"

	// MetadataRelationTypeKey key name to store the type of an edge
	MetadataRelationTypeKey = "RelationType"
	// MetadataTypeKey key name to store the type of node in node's metadata
	MetadataTypeKey = "Type"
	// MetadataNameKey key name to store the name of the server in node's metadata
	MetadataNameKey = "Name"
	// MetadataVIDKey key name where VLAN ID is stored in node metadata
	MetadataVIDKey = "VID"
	// MetadataNativeVLANKey key to store the Native VLAN ID in the switch metadata
	MetadataNativeVLANKey = "NativeVLAN"
	// MetadataMACKey key to store MAC address for interfaces or switch
	MetadataMACKey = "MAC"
	// MetadataVendorKey to store vendor information for switches or routers
	MetadataVendorKey = "Vendor"
	// MetadataModelKey  store model information for switches or routers
	MetadataModelKey = "Model"
)

// newEdge create a new edge in Skydive with a custom Origin field
func (r *Resolver) newEdge(n *graph.Node, c *graph.Node, m graph.Metadata) (*graph.Edge, error) {
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(n.ID+c.ID))
	i := graph.Identifier(u.String())

	e := graph.CreateEdge(i, n, c, m, graph.TimeUTC(), r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// newNode create a new edge in Skydive with a custom Origin field
func (r *Resolver) newNode(m graph.Metadata) (*graph.Node, error) {
	i := graph.GenID()
	n := graph.CreateNode(i, m, graph.TimeUTC(), r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

// addVLANsToInterface for each VLAN add a link of type "Mode" to each
// of the VLANs defined in the "VID" array.
func (r *Resolver) addVLANsToInterface(iface *graph.Node, vlan model.InterfaceVLANInput) error {
	for _, vid := range vlan.Vid {
		// Create or get VLAN node
		vlanNode, err := r.createVLAN(vid, "")
		if err != nil {
			return err
		}

		// Link interface to VLAN, if they are not already linked
		vlanEdgeFilter := graph.NewElementFilter(
			filters.NewTermStringFilter(MetadataRelationTypeKey, string(vlan.Mode)),
		)
		if !r.Graph.AreLinked(vlanNode, iface, vlanEdgeFilter) {
			_, err = r.newEdge(vlanNode, iface, map[string]interface{}{
				MetadataRelationTypeKey: string(vlan.Mode),
			})
			if err != nil {
				return fmt.Errorf("unable to link VLAN (%d) to interface (%+v): %v", vid, iface, err)
			}
		}
	}

	return nil
}

// createVLAN add a new node to Skydive to represent a VLAN.
// If a VLAN with this VID already exists return the node.
// Sets a default name if "name" param is not defined
func (r *Resolver) createVLAN(vid int, name string) (*graph.Node, error) {
	if name == "" {
		name = fmt.Sprintf("VLAN:%d", vid)
	}

	metadata := map[string]interface{}{
		MetadataNameKey: name,
		MetadataTypeKey: TypeVLAN,
		MetadataVIDKey:  vid,
	}

	// Find VLAN node with matching VID+Type
	nodes := r.Graph.GetNodes(graph.Metadata{
		MetadataTypeKey: TypeVLAN,
		MetadataVIDKey:  vid,
	})

	var node *graph.Node
	var err error

	// Create, if it does not exists.
	// Return internal ID if it exists.
	// Error if there more than one node matching
	if len(nodes) == 0 {
		node, err = r.newNode(metadata)
		if err != nil {
			return nil, err
		}
	} else if len(nodes) == 1 {
		newName, err := nodes[0].GetFieldString(MetadataNameKey)
		if err != nil {
			logging.GetLogger().Warningf("unable to get VLAN name: %+v", nodes[0])
		}
		if newName != name {
			logging.GetLogger().Warningf("stored VLAN name differs from this VLAN name: %s != %s", newName, name)
		}

		node = nodes[0]
	} else {
		logging.GetLogger().Errorf("more than one VLAN nodes with same pkey: %+v", nodes)
		return nil, fmt.Errorf("%d VLANs nodes with same pkey already exists", len(nodes))
	}

	return node, nil
}

// createInterface add a new node to Skydive to represent a interface.
// This node is also linked with the node using a layer2 edge.
// Return if any interface has been updated.
func (r *Resolver) createInterfaces(node *graph.Node, interfaces []*model.InterfaceInput) (updated bool, err error) {
	// Get currently connected interfaces
	// From the node, get layer2 edges.
	// From that edges, get only nodes type interface
	layer2Filter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeLayer2),
	)
	interfaceFilter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataTypeKey, TypeInterface),
	)

	layer2Edges := r.Graph.GetNodeEdges(node, layer2Filter)
	// currentInterfaces create an index of current node interfaces using its name
	var currentInterfaces map[string]*graph.Node
	for _, e := range layer2Edges {
		var edgeNodes []*graph.Node
		if e.Child == node.ID {
			edgeNodes, _ = r.Graph.GetEdgeNodes(e, interfaceFilter, nil)
		} else {
			_, edgeNodes = r.Graph.GetEdgeNodes(e, nil, interfaceFilter)
		}
		for _, n := range edgeNodes {
			ifaceName, err := n.GetFieldString(MetadataNameKey)
			if err != nil {
				logging.GetLogger().Errorf("interface without Metadata.Name: %v", n)
				continue
			}
			currentInterfaces[ifaceName] = n
		}
	}

	// Iterate over user defined interfaces, creating or updating while needed
	for _, userIface := range interfaces {
		ifaceMetadata := map[string]interface{}{
			MetadataNameKey: userIface.Name,
			MetadataTypeKey: TypeInterface,
		}

		// Add the Native VLAN if defined
		if userIface.Vlan != nil && userIface.Vlan.NativeVid != nil {
			ifaceMetadata[MetadataNativeVLANKey] = userIface.Vlan.NativeVid
		}

		iface, ok := currentInterfaces[userIface.Name]
		if ok {
			revisionPreUpdateIface := iface.Revision
			err := r.Graph.SetMetadata(iface, ifaceMetadata)
			if err != nil {
				logging.GetLogger().Errorf("unable to update interface metadata %+v: %v", iface, err)
				return updated, fmt.Errorf("unable to update interface metadata %+v: %v", iface, err)
			}
			if revisionPreUpdateIface != iface.Revision {
				updated = true
			}
		} else {
			// Create the interface and assign the node to "iface" var
			iface, err = r.newNode(ifaceMetadata)
			if err != nil {
				logging.GetLogger().Errorf("unable to create interface %+v: %v", ifaceMetadata, err)
				return updated, fmt.Errorf("unable to create interface: %v", err)
			}
		}

		// Link interface with node
		if !r.Graph.AreLinked(node, iface, layer2Filter) {
			_, err = r.newEdge(node, iface, map[string]interface{}{
				MetadataRelationTypeKey: RelationTypeLayer2,
			})
			if err != nil {
				return updated, fmt.Errorf("unable to link node (%+v) to interface (%+v): %v", node, iface, err)
			}
		}

		// Add VLANs if defined
		if userIface.Vlan != nil {
			err := r.addVLANsToInterface(iface, *userIface.Vlan)
			if err != nil {
				return updated, err
			}
		}

		// Add IP if defined
		if userIface.IP != nil {
			err := r.addIPToInterface(iface, *userIface.IP)
			if err != nil {
				return updated, err
			}
		}
	}

	return updated, nil
}

// addNodeWithInterfaces create a node with the data in "metadata" parameter and using
// "nodePKey" as the metadata to find if the node already exists.
// Once the node is created, add the interfaces.
// Return the node created.
// Return updated true if the node metadata has been updated.
// Return interfaceUpdated true if any of the interfaces has been updated.
func (r *Resolver) addNodeWithInterfaces(
	metadata graph.Metadata,
	interfaces []*model.InterfaceInput,
	nodePKey graph.Metadata,
) (node *graph.Node, updated bool, interfaceUpdated bool, err error) {
	// Find switch node with matching Name+Type
	nodes := r.Graph.GetNodes(nodePKey)

	// Create, if it does not exists.
	// Return internal ID if it exists.
	// Error if there more than one node matching
	if len(nodes) == 0 {
		node, err = r.newNode(metadata)
		if err != nil {
			return node, updated, interfaceUpdated, err
		}
	} else if len(nodes) == 1 {
		// Update metadata if needed
		node = nodes[0]
		revisionPreUpdate := node.Revision

		err = r.Graph.SetMetadata(node, metadata)
		if err != nil {
			logging.GetLogger().Errorf("unable to update node %+v: %v", node, err)
			return node, updated, interfaceUpdated, fmt.Errorf("error updating node: %v", err)
		}
		// If SetMetadata modifies metadata, it will increase revision number.
		// We use this to know if switch metadata has been modified and notify the user.
		if revisionPreUpdate != node.Revision {
			updated = true
		}
	} else if len(nodes) != 1 {
		logging.GetLogger().Errorf("more than one switch nodes with same pkey: %+v", nodes)
		return node, updated, interfaceUpdated, fmt.Errorf("%d switch nodes with same pkey already exists", len(nodes))
	}

	// Interfaces
	interfaceUpdated, err = r.createInterfaces(node, interfaces)

	return node, updated, interfaceUpdated, nil
}

// addIPToInterface add to the interface the IP configuration (IP + mask).
// Create the network if it does not exists yet.
func (r *Resolver) addIPToInterface(iface *graph.Node, vlan model.InterfaceIPInput) error {
	// TODO por aqui
	return nil
}
