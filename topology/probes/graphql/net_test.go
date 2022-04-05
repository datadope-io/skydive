package graphql

import (
	"context"
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
	"github.com/stretchr/testify/assert"
)

// strPointer used to pass pointers in the TDD struct
func strPointer(s string) *string {
	return &s
}

// createNodeWithTime helper function to create nodes for testing.
func createNodeWithTime(i graph.Identifier, m graph.Metadata, t time.Time) *graph.Node {
	return graph.CreateNode(i, m, graph.Time(t), "testhost", "graphql.analyzer.testhost")
}

// TestGetInterfaceID aseguramos que la generación del ID para interfaces este fijada.
// Se usa para obtener los nodos en el importador de eventos de ElasticSearch-ML.
// El ID debe ser sha256(device + "__" + interface)
func TestGetInterfaceID(t *testing.T) {
	expectedID := graph.Identifier("c83736f7543e7ee6f2f956f9f2fc88673c770e78c0654517bbecc162bbe25b4b")
	assert.Equal(t, expectedID, getInterfaceID("DEVICENAME", "INTERFACENAME"))
}

func TestNetAddDeviceUpdateNode(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	// Create the origin node
	createdAt := time.Unix(100, 0)
	n := createNodeWithTime(
		graph.GenID("deviceA", MetadataTypeNetworkDevice),
		graph.Metadata{
			MetaKeyName:   "nodeName",
			MetaKeyType:   "device",
			MetaKeyVendor: "vendorOrig",
			MetaKeyModel:  "modelOrig",
		},
		createdAt,
	)
	err := g.AddNode(n)
	assert.Nil(t, err)

	routerType := model.NetDeviceTypeRouter

	// Fail to modify an existing node with a different createdAt date
	newCreatedAt := time.Unix(200, 0)
	_, _, _, err = r.netAddDevice(
		"deviceA",
		nil, // TODO añadir tests con el DeviceID
		&routerType,
		nil,
		strPointer("vendorNew"),
		nil,
		nil,
		&newCreatedAt,
	)
	assert.EqualError(t, err, "forbidden to modify CreatedAt value of existing node")

	// It is valid to call with createdAt if it is the same as the origin node
	node, created, updated, err := r.netAddDevice(
		"deviceA",
		nil, // TODO añadir tests con el DeviceID
		&routerType,
		nil,
		strPointer("vendorNew"),
		strPointer("building"),
		nil,
		&createdAt,
	)
	assert.Nil(t, err)
	assert.Falsef(t, created, "node created")
	assert.Truef(t, updated, "node updated")

	// The only node in the graph should be the origin node modified
	nodes := g.GetNodes(nil)
	assert.Lenf(t, nodes, 1, "number of nodes")

	assert.Equal(t, node, nodes[0], "graph node and returned node should be the same")

	expectedMetadata := graph.Metadata{
		MetaKeyName:       "deviceA",
		MetaKeyType:       "device",
		MetaKeyDeviceType: "Router",
		MetaKeyVendor:     "vendorNew",
		MetaKeyBuilding:   "building",
		// Model was set to nil, so deleted from the node
	}

	// Creation date is not modified from the original value
	assert.Equalf(t, graph.Time(createdAt).UnixMilli(), node.CreatedAt.UnixMilli(), "create not modified")

	// Node updated date should be greater than createdAt
	assert.Greaterf(t, node.UpdatedAt.UnixMilli(), node.CreatedAt.UnixMilli(), "updated > created")

	assert.Equalf(t, expectedMetadata, node.Metadata, "expected node metadata")
	assert.Equalf(t, int64(2), node.Revision, "expected node metadata")

	// We could also pass nil to createAt, to ignore creation date of the origin node
	_, _, _, err = r.netAddDevice(
		"deviceA",
		nil, // TODO añadir tests con el DeviceID
		&routerType,
		nil,
		strPointer("vendorNew"),
		nil,
		nil,
		nil,
	)
	assert.Nil(t, err)
}

func TestNetAddDevice(t *testing.T) {
	routerType := model.NetDeviceTypeRouter
	// switchType := model.NetDeviceTypeSwitch
	createdAt := time.Unix(100, 0)

	tests := []struct {
		desc            string
		deviceName      string
		deviceType      *model.NetDeviceType
		model           *string
		vendor          *string
		building        *string
		cells           []string
		createdAt       *time.Time
		expectedNode    *graph.Node
		expectedCreated bool
		expectedUpdated bool
		expectedError   string
	}{
		{
			desc:          "empty values",
			expectedError: "device name could not be empty",
		},
		{
			desc:          "nil deviceType",
			deviceName:    "routerA",
			expectedError: "device type could not be nil",
		},
		{
			desc:       "router device with just name and type",
			deviceName: "routerA",
			deviceType: &routerType,
			createdAt:  &createdAt,
			expectedNode: createNodeWithTime(
				graph.GenID("routerA", MetadataTypeNetworkDevice),
				graph.Metadata{
					MetaKeyName:       "routerA",
					MetaKeyType:       "device",
					MetaKeyDeviceType: "Router",
				},
				createdAt,
			),
			expectedCreated: true,
		},
		{
			desc:       "router device with name,type, model and vendor",
			deviceName: "routerA",
			deviceType: &routerType,
			model:      strPointer("model"),
			vendor:     strPointer("vendor"),
			createdAt:  &createdAt,
			expectedNode: createNodeWithTime(
				graph.GenID("routerA", MetadataTypeNetworkDevice),
				graph.Metadata{
					MetaKeyName:       "routerA",
					MetaKeyType:       "device",
					MetaKeyDeviceType: "Router",
					MetaKeyModel:      "model",
					MetaKeyVendor:     "vendor",
				},
				createdAt,
			),
			expectedCreated: true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			g := newGraph(t)
			r := Resolver{Graph: g}
			node, created, updated, err := r.netAddDevice(
				test.deviceName,
				nil, // TODO añadir tests con el DeviceID
				test.deviceType,
				test.model,
				test.vendor,
				test.building,
				test.cells,
				test.createdAt,
			)

			if test.expectedError != "" {
				assert.EqualErrorf(t, err, test.expectedError, "error")
			}
			assert.Equalf(t, test.expectedNode, node, "node")
			assert.Equalf(t, test.expectedCreated, created, "created")
			assert.Equalf(t, test.expectedUpdated, updated, "updated")
		})
	}
}

func TestNetAddDeviceWithCells(t *testing.T) {
	routerType := model.NetDeviceTypeRouter
	g := newGraph(t)
	r := Resolver{Graph: g}
	node, created, updated, err := r.netAddDevice(
		"deviceName",
		nil,
		&routerType,
		strPointer("model"),
		strPointer("vendor"),
		strPointer("building"),
		[]string{"cell1", "cell2"},
		nil,
	)
	assert.Nil(t, err)
	assert.Truef(t, created, "node created")
	assert.Falsef(t, updated, "node not updated")
	nodeCellsIface, err := node.GetField(MetaKeyCells)
	assert.Nil(t, err)
	nodeCells, ok := nodeCellsIface.(map[string]bool)
	assert.Truef(t, ok, "node cells are a string slice")
	assert.Lenf(t, nodeCells, 2, "node cells length")

	// Get the keys of nodeCells
	var keys []string
	for k := range nodeCells {
		keys = append(keys, k)
	}
	assert.ElementsMatchf(t, []string{"cell1", "cell2"}, keys, "node cells keys")
}

