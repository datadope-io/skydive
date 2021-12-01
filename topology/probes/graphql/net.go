package graphql

import (
	"fmt" // TODO borrar
	"net"
	"strings"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
)

const (
	// GraphqlNetPeeringOriginName string to identify the creator of pseudo wires edges
	GraphqlNetPeeringOriginName = "graphql.net.peering"
	// MetaKeyRelationType metadata value to specify the type of edge
	MetaKeyRelationType = "RelationType"
	// RelationTypeOwnership edge type value for edges between device-agg and agg-iface
	RelationTypeOwnership = "ownership"
	// RelationConnect edge type value for edges between iface-iface
	RelationConnect = "connect"
	// RelationTypePseudoWire edge type value for pseudo wire between ifaces
	RelationTypePseudoWire = "pseudowire"

	// MetaKeyType metadata key to define the kind of node
	MetaKeyType = "Type"
	// MetaKeySubtype metadata key to define a particular kind of a node type
	MetaKeySubtype = "SubType"
	// MetaKeyName key storing the name of the node
	MetaKeyName = "Name"
	// MetaKeyDescription key storing the dict with the description of the interface
	MetaKeyDescription = "Description"
	// MetaKeyDeviceID identifier used by routers. Format IP address as A.B.C.D
	MetaKeyDeviceID = "DeviceID"
	// MetaKeyDeviceType used to distinguish between routers and switches
	MetaKeyDeviceType = "DeviceType"
	// MetaKeyVendor extra data added to devices' metadata if defined
	MetaKeyVendor = "Vendor"
	// MetaKeyModel extra data added to devices' metadata if defined
	MetaKeyModel = "Model"
	// MetaKeyPseudowireID pseudo wire id stored in edges
	MetaKeyPseudowireID = "PseudoWireID"

	// MetaKeyLocation key to store the location of the node, as a map of latitude and longitude
	MetaKeyLocation = "Location"

	// MetaKeyLatitude metadata key to store the latitude of the node
	MetaKeyLatitude = "Latitude"
	// MetaKeyLongitude metadata key to store the longitude of the node
	MetaKeyLongitude = "Longitude"

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

	// MetadataTypeBuilding is the value of Metadata.Type for building nodes
	MetadataTypeBuilding = "building"

	// MetaTypeProvince is the value of Metadata.Type for province nodes
	MetadataTypeProvince = "province"

	// MetaTypeRegion is the value of Metadata.Type for region nodes
	MetadataTypeRegion = "region"
)

// netAddLocation is a function to create nodes with location info.
func (r *Resolver) netAddLocation(name string, metadataType string, location *model.NetLocationInput) (*graph.Node, bool, bool, error) {
	// Create the building node
	buildingNode, created, updated, err := r.createOrUpdateNode(
		graph.GenID(fmt.Sprintf("%s_%s", name, metadataType)),
		graph.Metadata{
			MetaKeyName: name,
			MetaKeyType: metadataType,
			MetaKeyLocation: map[string]float64{
				MetaKeyLatitude:  location.Lat,
				MetaKeyLongitude: location.Lng,
			},
		},
		nil,
	)
	return buildingNode, created, updated, err
}

