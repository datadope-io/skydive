package netexternal

import (
	"fmt"
	"net"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/netexternal/model"
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

	// RelationTypeLayer2 is the edge type between interfaces of differents devices
	// Try to mean the "cables" connecting ports.
	RelationTypeLayer2 = "layer2"
	/*
	* Random names for edge RelationType field trying to explain the different relationships
	* between nodes
	 */
	RelationTypeDefine    = "Define"
	RelationTypeConfigure = "Configure"
	// RelationTypeOwnership is the parent-child relationship for Skydive. Is how skydive knows
	// what nodes could be grouped into its parent.
	RelationTypeOwnership = "Ownership"

	/*
	* Match relation between nodes to the names defined as relation types
	 */
	RelationTypeDeviceInterface = RelationTypeLayer2
	RelationTypeDeviceVRF       = RelationTypeDefine
	RelationTypeVRFNetwork      = RelationTypeConfigure
	RelationTypeInterfaceIP     = RelationTypeDefine
	RelationTypeNetworkIP       = RelationTypeOwnership

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
	// MetadataModelKey store model information for switches or routers
	MetadataModelKey = "Model"
	// MetadataCIDRKey key to store network in format CIDR (eg.: 192.168.2.0/24)
	MetadataCIDRKey = "CIDR"
	// MetadataIPKey key to store IP address
	MetadataIPKey = "IP"
	// MetadataRoutingTableKey is the metadata key used to store routing info avoiding naming
	// collision with the one created by the netlink probe
	MetadataRoutingTableKey = "NetExtRoutingTable"
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

// createIP create a IP node linked to the iface node.
// It also links the IP node to its network node.
// The network is also linked to the VRF node.
// Create the network node if it does not exists.
func (r *Resolver) createIP(ip net.IP, mask net.IPMask, iface *graph.Node, vrfNode *graph.Node) (*graph.Node, error) {
	metadata := map[string]interface{}{
		MetadataNameKey: fmt.Sprintf("IP:%s", ip),
		MetadataTypeKey: TypeIP,
		MetadataIPKey:   ip.String(),
	}

	// Find IP nodes linked to this interface
	ipNodeFilter := graph.NewElementFilter(
		filters.NewAndFilter(
			filters.NewTermStringFilter(MetadataTypeKey, TypeIP),
			filters.NewTermStringFilter(MetadataIPKey, ip.String()),
		))
	ipEdgeFilter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeInterfaceIP),
	)
	nodes := r.Graph.LookupChildren(iface, ipNodeFilter, ipEdgeFilter)

	var ipNode *graph.Node
	var err error

	// Create, if it does not exists.
	// Return internal ID if it exists.
	// Error if there more than one node matching
	if len(nodes) == 0 {
		ipNode, err = r.newNode(metadata)
		if err != nil {
			return nil, err
		}
	} else if len(nodes) == 1 {
		ipNode = nodes[0]
	} else {
		logging.GetLogger().Errorf("more than one IP nodes with same pkey linked to %+v: %+v", iface, nodes)
		return nil, fmt.Errorf("%d IP linked to %+v nodes with same pkey already exists", len(nodes), iface)
	}

	// Link IP to the VRF node
	if !r.Graph.AreLinked(iface, ipNode, ipEdgeFilter) {
		_, err = r.newEdge(iface, ipNode, map[string]interface{}{
			MetadataRelationTypeKey: RelationTypeInterfaceIP,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to link interface (%+v) to IP (%+v): %v", iface, ipNode, err)
		}
	}

	// Link the IP to its network
	// Create or get network node
	network := net.IPNet{IP: ip, Mask: mask}
	networkNode, err := r.createNetwork(network)
	if err != nil {
		return nil, err
	}

	networkEdgeFilter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeNetworkIP),
	)
	if !r.Graph.AreLinked(networkNode, ipNode, networkEdgeFilter) {
		_, err = r.newEdge(networkNode, ipNode, map[string]interface{}{
			MetadataRelationTypeKey: RelationTypeNetworkIP,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to link network (%+v) to IP (%+v): %v", networkNode, ipNode, err)
		}
	}

	// Link the VRF to the network
	vrfNetworkEdgeFilter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeVRFNetwork),
	)
	if !r.Graph.AreLinked(vrfNode, networkNode, vrfNetworkEdgeFilter) {
		_, err = r.newEdge(vrfNode, networkNode, map[string]interface{}{
			MetadataRelationTypeKey: RelationTypeVRFNetwork,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to link VRF (%+v) to network (%+v): %v", vrfNode, networkNode, err)
		}
	}

	return ipNode, nil
}