func TestNetCreateInterfacErrorCreateInterfaceNode(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	data := model.NetInterfaceInput{Name: "eth0"}
	subtype := "bar"
	prefix := "prefix"

	createdAt := time.Unix(100, 0)
	node, created, updated, err := r.netCreateInterface(data, subtype, prefix, "1.1.1.1", &createdAt)
	assert.Nil(t, err)
	assert.NotNil(t, node)
	assert.Truef(t, created, "created")
	assert.Falsef(t, updated, "updated")

	newCreatedAt := time.Unix(200, 0)
	_, _, _, err = r.netCreateInterface(data, subtype, prefix, "1.1.1.1", &newCreatedAt)
	assert.EqualErrorf(t, err, "network interface: forbidden to modify CreatedAt value of existing node", "error mod date")
}

func TestNetCreateInterface(t *testing.T) {
	// force node creation date to simplify testing
	createdAt := time.Unix(100, 0)

	nativeVID := 3

	tests := []struct {
		originNode      *graph.Node
		desc            string
		data            model.NetInterfaceInput
		subtype         string
		prefix          string
		deviceid        string
		createdAt       *time.Time
		expectedNode    *graph.Node
		expectedCreated bool
		expectedUpdated bool
		expectedError   string
	}{
		{
			desc:          "empty parameters",
			expectedError: "interface subtype could not be empty",
		},
		{
			desc:          "empty interface input",
			subtype:       "subA",
			data:          model.NetInterfaceInput{},
			expectedError: "interface name could not be empty",
		},
		{
			desc:    "new interface with only name, empty prefix and description",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Description: []*model.NetKeyValue{
					{
						Key:   "RE",
						Value: "abc",
					},
					{
						Key:   "XX",
						Value: "foo",
					},
					{
						Key:   "_raw",
						Value: "<RE=abc><XX=foo>",
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":     "eth0",
					"Type":     "interface",
					"SubType":  "subA",
					"DeviceID": "",
					"Description": map[string]string{
						"RE":   "abc",
						"XX":   "foo",
						"_raw": "<RE=abc><XX=foo>",
					},
					"NetworkInfo": &NetworkInfo{},
				},
				createdAt,
			),
		},
		{
			// Si tiene un "." al comienzo o final el value el elastic falla con el error:
			// field starting or ending with a [.] makes object resolution ambiguous
			desc:    "interface description with starting dot is removed",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Description: []*model.NetKeyValue{
					{
						Key:   ".RE.",
						Value: ".abc.",
					},
					{
						Key:   "XX",
						Value: "foo",
					},
					{
						Key:   "_raw",
						Value: ".<.RE.=.abc.><XX=foo>.",
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":     "eth0",
					"Type":     "interface",
					"SubType":  "subA",
					"DeviceID": "",
					"Description": map[string]string{
						"RE":   "abc",
						"XX":   "foo",
						"_raw": "<.RE.=.abc.><XX=foo>",
					},
					"NetworkInfo": &NetworkInfo{},
				},
				createdAt,
			),
		},
		{
			desc:            "new interface with only name and prefix",
			subtype:         "subA",
			prefix:          "foo",
			data:            model.NetInterfaceInput{Name: "eth0"},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("foo", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{},
				},
				createdAt,
			),
		},
		{
			desc:    "invalid IP",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				IP:   []string{"1.2.3.4", "abcde"},
			},
			createdAt:     &createdAt,
			expectedError: "invalid interface IP CIDR value: invalid CIDR address: 1.2.3.4",
		},
		{
			desc:    "interface with IP v4 and v6",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				IP: []string{
					"1.2.3.4/24",
					"1aa1:30a:222:dead:bee:f851:f16a:fbef/64",
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						IP: []string{
							"1.2.3.4/24",
							"1aa1:30a:222:dead:bee:f851:f16a:fbef/64",
						},
					},
				},
				createdAt,
			),
		},
		{
			desc:    "VLAN access and trunk",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Access: &model.NetVLANAccessInput{
							Vid: 10,
						},
						Trunk: &model.NetVLANTrunkInput{
							AcceptedVIDs:  []int{1, 2, 3},
							NativeVlanvid: &nativeVID,
						},
					},
				},
			},
			expectedError: "interface VLAN could not be access and trunk at the same time",
		},
		{
			desc:    "VLAN trunk with native",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Trunk: &model.NetVLANTrunkInput{
							AcceptedVIDs:  []int{1, 2, 3},
							NativeVlanvid: &nativeVID,
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Trunk: &VLANTrunkInfo{
									AcceptedVIDs:  []int{1, 2, 3},
									NativeVLANVID: int64(nativeVID),
								},
							},
						},
					},
				},
				createdAt,
			),
		},
		{
			desc:    "VLAN trunk without native",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Trunk: &model.NetVLANTrunkInput{
							AcceptedVIDs: []int{1, 2, 3},
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Trunk: &VLANTrunkInfo{
									AcceptedVIDs: []int{1, 2, 3},
								},
							},
						},
					},
				},
				createdAt,
			),
		},
		{
			desc:    "VLAN access",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Access: &model.NetVLANAccessInput{
							Vid: 100,
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Access: &VLANAccessInfo{
									VID: 100,
								},
							},
						},
					},
				},
				createdAt,
			),
		},
		{
			desc:    "update node with same info does nothing",
			subtype: "subA",
			originNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Trunk: &VLANTrunkInfo{
									AcceptedVIDs:  []int{1, 2, 3},
									NativeVLANVID: int64(nativeVID),
								},
							},
						},
					},
				},
				createdAt,
			),
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Trunk: &model.NetVLANTrunkInput{
							AcceptedVIDs:  []int{1, 2, 3},
							NativeVlanvid: &nativeVID,
						},
					},
				},
			},
			createdAt: &createdAt,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Trunk: &VLANTrunkInfo{
									AcceptedVIDs:  []int{1, 2, 3},
									NativeVLANVID: int64(nativeVID),
								},
							},
						},
					},
				},
				createdAt,
			),
		},
		{
			desc:    "update VLAN access node with trunk info",
			subtype: "subA",
			originNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Trunk: &VLANTrunkInfo{
									AcceptedVIDs:  []int{1, 2, 3},
									NativeVLANVID: int64(nativeVID),
								},
							},
						},
					},
				},
				createdAt,
			),
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Access: &model.NetVLANAccessInput{
							Vid: 100,
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedUpdated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"DeviceID":    "",
					"Description": map[string]string{},
					"NetworkInfo": &NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Access: &VLANAccessInfo{
									VID: 100,
								},
							},
						},
					},
				},
				createdAt,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			g := newGraph(t)
			r := Resolver{Graph: g}

			if test.originNode != nil {
				err := g.AddNode(test.originNode)
				assert.Nil(t, err)
			}

			node, created, updated, err := r.netCreateInterface(
				test.data,
				test.subtype,
				test.prefix,
				test.deviceid,
				test.createdAt,
			)
			if test.expectedError != "" {
				assert.EqualErrorf(t, err, test.expectedError, "error")
			}
			assert.Equalf(t, test.expectedCreated, created, "created")
			assert.Equalf(t, test.expectedUpdated, updated, "updated")

			// If expectedUpdated is true, node UpdatedAt value should be higher than CreatedAt
			// and revision should be 2
			if test.expectedUpdated {
				assert.Greaterf(t, node.UpdatedAt.UnixMilli(), node.CreatedAt.UnixMilli(), "updated higher created date")
				// Change UpdatedAt in the expected node to avoid incorrect comparasion in the next assert
				test.expectedNode.UpdatedAt = node.UpdatedAt

				assert.Equalf(t, int64(2), node.Revision, "revision updated")
				// Change Revision in the expected node to avoid incorrect comparasion in the next assert
				test.expectedNode.Revision = node.Revision
			}

			assert.Equalf(t, test.expectedNode, node, "node")
		})
	}
}

