package graphql

import (
	"fmt"
	"net"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
)

const (
	// MetaKeyRelationType metadata value to specify the type of edge
	MetaKeyRelationType = "RelationType"
	// RelationTypeOwnership edge type value for edges between device-agg and agg-iface
	RelationTypeOwnership = "ownership"
	// RelationConnectsTo edge type value for edges between iface-iface
	RelationConnectsTo = "ConnectsTo"
	// RelationTypeLogicalIfaceToPhyIface is the Metadata.RelationType value for logical interfaces linked to physical/aggregation interfaces
	RelationTypeLogicalIfaceToPhyIface = "use"

	// MetaKeyType metadata key to define the kind of node
	MetaKeyType = "Type"
	// MetaKeySubtype metadata key to define a particular kind of a node type
	MetaKeySubtype = "SubType"
	// MetaKeyName key storing the name of the node
	MetaKeyName = "Name"
	// MetaKeyDeviceType used to distinguish between routers and switches
	MetaKeyDeviceType = "DeviceType"
	// MetaKeyVendor extra data added to devices' metadata if defined
	MetaKeyVendor = "Vendor"
	// MetaKeyModel extra data added to devices' metadata if defined
	MetaKeyModel = "Model"

	// MetadataKeyNetworkInfo metadata key where interface nodes store its ethernet and IP data
	MetadataKeyNetworkInfo = "NetworkInfo"

	// MetadataTypeNetworkDevice is the value of Metadata.Type for network devices
	MetadataTypeNetworkDevice = "device"
	// MetadataTypeNetworkInterface is the value of Metadata.Type for network interfaces
	MetadataTypeNetworkInterface = "interface"

	// MetadataSubtypePhysicalInterface value of Metadata.Subtype for physical network interfaces
	MetadataSubtypePhysicalInterface = "physical"
	// MetadataSubtypeAggregationInterface value of Metadata.Subtype for aggregation network interfaces. This interfaces aggregate several physical interfaces, normally with LACP
	MetadataSubtypeAggregationInterface = "aggregation"
	// MetadataSubtypeLogicalInterface value of Metadata.Subtype for logical network interfaces. This interfaces use one physical or aggregation interface and are connected to one particular VLAN
	MetadataSubtypeLogicalInterface = "logical"
)

// netAddDevice create, or update, a network device.
// deviceType, model and vendor could be nil.
// createdAt define the date of creation that should be used in Skydive, so we can
// create devices in the past.
func (r *Resolver) netAddDevice(
	name string,
	deviceType *model.NetDeviceType,
	model *string,
	vendor *string,
	createdAt *time.Time,
) (node *graph.Node, nodeCreated bool, nodeUpdated bool, err error) {
	if name == "" {
		return node, nodeCreated, nodeUpdated, fmt.Errorf("device name could not be empty")
	}

	if deviceType == nil {
		return node, nodeCreated, nodeUpdated, fmt.Errorf("device type could not be nil")
	}

	// Get the ID of the network device for the name given.
	// This ID is uniq for the same name.
	id := getDeviceID(name)

	// Node metadata generated from the API call
	// It will be used to create the node or update an existing node.
	metadata := graph.Metadata{
		MetaKeyName: name,
		MetaKeyType: MetadataTypeNetworkDevice,
	}

	// Optional metadata values
	if deviceType != nil {
		// string so it will be the same type when recovered from the ES backend
		metadata[MetaKeyDeviceType] = string(*deviceType)
	}
	if model != nil {
		metadata[MetaKeyModel] = *model
	}
	if vendor != nil {
		metadata[MetaKeyVendor] = *vendor
	}

	node, nodeCreated, nodeUpdated, err = r.createOrUpdateNode(id, metadata, createdAt)
	return
}