// createVRF create a VRF node linked to the device node.
func (r *Resolver) createVRF(vrf *string, device *graph.Node) (*graph.Node, error) {
	var name string
	if vrf == nil {
		name = "default"
	} else {
		name = *vrf
	}

	metadata := map[string]interface{}{
		MetadataNameKey: name,
		MetadataTypeKey: TypeVRF,
	}

	// Find VRF nodes linked to this interface
	vrfNodeFilter := graph.NewElementFilter(filters.NewAndFilter(
		filters.NewTermStringFilter(MetadataTypeKey, TypeVRF),
		filters.NewTermStringFilter(MetadataNameKey, name),
	))
	vrfEdgeFilter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeDeviceVRF),
	)
	nodes := r.Graph.LookupChildren(device, vrfNodeFilter, vrfEdgeFilter)

	var vrfNode *graph.Node
	var err error

	// Create, if it does not exists.
	// Return internal ID if it exists.
	// Error if there more than one node matching
	if len(nodes) == 0 {
		vrfNode, err = r.newNode(metadata)
		if err != nil {
			return nil, err
		}
	} else if len(nodes) == 1 {
		vrfNode = nodes[0]
	} else {
		logging.GetLogger().Errorf("more than one VRF nodes with same pkey linked to %+v: %+v", device, nodes)
		return nil, fmt.Errorf("%d VRF linked to %+v nodes with same pkey already exists", len(nodes), device)
	}

	// Link VRF to the device
	if !r.Graph.AreLinked(device, vrfNode, vrfEdgeFilter) {
		_, err = r.newEdge(device, vrfNode, map[string]interface{}{
			MetadataRelationTypeKey: RelationTypeDeviceVRF,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to link device (%+v) to VRF (%+v): %v", device, vrfNode, err)
		}
	}

	return vrfNode, nil
}