func TestNetAddLogicalToInterfaceCorrectOutput(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	phyIfaceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "inteface"})
	err := g.AddNode(phyIfaceNode)
	assert.Nil(t, err)

	ifaceInput := model.NetInterfaceInput{Name: "foo"}
	_, _, _, err = r.netAddLogicalToInterface(phyIfaceNode, &ifaceInput, nil)
	assert.Nil(t, err)

	// graph should have the device and interface nodes
	assert.Lenf(t, g.GetNodes(nil), 2, "nodes in graph")
}

// TestNTestNetAddLogicalToInterfaceDoNotDeleteAlreadyIfaceFailEdge if the interface already
// exists but the edge creation fails, interface node should not be deleted
func TestNetAddLogicalToInterfaceDoNotDeleteAlreadyIfaceFailEdge(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	ifaceNode := createNodeWithTime(
		getInterfaceID("foo", "eth0"),
		graph.Metadata{
			"Name":        "eth0",
			"Type":        "interface",
			"SubType":     "subA",
			"NetworkInfo": NetworkInfo{},
		},
		time.Unix(100, 0),
	)
	err := g.AddNode(ifaceNode)
	assert.Nil(t, err)

	phyIfaceNode := createNode("phyIface1", graph.Metadata{"Name": "eth0", "Type": "inteface"})
	ifaceInput := model.NetInterfaceInput{Name: "foo"}
	_, _, _, err = r.netAddLogicalToInterface(phyIfaceNode, &ifaceInput, nil)
	assert.EqualError(t, err, "link base interface with interface, interface node not created: unable to create edge type ownership, between 'phyIface1' and '3989a4ffba1724397f3148c24460bc9cc6d410a0b56c4ae4c8515ae813d5a1e7': Parent node not found")

	// ifaceNode should be kept in the graph
	graphNodes := g.GetNodes(nil)
	assert.Lenf(t, graphNodes, 1, "node in graph")
	assert.Equal(t, ifaceNode, graphNodes[0])
}

func TestNetAddLogicalToInterfaceInitialChecks(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	_, _, _, err := r.netAddLogicalToInterface(nil, nil, nil)
	assert.EqualErrorf(t, err, "invalid nil physical interface node", "empty parameters")

	phyIfaceNode := createNode("phyIface1", graph.Metadata{"Name": "eth0", "Type": "inteface"})
	_, _, _, err = r.netAddLogicalToInterface(phyIfaceNode, nil, nil)
	assert.EqualErrorf(t, err, "interface data is not defined", "empty iface data")

	ifaceInput := model.NetInterfaceInput{}
	_, _, _, err = r.netAddLogicalToInterface(phyIfaceNode, &ifaceInput, nil)
	assert.EqualErrorf(t, err, "unable to create logical interface: interface name could not be empty", "empty subtype")

	ifaceInput = model.NetInterfaceInput{Name: "foo"}
	_, _, _, err = r.netAddLogicalToInterface(phyIfaceNode, &ifaceInput, nil)
	assert.EqualErrorf(t, err, "link base interface with interface, interface node not created: unable to create edge type ownership, between 'phyIface1' and '3989a4ffba1724397f3148c24460bc9cc6d410a0b56c4ae4c8515ae813d5a1e7': Parent node not found", "empty subtype")

	// No nodes has been created in the graph
	assert.Lenf(t, g.GetNodes(nil), 0, "no nodes in graph")
}

// TestNetAddLogicalToInterfaceWithPWRespectCreatedAt when creating logical interfaces with PW information,
// the createdAt value should be used in the CreatedAt and UpdatedAt fields.
func TestNetAddLogicalToInterfaceWithPWRespectCreatedAt(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	phyIfaceNode := createNode("phyIface1", graph.Metadata{"Name": "eth0", "Type": "inteface"})
	err := g.AddNode(phyIfaceNode)
	assert.Nil(t, err)

	ifaceInput := model.NetInterfaceInput{
		Name:        "eth0.123",
		PseudoWires: []*model.NetPseudowireInput{{ID: strPointer("123"), Neighbor: strPointer("1.1.1.1")}},
	}
	_, _, _, err = r.netAddLogicalToInterface(phyIfaceNode, &ifaceInput, nil)
	assert.Nil(t, err)

	graphNodes := g.GetNodes(graph.Metadata{"Name": "eth0.123"})
	assert.Equal(t, graphNodes[0].CreatedAt, graphNodes[0].UpdatedAt)

	// También debe mantener la info de PW
	assert.Equal(t, graph.Metadata{
		"Name":        "eth0.123",
		"Type":        "interface",
		"SubType":     "logical",
		"Description": map[string]string{},
		"DeviceID":    "",
		"NetworkInfo": &NetworkInfo{PseudoWires: []PseudoWireInfo{{ID: "123", Neighbor: "1.1.1.1"}}},
	}, graphNodes[0].Metadata)
}

// TestNetAddLogicalToInterfaceWithRespectCreatedAt when using createdAt value, CreatedAt and UpdatedAt values
// in the node should take that value.
func TestNetAddLogicalToInterfaceWithRespectCreatedAt(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	phyIfaceNode := createNode("phyIface1", graph.Metadata{"Name": "eth0", "Type": "inteface"})
	err := g.AddNode(phyIfaceNode)
	assert.Nil(t, err)

	ifaceInput := model.NetInterfaceInput{
		Name: "eth0.123",
	}
	_, _, _, err = r.netAddLogicalToInterface(phyIfaceNode, &ifaceInput, nil)
	assert.Nil(t, err)

	graphNodes := g.GetNodes(graph.Metadata{"Name": "eth0.123"})
	assert.Equal(t, graphNodes[0].CreatedAt, graphNodes[0].UpdatedAt)
}

func TestNetGetOrCreateAggInterfaceNewNode(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}
	createdAt := time.Unix(100, 0)

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	err := g.AddNode(deviceNode)
	assert.Nil(t, err)

	aggNode, err := r.netGetOrCreateAggInterface(deviceNode, "aggA", &createdAt)
	assert.Nil(t, err)

	expectedNode := createNodeWithTime(
		getInterfaceID("devA", "aggA"),
		graph.Metadata{
			"Name":        "aggA",
			"Type":        "interface",
			"SubType":     "aggregation",
			"Description": map[string]string{},
			"NetworkInfo": &NetworkInfo{},
			"DeviceID":    "",
		},
		createdAt,
	)

	assert.Equal(t, expectedNode, aggNode)

	// Device is linked to aggNode with ownership type edge
	edge := g.GetFirstLink(deviceNode, aggNode, nil)
	expectedEdgeMetadata := graph.Metadata{
		"RelationType": "ownership",
	}
	assert.Equalf(t, expectedEdgeMetadata, edge.Metadata, "edge type")
}