// netAddLogicalToInterface create a logical interface linked to a physical or aggregation
// If defined, createdAt is used as the creation date of the node
// If the logical interface did't exist and fails to link to the interface, remove it.
func (r *Resolver) netAddLogicalToInterface(
	phyIface *graph.Node,
	iface *model.NetInterfaceInput,
	createdAt *time.Time,
) (node *graph.Node, created bool, updated bool, err error) {
	if phyIface == nil {
		return node, created, updated, fmt.Errorf("invalid nil physical interface node")
	}
	if iface == nil {
		return node, created, updated, fmt.Errorf("interface data is not defined")
	}

	// To have a uniq ID for this logical interface, it is generated with the ID of the
	// physical/aggregation interface and the name and type of the logical interface

	// Create graph node representing the interface
	node, created, updated, err = r.netCreateInterface(*iface, MetadataSubtypeLogicalInterface, string(phyIface.ID), createdAt)
	if err != nil {
		return node, created, updated, fmt.Errorf("unable to create logical interface: %v", err)
	}

	_, _, _, err = r.createOrUpdateEdge(node, phyIface, graph.Metadata{MetaKeyRelationType: RelationTypeLogicalIfaceToPhyIface}, createdAt)
	if err != nil {
		// Remove newly created logical interface if we cannot link it to the interface
		if created {
			errDelNode := r.Graph.DelNode(node)
			if errDelNode != nil {
				return node, created, updated, fmt.Errorf("delete logical interface not linked: %v", errDelNode)
			}
		}
		return node, created, updated, fmt.Errorf("link device with interface, interface node not created: %v", err)
	}

	return node, created, updated, nil
}

// netAddInterfaceToDevice create an interface node with the information in iface and link
// it to the device node.
// Subtype defines which kind of interface is.
// If defined, createdAt is used as the creation date of the node
// If the interface did't exist and fails to link to the device, remove it. TODO testear esto
func (r *Resolver) netAddInterfaceToDevice(
	device *graph.Node,
	iface *model.NetInterfaceInput,
	subtype string,
	createdAt *time.Time,
) (node *graph.Node, created bool, updated bool, err error) {
	if device == nil {
		return node, created, updated, fmt.Errorf("invalid nil device node")
	}
	if iface == nil {
		return node, created, updated, fmt.Errorf("interface data is not defined")
	}
	if subtype == "" {
		return node, created, updated, fmt.Errorf("interface subtype could not be empty")
	}

	deviceName, err := device.Metadata.GetFieldString(MetaKeyName)
	if err != nil {
		return node, created, updated, fmt.Errorf("unable to get device name to create an interface: %v", err)
	}

	// Create graph node representing the interface
	node, created, updated, err = r.netCreateInterface(*iface, subtype, deviceName, createdAt)
	if err != nil {
		return node, created, updated, fmt.Errorf("unable to create interface: %v", err)
	}

	_, _, _, err = r.createOrUpdateEdge(device, node, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership}, createdAt)
	if err != nil {
		// Remove newly created interface if we cannot link it to the device
		if created {
			errDelNode := r.Graph.DelNode(node)
			if errDelNode != nil {
				return node, created, updated, fmt.Errorf("delete interface not linked: %v", errDelNode)
			}
		}
		return node, created, updated, fmt.Errorf("unable to link device with interface: %v", err)
	}

	return node, created, updated, nil
}