// createNetwork create a node to represent a IP network
func (r *Resolver) createNetwork(network net.IPNet) (*graph.Node, error) {
	baseAddr := network.IP.Mask(network.Mask)
	metadata := map[string]interface{}{
		MetadataNameKey: fmt.Sprintf("NET:%s", baseAddr.String()),
		MetadataTypeKey: TypeNetwork,
		MetadataCIDRKey: baseAddr.String(),
	}

	// Find network node with matching CIDR+Type
	nodes := r.Graph.GetNodes(graph.Metadata{
		MetadataTypeKey: TypeNetwork,
		MetadataCIDRKey: baseAddr.String(),
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
		node = nodes[0]
	} else {
		logging.GetLogger().Errorf("more than one network nodes with same pkey: %+v", nodes)
		return nil, fmt.Errorf("%d network nodes with same pkey already exists", len(nodes))
	}

	return node, nil
}

// createInterface add a new node to Skydive to represent a interface.
// This node is also linked with the node using a layer2 edge.
// Add to the interface node its routing table.
// Return if any interface has been updated.
func (r *Resolver) createInterfaces(
	node *graph.Node,
	interfaces []*model.InterfaceInput,
	routingTable map[string]RouteTable,
) (updated bool, err error) {
	// Get currently connected interfaces
	// From the node, get layer2 edges.
	// From that edges, get only nodes type interface
	interfaceEdgeFilter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeDeviceInterface),
	)
	interfaceNodeFilter := graph.NewElementFilter(
		filters.NewTermStringFilter(MetadataTypeKey, TypeInterface),
	)

	layer2Edges := r.Graph.GetNodeEdges(node, interfaceEdgeFilter)
	// currentInterfaces create an index of current node interfaces using its name
	currentInterfaces := map[string]*graph.Node{}
	for _, e := range layer2Edges {
		var edgeNodes []*graph.Node
		if e.Child == node.ID {
			edgeNodes, _ = r.Graph.GetEdgeNodes(e, interfaceNodeFilter, nil)
		} else {
			_, edgeNodes = r.Graph.GetEdgeNodes(e, nil, interfaceNodeFilter)
		}
		for _, n := range edgeNodes {
			ifaceName, errG := n.GetFieldString(MetadataNameKey)
			if errG != nil {
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

		// Add route table if we have a route table for that VRF
		if userIface.IP != nil {
			vrfName := "default"
			if userIface.IP.Vrf != nil {
				vrfName = *userIface.IP.Vrf
			}

			routes, ok := routingTable[vrfName]
			if ok {
				// TODO hacer el marshal/unmarshal de este metadato
				ifaceMetadata[MetadataRoutingTableKey] = routes
			} else {
				logging.GetLogger().Infof("adding interface %+v with IP but without route table", userIface)
			}
		}

		iface, ok := currentInterfaces[userIface.Name]
		if ok {
			revisionPreUpdateIface := iface.Revision
			errS := r.Graph.SetMetadata(iface, ifaceMetadata)
			if errS != nil {
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

		// Link node with interface
		if !r.Graph.AreLinked(node, iface, interfaceEdgeFilter) {
			_, err = r.newEdge(node, iface, map[string]interface{}{
				MetadataRelationTypeKey: RelationTypeDeviceInterface,
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

		// Add VRF/IP if defined
		if userIface.IP != nil {
			err := r.addIPToInterface(node, iface, *userIface.IP)
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
// To each interface node, add its routing table (matched by VRF).
// Return the node created.
// Return updated true if the node metadata has been updated.
// Return interfaceUpdated true if any of the interfaces has been updated.
func (r *Resolver) addNodeWithInterfaces(
	metadata graph.Metadata,
	interfaces []*model.InterfaceInput,
	nodePKey graph.Metadata,
	routingTableList []*model.VRFRouteTable,
) (node *graph.Node, updated bool, interfaceUpdated bool, err error) {
	// Find switch node with matching Name+Type
	nodes := r.Graph.GetNodes(nodePKey)

	// Map routing table by VRF name
	routingTable := map[string]RouteTable{}
	if routingTableList != nil {
		for _, rt := range routingTableList {
			// Set default VRF name to "default"
			name := "default"
			if rt.Vrf != nil {
				name = *rt.Vrf
			}

			routes := RouteTable{}
			// Convert the route format to the one that will be stored in skydive
			// Check that values are correct and store network info as CIDR
			for _, r := range rt.Routes {
				var network *net.IPNet
				if r.Cidr != nil {
					_, network, err = net.ParseCIDR(*r.Cidr)
					if err != nil {
						return nil, false, false, fmt.Errorf("invalid CIDR for routing: %v", r.Cidr)
					}
				} else if r.IP != nil && r.Mask != nil {
					ip := net.ParseIP(*r.IP)
					if ip == nil {
						return nil, false, false, fmt.Errorf("Invalid IP for route: %v", r.IP)
					}
					maskIP := net.ParseIP(*r.Mask)
					if ip == nil {
						return nil, false, false, fmt.Errorf("Invalid mask for route: %v", r.Mask)
					}
					network = &net.IPNet{
						IP:   ip,
						Mask: net.IPMask(maskIP),
					}
				} else {
					return nil, false, false, fmt.Errorf("Network not specified for routing table")
				}

				route := Route{Network: Prefix(*network)}
				if r.Name != nil {
					route.Name = *r.Name
				}
				if r.NextHop != nil {
					ipNextHop := net.ParseIP(*r.NextHop)
					if ipNextHop == nil {
						return nil, false, false, fmt.Errorf("Invalid next hop IP: %v", *r.NextHop)
					}
					route.NextHop = ipNextHop
				}
				if r.DeviceNextHop != nil {
					route.DeviceNextHop = *r.DeviceNextHop
				}
				routes = append(routes, route)
			}
			routingTable[name] = routes
		}
	}

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
	interfaceUpdated, err = r.createInterfaces(node, interfaces, routingTable)
	if err != nil {
		logging.GetLogger().Errorf("creating interfaces for node %+v: %v", node, err)
		return nil, false, false, fmt.Errorf("creating interfaces: %v", err)
	}

	return node, updated, interfaceUpdated, nil
}

// addIPToInterface add to the interface the IP node.
// Links de IP also to its network. And the network to the VRF.
// Create the network and VRF if it does not exists yet.
func (r *Resolver) addIPToInterface(device *graph.Node, iface *graph.Node, ip model.InterfaceIPInput) error {
	parsedIP := net.ParseIP(ip.IP)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP address: %s", ip.IP)
	}

	parsedMaskIP := net.ParseIP(ip.Mask)
	if parsedMaskIP == nil {
		return fmt.Errorf("invalid network mask: %s", ip.Mask)
	}
	parsedMask := net.IPMask(parsedMaskIP)

	// Create VRF node and link to this device
	vrfNode, err := r.createVRF(ip.Vrf, device)
	if err != nil {
		return err
	}

	// Create IP node and link to this interface
	// Also links the IP to its network
	_, err = r.createIP(parsedIP, parsedMask, iface, vrfNode)
	if err != nil {
		return err
	}

	return nil
}