func TestNetGetOrCreateAggInterfaceExistingNodeNotLinkedToDevice(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}
	createdAt := time.Unix(100, 0)

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	err := g.AddNode(deviceNode)
	assert.Nil(t, err)

	aggNodePresent := createNodeWithTime(
		getInterfaceID("devA", "aggA"),
		graph.Metadata{
			"Name":        "aggA",
			"Type":        "interface",
			"SubType":     "aggregation",
			"NetworkInfo": NetworkInfo{},
		},
		createdAt,
	)
	err = g.AddNode(aggNodePresent)
	assert.Nil(t, err)

	aggNode, err := r.netGetOrCreateAggInterface(deviceNode, "aggA", &createdAt)
	assert.Nil(t, err)

	assert.Equal(t, aggNodePresent, aggNode)

	// Device is linked to aggNode with ownership type edge
	edge := g.GetFirstLink(deviceNode, aggNode, nil)
	assert.NotNil(t, edge, "edge between device and agg")

	if edge != nil {
		expectedEdgeMetadata := graph.Metadata{
			"RelationType": "ownership",
		}
		assert.Equalf(t, expectedEdgeMetadata, edge.Metadata, "edge type")
	}
}

func TestNetGetOrCreateAggInterfaceInitialChecks(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	_, err := r.netGetOrCreateAggInterface(nil, "", nil)
	assert.EqualErrorf(t, err, "invalid nil device to get/create aggregation interface", "empty parameters")

	deviceNodeNoName := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	_, err = r.netGetOrCreateAggInterface(deviceNodeNoName, "", nil)
	assert.EqualErrorf(t, err, "aggregation interface name could not be empty", "empty device name")

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	_, err = r.netGetOrCreateAggInterface(deviceNode, "", nil)
	assert.EqualErrorf(t, err, "aggregation interface name could not be empty", "empty aggregation name")

	_, err = r.netGetOrCreateAggInterface(deviceNode, "aggA", nil)
	assert.EqualErrorf(t, err, "unable to create aggregation interface: unable to link device with interface: unable to create edge type ownership, between 'device' and 'afdb36e22995eb3a74edb2220aa4ef3b3880c80b4ab9033a9701c661539b0337': Parent node not found", "device not in graph")

	// No nodes has been created in the graph
	assert.Lenf(t, g.GetNodes(nil), 0, "no nodes in graph")
}

func TestNetAddInterfaceToDeviceInitialChecks(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	_, _, _, err := r.netAddInterfaceToDevice(nil, nil, "", nil)
	assert.EqualErrorf(t, err, "invalid nil device node", "empty parameters")

	deviceNodeNoName := createNode("device", graph.Metadata{"Type": "device"})
	_, _, _, err = r.netAddInterfaceToDevice(deviceNodeNoName, nil, "", nil)
	assert.EqualErrorf(t, err, "interface data is not defined", "empty iface data")

	ifaceInput := model.NetInterfaceInput{}
	_, _, _, err = r.netAddInterfaceToDevice(deviceNodeNoName, &ifaceInput, "", nil)
	assert.EqualErrorf(t, err, "interface subtype could not be empty", "empty subtype")

	_, _, _, err = r.netAddInterfaceToDevice(deviceNodeNoName, &ifaceInput, "foo", nil)
	assert.EqualErrorf(t, err, "unable to get device name to create an interface: Field not found", "device node no name")

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	_, _, _, err = r.netAddInterfaceToDevice(deviceNode, &ifaceInput, "foo", nil)
	assert.EqualErrorf(t, err, "unable to create interface: interface name could not be empty", "empty interface definition")

	ifaceInput = model.NetInterfaceInput{Name: "eth0"}
	_, _, _, err = r.netAddInterfaceToDevice(deviceNode, &ifaceInput, "foo", nil)
	assert.EqualErrorf(t, err, "unable to link device with interface: unable to create edge type ownership, between 'device' and '58f56e173d570d6077c46ee925b88a8543782ce97e7f164cc93501069c2a8cc0': Parent node not found", "device node not in graph")

	// No nodes has been created in the graph
	assert.Lenf(t, g.GetNodes(nil), 0, "no nodes in graph")
}

func TestNetAddInterfaceToDevice(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	err := g.AddNode(deviceNode)
	assert.Nilf(t, err, "add device node")

	ifaceInput := model.NetInterfaceInput{
		Name: "eth0",
	}
	node, created, updated, err := r.netAddInterfaceToDevice(deviceNode, &ifaceInput, "foo", nil)
	assert.Nilf(t, err, "error")
	assert.Truef(t, created, "created")
	assert.Falsef(t, updated, "updated")

	expectedMetadata := graph.Metadata{
		"Name":        "eth0",
		"Type":        "interface",
		"SubType":     "foo",
		"Description": map[string]string{},
		"NetworkInfo": &NetworkInfo{},
		"DeviceID":    "",
	}

	assert.Equalf(t, expectedMetadata, node.Metadata, "expected metadata")
}

func TestNetGetNodeFromSelectorValidID(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	node := createNode("nodeA", graph.Metadata{"Name": "devA", "Type": "device"})
	err := g.AddNode(node)
	assert.Nilf(t, err, "add node")

	nodeID := "nodeA"
	selector := model.NetSelector{ID: &nodeID}
	selectecNode, err := r.netGetNodeFromSelector(&selector)
	assert.Nilf(t, err, "select node")
	assert.Equalf(t, node, selectecNode, "nodes equal")
}

func TestNetGetNodeFromSelectorInitialChecks(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	_, err := r.netGetNodeFromSelector(nil)
	assert.EqualError(t, err, "invalid nil selector")

	selector := model.NetSelector{}
	_, err = r.netGetNodeFromSelector(&selector)
	assert.EqualError(t, err, "invalid selector, no ID neither gremlin query")

	id := "unkown"
	selector = model.NetSelector{
		ID:           &id,
		GremlinQuery: nil,
	}
	_, err = r.netGetNodeFromSelector(&selector)

	assert.EqualError(t, err, "no node returned by selector ID")

	query := "G.E()"
	selector = model.NetSelector{
		GremlinQuery: &query,
	}
	_, err = r.netGetNodeFromSelector(&selector)
	assert.EqualError(t, err, "invalid graph output, expecting nodes")
}

func TestPseudoWireHasherOnePW(t *testing.T) {
	node := createNode("Dev2-Iface-eth0", graph.Metadata{
		MetaKeyName: "Dev2-Iface-eth0",
		MetaKeyType: MetadataTypeNetworkInterface,
		MetadataKeyNetworkInfo: &NetworkInfo{
			PseudoWires: []PseudoWireInfo{
				{ID: "1", Neighbor: "1.1.1.1"},
			},
		},
	})

	dict := pseudoWireHasher(node)

	dictExpected := map[string]interface{}{
		"1": map[string]interface{}{"1.1.1.1": nil},
	}

	assert.Equal(t, dictExpected, dict)
}

func TestPseudoWireHasherTwoPWDifferentID(t *testing.T) {
	node := createNode("Dev2-Iface-eth0", graph.Metadata{
		MetaKeyName: "Dev2-Iface-eth0",
		MetaKeyType: MetadataTypeNetworkInterface,
		MetadataKeyNetworkInfo: &NetworkInfo{
			PseudoWires: []PseudoWireInfo{
				{ID: "1", Neighbor: "1.1.1.1"},
				{ID: "2", Neighbor: "1.1.1.2"},
			},
		},
	})

	dict := pseudoWireHasher(node)

	dictExpected := map[string]interface{}{
		"1": map[string]interface{}{"1.1.1.1": nil},
		"2": map[string]interface{}{"1.1.1.2": nil},
	}

	assert.Equal(t, dictExpected, dict)
}