// netAddDevice create, or update, a network device.
// deviceType, model and vendor could be nil.
// createdAt define the date of creation that should be used in Skydive, so we can
// create devices in the past.
func (r *Resolver) netAddDevice(
	name string,
	deviceID *string,
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
	if deviceID != nil {
		metadata[MetaKeyDeviceID] = *deviceID
	}
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
// The phyIface is linked to the logical interface using an ownership edge.
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

	// Get the device ID from the physical interface
	deviceID, _ := phyIface.GetFieldString(MetaKeyDeviceID)

	// To have a uniq ID for this logical interface, it is generated with the ID of the
	// physical/aggregation interface and the name and type of the logical interface

	// Create graph node representing the interface
	node, created, updated, err = r.netCreateInterface(*iface, MetadataSubtypeLogicalInterface, string(phyIface.ID), deviceID, createdAt, false)
	if err != nil {
		return node, created, updated, fmt.Errorf("unable to create logical interface: %v", err)
	}

	// Link the physical interface with the logical interface
	_, _, _, err = r.createOrUpdateEdge(phyIface, node, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership}, createdAt)
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

	// TODO problema con los pseudowires, queremos crear los links hacia los padres antes de que el nodo de la interfaz sepa
	// quien es su padre, por lo que no puede subir hacia arriba.
	// Intento simular que creo de nuevo el nodo a ver si salta de nuevo el GetABLinks
	node, created, updated, err = r.netCreateInterface(*iface, MetadataSubtypeLogicalInterface, string(phyIface.ID), deviceID, createdAt, true)
	if err != nil {
		return node, created, updated, fmt.Errorf("unable to create logical interface: %v", err)
	}

	return node, created, updated, nil
}

// netAddInterfaceToDevice create an interface node with the information in iface and link
// it to the device node.
// Subtype defines which kind of interface is.
// If defined, createdAt is used as the creation date of the node
// If the interface did't exist and fails to link to the device, remove it.
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

	deviceID, _ := device.Metadata.GetFieldString(MetaKeyDeviceID)

	// Create graph node representing the interface
	node, created, updated, err = r.netCreateInterface(*iface, subtype, deviceName, deviceID, createdAt, false)
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

	// TODO problema con los pseudowires, queremos crear los links hacia los padres antes de que el nodo de la interfaz sepa
	// quien es su padre, por lo que no puede subir hacia arriba.
	// Intento simular que creo de nuevo el nodo a ver si salta de nuevo el GetABLinks
	// Ignoramos created y updated porque aqui siempre serían false y true. Cogemos los originales
	node, _, _, err = r.netCreateInterface(*iface, subtype, deviceName, deviceID, createdAt, true)
	if err != nil {
		return node, created, updated, fmt.Errorf("unable to create logical interface: %v", err)
	}

	return node, created, updated, nil
}