// netCreateInterface create a new interface node with the data and subtype values.
// This function create the node and add it to the graph, but does not link it to any node.
// Prefix is used to create the identifier, together with the interface name and subtype
// but not between devices.
// CreatedAt, if defined, set the creation date of the skydive node.
func (r *Resolver) netCreateInterface(
	data model.NetInterfaceInput,
	subtype string,
	prefix string,
	createdAt *time.Time,
) (node *graph.Node, created bool, updated bool, err error) {
	if subtype == "" {
		return node, created, updated, fmt.Errorf("interface subtype could not be empty")
	}

	if data.Name == "" {
		return node, created, updated, fmt.Errorf("interface name could not be empty")
	}

	// Convert interface data from graphql format to skydive metadata format
	networkInfo := NetworkInfo{}

	if data.Ethernet != nil && data.Ethernet.Vlan != nil {
		networkInfo.Ethernet = &EthernetInfo{}

		if data.Ethernet.Vlan.Access != nil && data.Ethernet.Vlan.Trunk != nil {
			return node, created, updated, fmt.Errorf("interface VLAN could not be access and trunk at the same time")
		}

		if data.Ethernet.Vlan.Access != nil {
			networkInfo.Ethernet.VLAN = &VLANInfo{
				Access: &VLANAccessInfo{
					AcceptedVIDs: data.Ethernet.Vlan.Access.AcceptedVIDs,
				},
			}
			if data.Ethernet.Vlan.Access.NativeVlanvid != nil {
				networkInfo.Ethernet.VLAN.Access.NativeVLANVID = int64(*data.Ethernet.Vlan.Access.NativeVlanvid)
			}
		}
		if data.Ethernet.Vlan.Trunk != nil {
			networkInfo.Ethernet.VLAN = &VLANInfo{
				Trunk: &VLANTrunkInfo{
					VID: data.Ethernet.Vlan.Trunk.Vid,
				},
			}
		}
	}

	ipinfo := []string{}
	for _, cidr := range data.IP {
		_, _, err = net.ParseCIDR(cidr)
		if err != nil {
			return node, created, updated, fmt.Errorf("invalid interface IP CIDR value: %v", err)
		}
		ipinfo = append(ipinfo, cidr)
	}

	if len(ipinfo) > 0 {
		networkInfo.IP = ipinfo
	}

	ifaceMetadata := graph.Metadata{
		MetaKeyName: data.Name,
		// MetaKeyType is string, so it will be the same type when recovered from the ES backend
		MetaKeyType:            MetadataTypeNetworkInterface,
		MetaKeySubtype:         subtype,
		MetadataKeyNetworkInfo: networkInfo,
	}

	// Generate a uniq identifier for device and interface name
	// Get the existant interface node or create it otherwise.
	ifID := getInterfaceID(prefix, data.Name, subtype)

	node, created, updated, err = r.createOrUpdateNode(ifID, ifaceMetadata, createdAt)
	if err != nil {
		err = fmt.Errorf("network interface: %v", err)
	}
	return node, created, updated, err
}

// netGetNodeFromSelector return a graph node from a net selector (id or gremlin query)
func (r *Resolver) netGetNodeFromSelector(selector *model.NetSelector) (*graph.Node, error) {
	if selector == nil {
		return nil, fmt.Errorf("invalid nil selector")
	}

	if selector.ID == nil && selector.GremlinQuery == nil {
		return nil, fmt.Errorf("invalid selector, no ID neither gremlin query")
	}

	if selector.ID != nil {
		n := r.Graph.GetNode(graph.Identifier(*selector.ID))
		if n == nil {
			return nil, fmt.Errorf("no node returned by selector ID")
		}
		return n, nil
	}

	// GremlinQuery not nil
	return r.getNodeFromGremlinQuery(*selector.GremlinQuery)
}

// netGetOrCreateAggInterface obtain the aggregation interface node given a device and the name.
// If it does not exists, create the node representing that aggregation interface, link it
// with the device and return that created node.
func (r *Resolver) netGetOrCreateAggInterface(
	device *graph.Node,
	aggName string,
	createdAt *time.Time,
) (*graph.Node, error) {
	if device == nil {
		return nil, fmt.Errorf("invalid nil device to get/create aggregation interface")
	}
	if aggName == "" {
		return nil, fmt.Errorf("aggregation interface name could not be empty")
	}

	aggNode, _, _, err := r.netAddInterfaceToDevice(
		device,
		&model.NetInterfaceInput{Name: aggName},
		MetadataSubtypeAggregationInterface,
		createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create aggregation interface: %v", err)
	}

	return aggNode, nil
}

// getInterfaceID return a graph identifier for network interfaces.
// This function will always return the same id if the pair of values are the same.
func getInterfaceID(device, iface string, subtype string) graph.Identifier {
	return graph.GenID(device, iface, MetadataTypeNetworkInterface, subtype)
}

// getDeviceID return a graph identifier for network device.
// This function will always return the same id if the value is the same.
// Device type is not used because it could be unkown.
func getDeviceID(device string) graph.Identifier {
	return graph.GenID(device, MetadataTypeNetworkDevice)
}