func TestPseudoWireHasherTwoPWSameID(t *testing.T) {
	node := createNode("Dev2-Iface-eth0", graph.Metadata{
		MetaKeyName: "Dev2-Iface-eth0",
		MetaKeyType: MetadataTypeNetworkInterface,
		MetadataKeyNetworkInfo: &NetworkInfo{
			PseudoWires: []PseudoWireInfo{
				{ID: "1", Neighbor: "1.1.1.1"},
				{ID: "1", Neighbor: "1.1.1.2"},
			},
		},
	})

	dict := pseudoWireHasher(node)

	dictExpected := map[string]interface{}{
		"1": map[string]interface{}{
			"1.1.1.1": nil,
			"1.1.1.2": nil,
		},
	}

	assert.Equal(t, dictExpected, dict)
}

func TestPseudoWireHasherEVPN(t *testing.T) {
	node := createNode("Dev2-Iface-eth0", graph.Metadata{
		MetaKeyName: "Dev2-Iface-eth0",
		MetaKeyType: MetadataTypeNetworkInterface,
		MetadataKeyNetworkInfo: &NetworkInfo{
			PseudoWires: []PseudoWireInfo{
				{LocalVpws: "1234", RemoteVpws: "4321"},
			},
		},
	})

	dict := pseudoWireHasher(node)

	dictExpected := map[string]interface{}{
		"4321": map[string]interface{}{
			"1234": nil,
		},
	}

	assert.Equal(t, dictExpected, dict)
}

func TestPseudoWireWithRegionsProvincesBuildingsAndTwoInterfacesPerDeviceWithTwoPW(t *testing.T) {
	getProbeWithTwoDevicesLinkedByPseudowire(t)
}

// getProbeWithTwoDevicesLinkedByPseudowire genera un grafo, la probe Graphql e inyecta dos devices con sus
// buildings, provinces y regions.
// Cada device tiene una interfaz conectadas con un pseudowire.
// Usamos esta función como punto de partida para otros tests
func getProbeWithTwoDevicesLinkedByPseudowire(t *testing.T) *Probe {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Creamos las regiones, provincias y buildings
	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionAProvinceA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionA",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionBProvinceB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionB",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:     "RegionAProvinceABuildingA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province: "RegionAProvinceA",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:     "RegionBProvinceBBuildingB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province: "RegionBProvinceB",
	})
	assert.NoError(t, err)

	// Device 1
	routerType := model.NetDeviceTypeRouter
	buildA := "RegionAProvinceABuildingA"
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
		Building: &buildA,
	})

	// Create interface eth0 in Dev1
	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth1",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("2"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})

	// Device 2
	buildB := "RegionBProvinceBBuildingB"
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
		Building: &buildB,
	})

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eno0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("1.1.1.1")},
			},
		},
	})

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eno1",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("2"), Neighbor: strPointer("1.1.1.1")},
			},
		},
	})

	// 10 edge ownership 2x(r--p--b--d-(2x)-iface)
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypeOwnership}), 10, "edge ownership")

	// 20 edge pseudowires, por cada pw id:
	//    - [r,p,b] <-> [r,p,b] (9)
	//    - iface<->iface (1)
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 20, "edge pseudowire")

	return probe
}

// TestPseudoWireDisconnectOnePW comprueba que al desconfigurar un pseudowire en una interfaz, se borra el edge tipo
// pseudowire y se elimina su id de los edge pseudowire parent.
func TestPseudoWireDisconnectOnePW(t *testing.T) {
	p := getProbeWithTwoDevicesLinkedByPseudowire(t)

	nodes := p.graph.GetNodes(graph.Metadata{"Name": "eth1"})
	// nodes := p.graph.GetNodes(graph.Metadata{"Name": "eno1"}) // TODO poner tambien este test
	assert.Len(t, nodes, 1)
	dev2IfaceEno1 := nodes[0]

	err := p.graph.UpdateMetadata(dev2IfaceEno1, MetadataKeyNetworkInfo, func(field interface{}) bool {
		ni, ok := field.(*NetworkInfo)
		assert.True(t, ok)
		ni.PseudoWires = []PseudoWireInfo{}
		return true
	})
	assert.NoError(t, err)

	pws := p.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire})

	// Debemos tener un PW menos, 10, tras eliminar el que conectaba eno1 con eth1
	assert.Lenf(t, pws, 10, "after deleting pw config")

	// Los edges pseudowire deben ser todos del pwid=1, el que no se ha borrado
	for _, pw := range pws {
		metaPW, err := pw.GetFieldString(MetaKeyPseudowireID)
		assert.NoError(t, err)
		assert.Equalf(t, "1", metaPW, "pw id should be 1")
	}
}

// TODO gestionar que pasa cuando se borra un PW de una interfaz, quitar el enlace y los de sus padres
// TODO gestionar que pasa cuando se borra un edge tipo pw, deben borrarse hacia arriba.
// En realidad no deben borrarse los parents, solo debe quitarse el pwid de la lista y si la lista queda vacía
// entonces borrar el edge.
// Estoy creando links entre A y B y entre B y A. Depende cual se ejecute primero.
// No debería crear los dos sentidos.

// TODO Crear un test para el tema de AB vs BA

// TestPseudoWireLowerIDTriggersFirst comprobar que se generan correctamente los pseudowires entre dos interfaces.
// Se ejecutan actualizaciones en ambos nodos para comprobar que no se generan más edges de los esperados.
// Luego se procede a quitar la configuración de PW de la primera interfaz y comprobar que se borran los edges-pw.
// De los dos nodos creados, el segundo es el que dispara la creación.
// Este segundo nodo tiene un ID menor que el primero.
// Esto es importante por como se decide donde se generan los edges.
// Resumen de las acciones del test:
//  - create dev1 + iface1 (id mayor)
//  - create dev2 + iface2 (id menor)
//  - add metadata to iface1
//  - add metadata to iface2
//  - remove PW from iface1
func TestPseudoWireLowerIDTriggersFirst(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	dev1Iface, err := probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})
	assert.NoError(t, err)

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	dev2Iface, err := probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "xgigaeno0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("1.1.1.1")},
			},
		},
	})
	assert.NoError(t, err)

	// Aseguramos que el id que dispara GetAB/BALinks es el menor
	assert.Lessf(t, dev2Iface.Interface.ID, dev1Iface.Interface.ID, "invalid interface id order")

	// Comprobamos que solo tenemos un pseudowire, lo esperado
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1, "pw after second iface creation")

	// Modificamos la interfaz del device 1 para forzar que se ejecute GetABLinks y GetBALinks
	dev1IfaceNode := probe.graph.GetNode(graph.Identifier(dev1Iface.Interface.ID))
	err = probe.graph.AddMetadata(dev1IfaceNode, "foo", "bar")
	assert.NoError(t, err)

	// No se deben haber generado nuevos edges PseudoWires
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1, "after updating first iface")

	// Modificamos la interfaz del device 2 para forzar que se ejecute GetABLinks y GetBALinks
	dev2IfaceNode := probe.graph.GetNode(graph.Identifier(dev2Iface.Interface.ID))
	err = probe.graph.AddMetadata(dev2IfaceNode, "foo", "bar")
	assert.NoError(t, err)

	// No se deben haber generado nuevos edges PseudoWires
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1, "after updating second iface")

	// Si borramos el PW de la definición de la interfaz 1, debe desaparecer el pw-edge entre las interfaces
	err = probe.graph.UpdateMetadata(dev1IfaceNode, MetadataKeyNetworkInfo, func(field interface{}) bool {
		ni, ok := field.(*NetworkInfo)
		assert.True(t, ok)
		ni.PseudoWires = []PseudoWireInfo{}
		return true
	})
	assert.NoError(t, err)
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 0, "after removing PW from iface")
}