// netCreateInterface create a new interface node with the data and subtype values.
// This function create the node and add it to the graph, but does not link it to any node.
// Prefix is used to create the identifier, together with the interface name and subtype
// but not between devices.
// If defined, deviceID is added as metadata to the node.
// CreatedAt, if defined, set the creation date of the skydive node.
func (r *Resolver) netCreateInterface(
	data model.NetInterfaceInput,
	subtype string,
	prefix string,
	deviceID string,
	createdAt *time.Time,
	addPseudo bool, // TODO temporal, para apañar la creacion de pseudowires padres
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
					VID: data.Ethernet.Vlan.Access.Vid,
				},
			}
		}
		if data.Ethernet.Vlan.Trunk != nil {
			networkInfo.Ethernet.VLAN = &VLANInfo{
				Trunk: &VLANTrunkInfo{
					AcceptedVIDs: data.Ethernet.Vlan.Trunk.AcceptedVIDs,
				},
			}
			if data.Ethernet.Vlan.Trunk.NativeVlanvid != nil {
				networkInfo.Ethernet.VLAN.Trunk.NativeVLANVID = int64(*data.Ethernet.Vlan.Trunk.NativeVlanvid)
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

	if addPseudo && len(data.PseudoWires) != 0 {
		pw := []PseudoWireInfo{}
		for _, pwData := range data.PseudoWires {
			pw = append(pw, PseudoWireInfo{
				ID:       pwData.ID,
				Neighbor: pwData.Neighbor,
			})
		}
		networkInfo.PseudoWires = pw
	}

	// Convert description from graphql format to skydive metadata format (map[string]string)
	description := map[string]string{}
	for _, d := range data.Description {
		// Strip dot from beggining or end of key and value
		// Avoid Elastic error trying o index.
		// https://github.com/elastic/elasticsearch/issues/28948
		key := strings.Trim(d.Key, ".")
		value := strings.Trim(d.Value, ".")
		description[key] = value
	}

	ifaceMetadata := graph.Metadata{
		MetaKeyName: data.Name,
		// MetaKeyType is string, so it will be the same type when recovered from the ES backend
		MetaKeyType:            MetadataTypeNetworkInterface,
		MetaKeySubtype:         subtype,
		MetaKeyDescription:     description,
		MetadataKeyNetworkInfo: &networkInfo,
		// Añadimos el deviceID. Necesario para los pseudowires (para saber si estamos matcheando el neighbor correcto)
		// Puede que sea la cadena vacía (si el device no lo tiene definido)
		MetaKeyDeviceID: deviceID,
	}

	// Generate a uniq identifier for device and interface name
	// Get the existant interface node or create it otherwise.
	ifID := getInterfaceID(prefix, data.Name, subtype)

	logging.GetLogger().Debugf("Crear interface, metadata: %+v", ifaceMetadata) // TODO

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

// getHierarchyNodesFromNode TODO documentar
func (r *mutationResolver) getHierarchyNodesFromNode(node *graph.Node) []*graph.Node {
	nodes := []*graph.Node{node}
	for {
		parents := r.Graph.LookupParents(node, nil, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership})
		if len(parents) == 0 {
			break
		} else if len(parents) > 1 {
			// TODO que hacer en este caso?
			logging.GetLogger().Errorf("getHierarchyNodesFromNode: more than one parent for node %s", node.ID)
		}

		nodes = append(nodes, parents[0])
		node = parents[0]
	}
	return nodes
}

// pseudoWireHasher creates an index of pseudo wires by their ID. Used by the linker
func pseudoWireHasher(n *graph.Node) map[string]interface{} {
	netInfoRaw, err := n.Metadata.GetField(MetadataKeyNetworkInfo)
	if err != nil {
		return nil
	}

	netInfo, ok := netInfoRaw.(*NetworkInfo)
	if !ok {
		// Ignore invalid values for NetworkInfo metadata
		return nil
	}

	if len(netInfo.PseudoWires) == 0 {
		// Tenemos que retornar un dict no vacío para que el linker llame a GetABLinks, donde se borrarán los
		// edges que ya no dabn existir al quitarse los PW.
		return map[string]interface{}{
			"_EMPTY": nil,
		}
	}

	kv := map[string]map[string]interface{}{}

	for _, pw := range netInfo.PseudoWires {
		if _, ok := kv[pw.ID]; ok {
			kv[pw.ID][pw.Neighbor] = nil
		} else {
			kv[pw.ID] = map[string]interface{}{pw.Neighbor: nil}
		}
	}

	logging.GetLogger().Debugf("PseudoWire index for node %s: %+v", n.ID, kv)

	// Convert to needed format by the hasher
	ret := map[string]interface{}{}
	for k, v := range kv {
		ret[k] = v
	}

	return ret
}

type simpleLinker struct {
	probe *Probe
}

// GetABLinks a partir de un nodo, se obtienen sus pseudowires y se busca en el índice de pseudowires otros nodos
// que tengan ese pseudowire configurado.
// Esta función retorna los edges que deben existir determinados por esos pseudowires.
// Se crearán edge-pseudowire entre las interfaces y luego también entre los edificios, provincias y regiones.
// Edges que se crearán (en caso de que los nodos tengan edificios, provincias y regiones):
//  - interface - interface
//  - building - building
//  - building - province
//  - building - region
//  - province - province
//  - province - region
//  - region - region
//
// Para evitar generar edges repetidos (uno en el setido DevA -> DevB y otro en DevB -> DevA), los edges siempre irán
// desde el nodo con el mayor ID (parent) al de menor ID (child).
// El resto de edges (building, province, region) se guiarán por el orden de las interfaces.
// Es decir, el building de la interfaz con mayor ID será el origen del edge contra el resto de nodos.
// Los nodos con mayor ID usarán esta función para crear los edges, salientes.
func (l *simpleLinker) GetABLinks(n *graph.Node) (edges []*graph.Edge) {
	netInfoRaw, err := n.Metadata.GetField(MetadataKeyNetworkInfo)
	if err != nil {
		return nil
	}

	netInfo, ok := netInfoRaw.(*NetworkInfo)
	if !ok {
		// Ignore invalid values for NetworkInfo metadata
		return nil
	}

	// Iteramos por los diferentes PW que puede tener definidos la interfaz
	for _, pw := range netInfo.PseudoWires {
		// Por cada PW id, consultamos el índice para obtener que otros nodos tienen configurado ese PW.
		// Obtenemos los nodos y el contenido del índice, que son las IPs que ese nodo remoto tiene como neighbors del PW.
		remoteNodes, remoteNodesNeighbors := l.probe.pseudoWireIndexer.Get(pw.ID)
		for i := 0; i < len(remoteNodes); i++ {
			remoteNode := remoteNodes[i]
			remoteNodeNeighbors, ok := remoteNodesNeighbors[i].(map[string]interface{})
			if !ok {
				logging.GetLogger().Errorf("invalid conversion to pseudowire in node %v: %v", remoteNode.ID, remoteNodesNeighbors[i])
				continue
			}

			// Evitamos crear nodos hacia el propio nodo
			if remoteNode.ID == n.ID {
				continue
			}

			// Para evitar duplicar edges, solo si este nodo (n) tiene mayor ID que el nodo remoto (remoteNode) se crearán los edges.
			// Estos edges no creados se crearán en GetBALinks para este nodo o con GetABLinks cuando sea el nodo remoto el parámetro.
			if remoteNode.ID > n.ID {
				continue
			}

			// Si el device id del nodo remoto no coincide con el neighbor decalarado en el PW, no creamos el edge.
			remoteNodeDeviceID, err := remoteNode.GetFieldString(MetaKeyDeviceID)
			if err != nil {
				logging.GetLogger().Errorf("node without device id, needed for pseudowiring: %+v", remoteNode)
				continue
			}
			if remoteNodeDeviceID != pw.Neighbor {
				continue
			}

			// Si el device ID del nodo que ha llamado a GetABLinks no está en los neighbors definidos por el nodo remoto, tampoco
			// creamos el edge.
			sourceNodeDeviceID, err := n.GetFieldString(MetaKeyDeviceID)
			if err != nil {
				logging.GetLogger().Errorf("node without device id, needed for pseudowiring: %+v", n)
				continue
			}
			if _, ok := remoteNodeNeighbors[sourceNodeDeviceID]; !ok {
				continue
			}

			// Genero el edge entre las interfaces.
			edges = append(edges, l.generatePseudowireEdge(n, remoteNode, pw.ID))

			// Ahora iteramos por los parents del nodo origen, para crear los edges entre buildings, provinces y regions.
			for _, srcNode := range l.probe.getHierarchyNodesFromNode(n) {
				srcType := srcNode.Metadata[MetaKeyType]

				// Solo queremos crear edges entre buildings, provinces y regions.
				// Ignoramos los otros parent nodes que pueda tener (interfaces y devices).
				if srcType != MetadataTypeBuilding &&
					srcType != MetadataTypeProvince &&
					srcType != MetadataTypeRegion {
					continue
				}

				// Ahora obtenemos los parents del nodo remoto
				for _, dstNode := range l.probe.getHierarchyNodesFromNode(remoteNode) {
					dstType := dstNode.Metadata[MetaKeyType]

					if dstType != MetadataTypeBuilding &&
						dstType != MetadataTypeProvince &&
						dstType != MetadataTypeRegion {
						continue
					}

					edges = append(edges, l.generatePseudowireEdge(srcNode, dstNode, pw.ID))
				}
			}
		}
	}

	return edges
}

// GetBALinks devolvemos las conexiones entrandes al nodo "n" cuando el nodo n tiene un identificador mayor a los nodos remotos.
// De esta manera conseguimos que coincida lo que genera el nodo "A" al ejecutar "AB" con lo que genera el nodo "B"
// al ejecutar "BA".
// Los nodos con menor ID usarán esta función para crear los edges, entrantes.
func (l *simpleLinker) GetBALinks(n *graph.Node) (edges []*graph.Edge) {
	netInfoRaw, err := n.Metadata.GetField(MetadataKeyNetworkInfo)
	if err != nil {
		return nil
	}

	netInfo, ok := netInfoRaw.(*NetworkInfo)
	if !ok {
		// Ignore invalid values for NetworkInfo metadata
		return nil
	}

	// Iteramos por los diferentes PW que puede tener definidos la interfaz
	for _, pw := range netInfo.PseudoWires {
		// Por cada PW id, consultamos el índice para obtener que otros nodos tienen configurado ese PW.
		// Obtenemos los nodos y el contenido del índice, que son las IPs que ese nodo remoto tiene como neighbors del PW.
		remoteNodes, remoteNodesNeighbors := l.probe.pseudoWireIndexer.Get(pw.ID)
		for i := 0; i < len(remoteNodes); i++ {
			remoteNode := remoteNodes[i]
			remoteNodeNeighbors, ok := remoteNodesNeighbors[i].(map[string]interface{})
			if !ok {
				logging.GetLogger().Errorf("invalid conversion to pseudowire in node %v: %v", remoteNode.ID, remoteNodesNeighbors[i])
				continue
			}

			// Evitamos crear nodos hacia el propio nodo
			if remoteNode.ID == n.ID {
				continue
			}

			// Para evitar duplicar edges, solo si este nodo (n) tiene menor ID que el nodo remoto (remoteNode) se crearán los edges.
			// Estos edges no creados se crearán en GetABLinks para este nodo o con GetABLinks cuando sea el nodo remoto el parámetro.
			if remoteNode.ID < n.ID {
				continue
			}

			// Si el device id del nodo remoto no coincide con el neighbor decalarado en el PW, no creamos el edge.
			remoteNodeDeviceID, err := remoteNode.GetFieldString(MetaKeyDeviceID)
			if err != nil {
				logging.GetLogger().Errorf("node without device id, needed for pseudowiring: %+v", remoteNode)
				continue
			}
			if remoteNodeDeviceID != pw.Neighbor {
				continue
			}

			// Si el device ID del nodo que ha llamado a GetABLinks no está en los neighbors definidos por el nodo remoto, tampoco
			// creamos el edge.
			sourceNodeDeviceID, err := n.GetFieldString(MetaKeyDeviceID)
			if err != nil {
				logging.GetLogger().Errorf("node without device id, needed for pseudowiring: %+v", n)
				continue
			}
			if _, ok := remoteNodeNeighbors[sourceNodeDeviceID]; !ok {
				continue
			}

			// Genero el edge entre las interfaces.
			// Para este caso "BA" los edges se generan desde el nodo remoto hacia el nodo origen.
			edges = append(edges, l.generatePseudowireEdge(remoteNode, n, pw.ID))

			// Ahora iteramos por los parents del nodo origen, para crear los edges entre buildings, provinces y regions.
			for _, srcNode := range l.probe.getHierarchyNodesFromNode(n) {
				srcType := srcNode.Metadata[MetaKeyType]

				// Solo queremos crear edges entre buildings, provinces y regions.
				// Ignoramos los otros parent nodes que pueda tener (interfaces y devices).
				if srcType != MetadataTypeBuilding &&
					srcType != MetadataTypeProvince &&
					srcType != MetadataTypeRegion {
					continue
				}

				// Ahora obtenemos los parents del nodo remoto
				for _, dstNode := range l.probe.getHierarchyNodesFromNode(remoteNode) {
					dstType := dstNode.Metadata[MetaKeyType]

					if dstType != MetadataTypeBuilding &&
						dstType != MetadataTypeProvince &&
						dstType != MetadataTypeRegion {
						continue
					}

					edges = append(edges, l.generatePseudowireEdge(dstNode, srcNode, pw.ID))
				}
			}
		}
	}

	return edges
}

// generatePseudowireEdge retorna un edge tipo pseudowire a partir de los nodos pasados por parámetro y el PW id, que
// se introduce como metadata.
func (l *simpleLinker) generatePseudowireEdge(srcNode, dstNode *graph.Node, pwid string) *graph.Edge {
	return graph.CreateEdge(
		graph.Identifier(string(srcNode.ID)+"_"+string(dstNode.ID)+"_"+RelationTypePseudoWire+"_"+pwid),
		srcNode,
		dstNode,
		graph.Metadata{
			MetaKeyRelationType: RelationTypePseudoWire,
			MetaKeyPseudowireID: pwid,
		},
		graph.TimeUTC(),
		srcNode.Host, // Host to the same value of the connection originator node
		GraphqlNetPeeringOriginName+l.probe.graph.GetOrigin(), // Procpeering creator of the edge
	)
}

// getHierarchyNodesFromNode TODO documentar y ver como dejar solo una entre aqui y func (r *mutationResolver) getHierarchyNodesFromNode
func (p *Probe) getHierarchyNodesFromNode(node *graph.Node) []*graph.Node {
	nodes := []*graph.Node{node}
	for {
		parents := p.graph.LookupParents(node, nil, graph.Metadata{MetaKeyRelationType: RelationTypeOwnership})
		if len(parents) == 0 {
			break
		} else if len(parents) > 1 {
			// TODO que hacer en este caso?
			logging.GetLogger().Errorf("getHierarchyNodesFromNode: more than one parent for node %s", node.ID)
		}

		nodes = append(nodes, parents[0])
		node = parents[0]
	}
	return nodes
}

// handleDeletedPseudowire dado un edge tipo pseudowire, borramos todos los otros edges-pw que sean del mismo id.
// Para evitar borrar otros edge-pw con el mismo id pero que no tuviesen relación (caso esquina), solo se borrarán
// los que tengan en el parent o child alguno de los nodos padres del edge original.
// parent are involved.
func (p *Probe) handleDeletedPseudowire(edge *graph.Edge) error {
	p.graph.Lock()
	defer p.graph.Unlock()

	pwid, err := edge.GetFieldString(MetaKeyPseudowireID)
	if err != nil {
		return fmt.Errorf("pseudowire edge without pwid: %+v", edge)
	}

	// Generamos el listado de node IDs pertenecientes a parents del edge.
	parentIDs := map[graph.Identifier]interface{}{}
	parentNode := p.graph.GetNode(edge.Parent)
	childNode := p.graph.GetNode(edge.Child)

	for _, node := range p.getHierarchyNodesFromNode(parentNode) {
		parentIDs[node.ID] = nil
	}
	for _, node := range p.getHierarchyNodesFromNode(childNode) {
		parentIDs[node.ID] = nil
	}

	// Obtenemos todos los edges-pw con el mismo pwid.
	edges := p.graph.GetEdges(graph.Metadata{
		MetaKeyRelationType: RelationTypePseudoWire,
		MetaKeyPseudowireID: pwid,
	})

	// Iteramos por los edges, borrando aquellos cuyo parent o child esté en el listado de parents.
	for _, edge := range edges {
		if _, ok := parentIDs[edge.Parent]; ok {
			err := p.graph.DelEdge(edge)
			if err != nil {
				logging.GetLogger().Errorf("Error deleting pseudowire edge %s: %v", edge.ID, err)
			}
		} else if _, ok := parentIDs[edge.Child]; ok {
			err := p.graph.DelEdge(edge)
			if err != nil {
				logging.GetLogger().Errorf("Error deleting pseudowire edge %s: %v", edge.ID, err)
			}
		}
	}

	return nil
}