// TestPseudoWireHigherIDTriggersFirst como el test anterior, pero esta vez el identificador
// con el mayor ID se crea después, por lo que es quien dispara la creación de edges
// en un primer momento.
//  - create dev2 + iface2 (id menor)
//  - create dev1 + iface1 (id mayor)
//  - add metadata to iface2
//  - add metadata to iface1
//  - remove PW from iface2
func TestPseudoWireHigherIDTriggersFirst(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceAID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	dev1Iface, err := probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "agigaeth0", // Nombre elegido para que genere un id menor
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})
	assert.NoError(t, err)

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceBID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	dev2Iface, err := probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eno0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("1.1.1.1")},
			},
		},
	})
	assert.NoError(t, err)

	// Aseguramos que el id que dispara GetAB/BALinks es el mayor
	assert.Greaterf(t, dev2Iface.Interface.ID, dev1Iface.Interface.ID, "invalid interface id order")

	// Comprobamos que solo tenemos un pseudowire, lo esperado
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1, "pw after second iface creation")

	// Modificamos la interfaz del device 1 para forzar que se ejecute GetABLinks y GetBALinks
	dev1IfaceNode := probe.graph.GetNode(graph.Identifier(dev1Iface.Interface.ID))
	err = probe.graph.AddMetadata(dev1IfaceNode, "foo", "bar")
	assert.NoError(t, err)

	// No se deben haber generado nuevos edges PseudoWires
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1, "after updating first iface")

	// Modificamos la interfaz del device 2 para forzar que se ejecute GetABLinks y GetBALinks
	dev2IfaceNode := probe.graph.GetNode(graph.Identifier(dev2Iface.Interface.ID))
	err = probe.graph.AddMetadata(dev2IfaceNode, "foo", "bar")
	assert.NoError(t, err)

	// No se deben haber generado nuevos edges PseudoWires
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1, "after updating second iface")

	// Si borramos el PW de la definición de la interfaz 1, debe desaparecer el pw-edge entre las interfaces
	err = probe.graph.UpdateMetadata(dev1IfaceNode, MetadataKeyNetworkInfo, func(field interface{}) bool {
		ni := field.(*NetworkInfo)
		ni.PseudoWires = []PseudoWireInfo{}
		return true
	})
	assert.NoError(t, err)
	assert.Lenf(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 0, "after removing PW from iface")
}

// TestPseudoWireWithSameBuildingProvinceRegion comprobamos que no se duplican edges-pw al establecer PW dentro del
// mismo building.
func TestPseudoWireWithSameBuildingProvinceRegion(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Creamos las regiones, provincias y buildings
	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionAProvinceA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionA",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:     "RegionAProvinceABuildingA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province: "RegionAProvinceA",
	})
	assert.NoError(t, err)

	// Device 1
	routerType := model.NetDeviceTypeRouter
	buildA := "RegionAProvinceABuildingA"
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
		Building: &buildA,
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})
	assert.NoError(t, err)

	// Device 2
	buildB := "RegionAProvinceABuildingA"
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
		Building: &buildB,
	})

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eno0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("1.1.1.1")},
			},
		},
	})

	// Tiene que haber un solo edge-pw, entre las interfaces
	assert.Len(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1)
}

// TestPseudoWireDeletePrevious comprobamos que los PWs anteriormente definidos se borran al volver a declarar una interfaz
func TestPseudoWireDeletePrevious(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	payload, err := probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("1.1.1.1")},
				{ID: strPointer("2"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})
	assert.NoError(t, err)

	ifaceNode := probe.graph.GetNode(graph.Identifier(payload.Interface.ID))
	netinfoRaw, err := ifaceNode.GetField(MetadataKeyNetworkInfo)
	assert.NoError(t, err)

	netinfo := netinfoRaw.(*NetworkInfo)
	assert.Len(t, netinfo.PseudoWires, 2)

	// Modificamos la interfaz, quitando uno de los PW y creando otro
	payload, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("2"), Neighbor: strPointer("2.2.2.2")},
				{ID: strPointer("99"), Neighbor: strPointer("9.9.9.9")},
			},
		},
	})
	assert.NoError(t, err)

	ifaceNode = probe.graph.GetNode(graph.Identifier(payload.Interface.ID))
	netinfoRaw, err = ifaceNode.GetField(MetadataKeyNetworkInfo)
	assert.NoError(t, err)

	netinfo = netinfoRaw.(*NetworkInfo)

	expectedPWs := []PseudoWireInfo{
		{ID: "2", Neighbor: "2.2.2.2"},
		{ID: "99", Neighbor: "9.9.9.9"},
	}
	assert.Equal(t, expectedPWs, netinfo.PseudoWires)
}

// TestDeleteInterface comprueba que la mutation de borrado de interfaces funciona correctamente
func TestDeleteInterface(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	payload, err := probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("1.1.1.1")},
				{ID: strPointer("2"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})
	assert.NoError(t, err)

	// Borramos la interfaz
	payload, err = probe.resolver.Mutation().NetDelInterface(context.Background(), model.NetDelInterfaceInput{
		InterfaceSelector: &model.NetSelector{
			ID: &payload.Interface.ID,
		},
	})

	// Comprobamos que solo tenemos el nodo del device en el grafo
	assert.Len(t, probe.graph.GetNodes(nil), 1)
}

// TestDeleteInterfaceLinkedWithPWEdge comprueba que se puede borrar correctamente una interfaz que está linkada con un edge-pw.
// Esta comprobación mezcla el borrado de interfaces con la gestión de borrado de PWs parents, para comprobar que no existe
// problemas de bloqueos.
func TestDeleteInterfaceLinkedWithPWEdge(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
	})

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("2.2.2.2")},
			},
		},
	})

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
	})

	payload, err := probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eno0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{ID: strPointer("1"), Neighbor: strPointer("1.1.1.1")},
			},
		},
	})

	// Tiene que haber un solo edge-pw, entre las interfaces
	assert.Len(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 1)

	// Borramos la interfaz
	_, err = probe.resolver.Mutation().NetDelInterface(context.Background(), model.NetDelInterfaceInput{
		InterfaceSelector: &model.NetSelector{
			ID: &payload.Interface.ID,
		},
	})

	// Comprobamos que solo tenemos tres nodos (dev1+iface y dev2)
	assert.Len(t, probe.graph.GetNodes(nil), 3)

	// El edge-pw tiene que haber sido borrado
	assert.Len(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire}), 0)
}

// TestDeleteLLDPLink comprueba que el borrado de edges LLDP funciona correctamente
func TestDeleteLLDPLink(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eth0"},
	})
	assert.NoError(t, err)

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eno0"},
	})
	assert.NoError(t, err)

	// Creamos un link sin especificar, por defecto es un link LLDP (connect)
	_, err = probe.resolver.Mutation().NetAddIf2IfLink(context.Background(), model.NetAddIf2IfLinkInput{
		SrcInterface: &model.NetSelector{ID: &dev1.Device.ID},
		DstInterface: &model.NetSelector{ID: &dev2.Device.ID},
		Name:         strPointer("eth0"),
		State:        strPointer("up"),
	})
	assert.NoError(t, err)

	// Comprobamos que se ha creado un edge tipo LLDP (connect) con el Name y State especificados
	edges := probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: string(model.NetLinkTypeConnect)})
	assert.Len(t, edges, 1)
	edgeName, err := edges[0].GetFieldString(MetaKeyName)
	assert.NoError(t, err)
	assert.Equal(t, "eth0", edgeName)
	edgeState, err := edges[0].GetFieldString(MetaKeyState)
	assert.NoError(t, err)
	assert.Equal(t, "up", edgeState)

	// Creamos un link tipo radio link
	rl := model.NetLinkTypeRadioLink
	_, err = probe.resolver.Mutation().NetAddIf2IfLink(context.Background(), model.NetAddIf2IfLinkInput{
		SrcInterface: &model.NetSelector{ID: &dev1.Device.ID},
		DstInterface: &model.NetSelector{ID: &dev2.Device.ID},
		Type:         &rl,
	})
	assert.NoError(t, err)

	// Comprobamos que se ha creado un edge tipo LLDP (connect)
	assert.Len(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: string(model.NetLinkTypeRadioLink)}), 1)

	// Llama a la mutation de borrado de edges LLDP (se borra por defecto connect)
	_, err = probe.resolver.Mutation().NetDelIf2IfLink(context.Background(), model.NetDelIf2IfLinkInput{
		SrcInterface: &model.NetSelector{ID: &dev1.Device.ID},
		DstInterface: &model.NetSelector{ID: &dev2.Device.ID},
	})

	// Comprobamos que no quedan edges tipo LLDP (connect), pero si el radio link
	assert.Len(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: string(model.NetLinkTypeConnect)}), 0)
	assert.Len(t, probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: string(model.NetLinkTypeRadioLink)}), 1)
}

// TestMultipleRadioLinks comprobar que si creamos varios radios links, todos se mantienen a nivel de CCAA y provincia.
// Tendremos que tener dos edges a nivel de CCAA, uno con cada nombre
func TestMultipleRadioLinks(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Creamos la region y provincia
	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionAProvinceA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionA",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionBProvinceB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionB",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:        "BuildingA",
		VisibleName: strPointer("Building A"),
		IDSite:      strPointer("SiteA"),
		Location:    &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province:    "RegionAProvinceA",
		Class:       strPointer("classA"),
	})

	assert.NoError(t, err)
	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:        "BuildingB",
		VisibleName: strPointer("Building B"),
		IDSite:      strPointer("SiteB"),
		Location:    &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province:    "RegionBProvinceB",
		Class:       strPointer("classB"),
	})
	assert.NoError(t, err)

	assert.NoError(t, err)
	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:        "BuildingC",
		VisibleName: strPointer("Building C"),
		IDSite:      strPointer("SiteC"),
		Location:    &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province:    "RegionBProvinceB",
		Class:       strPointer("classC"),
	})
	assert.NoError(t, err)

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
		Building: strPointer("BuildingA"),
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eth0"},
	})
	assert.NoError(t, err)

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
		Building: strPointer("BuildingB"),
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eno0"},
	})
	assert.NoError(t, err)

	// Device 3
	deviceCID := "3.3.3.3"
	dev3, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev3",
		DeviceID: &deviceCID,
		Type:     &routerType,
		Building: strPointer("BuildingC"),
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eno0"},
	})
	assert.NoError(t, err)

	// Creamos un radiolink entre el device 1 y el device 2
	radioLinkType := model.NetLinkTypeRadioLink
	_, err = probe.resolver.Mutation().NetAddIf2IfLink(context.Background(), model.NetAddIf2IfLinkInput{
		SrcInterface: &model.NetSelector{ID: &dev1.Device.ID},
		DstInterface: &model.NetSelector{ID: &dev2.Device.ID},
		Name:         strPointer("1to2"),
		State:        strPointer("up"),
		Type:         &radioLinkType,
	})
	assert.NoError(t, err)

	// Creamos un radiolink entre el device 1 y el device 3, donde el device 2 y
	// el device 3 están en la misma región y provincia
	_, err = probe.resolver.Mutation().NetAddIf2IfLink(context.Background(), model.NetAddIf2IfLinkInput{
		SrcInterface: &model.NetSelector{ID: &dev1.Device.ID},
		DstInterface: &model.NetSelector{ID: &dev3.Device.ID},
		Name:         strPointer("1to3"),
		State:        strPointer("up"),
		Type:         &radioLinkType,
	})
	assert.NoError(t, err)

	// Comprobamos que existen 8 edges tipo radio link:
	//   - entre las interfaces de dev1 y dev2 (1to2)
	//   - entre las interfaces de dev1 y dev3 (1to3)
	//   - entre el buildingA y el buildingB (1to2)
	//   - (2x) entre el buildingA, RegionBProvinceB y RegionB (1to2)
	//   - (2x) entre el buildingB, RegionAProvinceA y RegionA (1to2)
	//   - entre el buildingA y el buildingC (1to3)
	//   - (2x) entre el buildingA, RegionBProvinceB y RegionB (1to3)
	//   - (2x) entre el buildingC, RegionAProvinceA y RegionA (1to3)
	//   - entre la RegionAProvinceA y RegionBProvinceB (1to2)
	//   - entre la RegionAProvinceA y RegionB (1to2)
	//   - entre la RegionA y la RegionBProvinceB (1to2)
	//   - entre la RegionA y la RegionB (1to2)
	//   - entre la RegionAProvinceA y RegionBProvinceB (1to3)
	//   - entre la RegionAProvinceA y RegionB (1to3)
	//   - entre la RegionA y la RegionBProvinceB (1to3)
	//   - entre la RegionA y la RegionB (1to3)

	edges := probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: string(model.NetLinkTypeRadioLink)})
	assert.Len(t, edges, 20)
}

// TestMultipleLLDPLinks como el anterior, pero usando links LLDP en vez de radiolinks.
// En este caso, no se deben crear múltiples links entre regiones y provincias, ya que al no tener más información,
// un único link es suficiente para declarar que tenemos esa conectividad.
func TestMultipleLLDPLinks(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Creamos la region y provincia
	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionAProvinceA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionA",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionBProvinceB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionB",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:        "BuildingA",
		VisibleName: strPointer("Building A"),
		IDSite:      strPointer("SiteA"),
		Location:    &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province:    "RegionAProvinceA",
		Class:       strPointer("classA"),
	})

	assert.NoError(t, err)
	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:        "BuildingB",
		VisibleName: strPointer("Building B"),
		IDSite:      strPointer("SiteB"),
		Location:    &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province:    "RegionBProvinceB",
		Class:       strPointer("classB"),
	})
	assert.NoError(t, err)

	assert.NoError(t, err)
	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:        "BuildingC",
		VisibleName: strPointer("Building C"),
		IDSite:      strPointer("SiteC"),
		Location:    &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province:    "RegionBProvinceB",
		Class:       strPointer("classC"),
	})
	assert.NoError(t, err)

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
		Building: strPointer("BuildingA"),
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eth0"},
	})
	assert.NoError(t, err)

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
		Building: strPointer("BuildingB"),
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eno0"},
	})
	assert.NoError(t, err)

	// Device 3
	deviceCID := "3.3.3.3"
	dev3, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev3",
		DeviceID: &deviceCID,
		Type:     &routerType,
		Building: strPointer("BuildingC"),
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface:      &model.NetInterfaceInput{Name: "eno0"},
	})
	assert.NoError(t, err)

	connectLinkType := model.NetLinkTypeConnect
	_, err = probe.resolver.Mutation().NetAddIf2IfLink(context.Background(), model.NetAddIf2IfLinkInput{
		SrcInterface: &model.NetSelector{ID: &dev1.Device.ID},
		DstInterface: &model.NetSelector{ID: &dev2.Device.ID},
		Type:         &connectLinkType,
	})
	assert.NoError(t, err)

	// Creamos un radiolink entre el device 1 y el device 3, donde el device 2 y
	// el device 3 están en la misma región y provincia
	_, err = probe.resolver.Mutation().NetAddIf2IfLink(context.Background(), model.NetAddIf2IfLinkInput{
		SrcInterface: &model.NetSelector{ID: &dev1.Device.ID},
		DstInterface: &model.NetSelector{ID: &dev3.Device.ID},
		Type:         &connectLinkType,
	})
	assert.NoError(t, err)

	// Comprobamos que existen 8 edges tipo radio link:
	//   - entre las interfaces de dev1 y dev2
	//   - entre las interfaces de dev1 y dev3
	//   - entre el buildingA y el buildingB
	//   - (2x) entre el buildingA, RegionBProvinceB y RegionB
	//   - (2x) entre el buildingB, RegionAProvinceA y RegionA
	//   - entre el buildingA y el buildingC
	//   - (2x) entre el buildingC, RegionAProvinceA y RegionA
	//   - entre la RegionAProvinceA y RegionBProvinceB
	//   - entre la RegionAProvinceA y RegionB
	//   - entre la RegionA y la RegionBProvinceB
	//   - entre la RegionA y la RegionB
	edges := probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: string(model.NetLinkTypeConnect)})
	assert.Len(t, edges, 14)
}

func TestEVPNPseudoWiresWithoutBuildings(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
	})

	// Create interface eth0 in Dev1
	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{LocalVpws: strPointer("123456"), RemoteVpws: strPointer("654321")},
			},
		},
	})

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
	})

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eno0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{LocalVpws: strPointer("654321"), RemoteVpws: strPointer("123456")},
			},
		},
	})

	edges := probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire})
	assert.Len(t, edges, 1)
	expectedEdgeMetadata := graph.Metadata{
		MetaKeyRelationType: RelationTypePseudoWire,
		MetaKeyPseudowireID: "123456-654321",
	}
	assert.Equal(t, expectedEdgeMetadata, edges[0].Metadata)
}

func TestEVPNPseudoWiresWithBuildings(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Creamos las regiones, provincias y buildings
	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionAProvinceA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionA",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionBProvinceB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionB",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:     "RegionAProvinceABuildingA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province: "RegionAProvinceA",
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:     "RegionBProvinceBBuildingB",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province: "RegionBProvinceB",
	})
	assert.NoError(t, err)

	// Device 1
	routerType := model.NetDeviceTypeRouter
	deviceAID := "1.1.1.1"
	dev1, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev1",
		DeviceID: &deviceAID,
		Type:     &routerType,
		Building: strPointer("RegionAProvinceABuildingA"),
	})

	// Create interface eth0 in Dev1
	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev1.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eth0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{LocalVpws: strPointer("123456"), RemoteVpws: strPointer("654321")},
			},
		},
	})

	// Device 2
	deviceBID := "2.2.2.2"
	dev2, err := probe.resolver.Mutation().NetAddDevice(context.Background(), model.NetDeviceInput{
		Name:     "Dev2",
		DeviceID: &deviceBID,
		Type:     &routerType,
		Building: strPointer("RegionBProvinceBBuildingB"),
	})

	_, err = probe.resolver.Mutation().NetAddInterface(context.Background(), model.NetAddInterfaceInput{
		DeviceSelector: &model.NetSelector{ID: &dev2.Device.ID},
		Interface: &model.NetInterfaceInput{
			Name:        "eno0",
			Description: []*model.NetKeyValue{},
			PseudoWires: []*model.NetPseudowireInput{
				{LocalVpws: strPointer("654321"), RemoteVpws: strPointer("123456")},
			},
		},
	})

	edges := probe.graph.GetEdges(graph.Metadata{MetaKeyRelationType: RelationTypePseudoWire})
	// 10 edge pseudowires:
	//    - [r,p,b] <-> [r,p,b] (9)
	//    - iface<->iface (1)
	assert.Len(t, edges, 10)
}

// TestBuildingWithClasses test if an input building with a class defined is correctly added to the graph.
func TestBuildingWithClasses(t *testing.T) {
	g := newGraph(t)

	probe, err := NewProbe(g)
	assert.NoError(t, err)
	probe.Start()

	// Needed region and province to create the building
	_, err = probe.resolver.Mutation().NetAddRegion(context.Background(), model.NetRegionInput{
		Name:     "RegionA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
	})
	assert.NoError(t, err)

	_, err = probe.resolver.Mutation().NetAddProvince(context.Background(), model.NetProvinceInput{
		Name:     "RegionAProvinceA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Region:   "RegionA",
	})
	assert.NoError(t, err)

	// Create a building with a class
	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:        "BuildingA",
		VisibleName: strPointer("Building A"),
		IDSite:      strPointer("SiteA"),
		Location:    &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province:    "RegionAProvinceA",
		Class:       strPointer("classA"),
	})
	assert.NoError(t, err)

	// Get building nodes from graph
	nodes := probe.graph.GetNodes(graph.Metadata{MetaKeyName: "BuildingA"})
	assert.Len(t, nodes, 1)

	// The building should have the class value as a metadata key. Value should be always true
	building := nodes[0]
	v, err := building.GetFieldString("classA")
	assert.NoError(t, err)
	assert.Equal(t, "t", v)

	// Create the same building with a different class
	_, err = probe.resolver.Mutation().NetAddBuilding(context.Background(), model.NetBuildingInput{
		Name:     "BuildingA",
		Location: &model.NetLocationInput{Lat: 0.0, Lng: 0.0},
		Province: "RegionAProvinceA",
		Class:    strPointer("classB"),
	})
	assert.NoError(t, err)

	// Get building nodes from graph
	nodes = probe.graph.GetNodes(graph.Metadata{MetaKeyName: "BuildingA"})
	assert.Len(t, nodes, 1)

	// The building should have the class value as a metadata key. Value should be always true
	building = nodes[0]
	v, err = building.GetFieldString("classB")
	assert.NoError(t, err)
	assert.Equal(t, "t", v)

	// Old class should be kept
	v, err = building.GetFieldString("classA")
	assert.NoError(t, err)
	assert.Equal(t, "t", v)

	// VisibleName and IDSite should be kept
	v, err = building.GetFieldString("VisibleName")
	assert.NoError(t, err)
	assert.Equal(t, "Building A", v)

	v, err = building.GetFieldString("IDSite")
	assert.NoError(t, err)
	assert.Equal(t, "SiteA", v)
}
