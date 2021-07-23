package graphql

import (
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

// createNodeWithTime helper function to create nodes for testing
func createNodeWithTime(i graph.Identifier, m graph.Metadata, t time.Time) *graph.Node {
	return graph.CreateNode(i, m, graph.Time(t), "testhost", "graphql.analyzer.testhost")
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
		&routerType,
		nil,
		strPointer("vendorNew"),
		&newCreatedAt,
	)
	assert.EqualError(t, err, "forbidden to modify CreatedAt value of existing node")

	// It is valid to call with createdAt if it is the same as the origin node
	node, created, updated, err := r.netAddDevice(
		"deviceA",
		&routerType,
		nil,
		strPointer("vendorNew"),
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
		&routerType,
		nil,
		strPointer("vendorNew"),
		nil,
	)
	assert.Nil(t, err)
}

func TestNetAddDevice(t *testing.T) {
	routerType := model.NetDeviceTypeRouter
	//switchType := model.NetDeviceTypeSwitch
	createdAt := time.Unix(100, 0)

	tests := []struct {
		desc            string
		deviceName      string
		deviceType      *model.NetDeviceType
		model           *string
		vendor          *string
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
				test.deviceType,
				test.model,
				test.vendor,
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

func TestNetCreateInterfacErrorCreateInterfaceNode(t *testing.T) {
	g := newGraph(t)
	r := Resolver{Graph: g}

	data := model.NetInterfaceInput{Name: "eth0"}
	subtype := "bar"
	prefix := "prefix"

	createdAt := time.Unix(100, 0)
	node, created, updated, err := r.netCreateInterface(data, subtype, prefix, &createdAt)
	assert.Nil(t, err)
	assert.NotNil(t, node)
	assert.Truef(t, created, "created")
	assert.Falsef(t, updated, "updated")

	newCreatedAt := time.Unix(200, 0)
	_, _, _, err = r.netCreateInterface(data, subtype, prefix, &newCreatedAt)
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
			desc:            "new interface with only name and empty prefix",
			subtype:         "subA",
			data:            model.NetInterfaceInput{Name: "eth0"},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"NetworkInfo": NetworkInfo{},
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
				getInterfaceID("foo", "eth0", "subA"),
				graph.Metadata{
					"Name":        "eth0",
					"Type":        "interface",
					"SubType":     "subA",
					"NetworkInfo": NetworkInfo{},
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
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
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
							AcceptedVIDs:  []int{1, 2, 3},
							NativeVlanvid: &nativeVID,
						},
						Trunk: &model.NetVLANTrunkInput{
							Vid: 10,
						},
					},
				},
			},
			expectedError: "interface VLAN could not be access and trunk at the same time",
		},
		{
			desc:    "VLAN access with native",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Access: &model.NetVLANAccessInput{
							AcceptedVIDs:  []int{1, 2, 3},
							NativeVlanvid: &nativeVID,
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Access: &VLANAccessInfo{
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
			desc:    "VLAN access without native",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Access: &model.NetVLANAccessInput{
							AcceptedVIDs: []int{1, 2, 3},
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Access: &VLANAccessInfo{
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
			desc:    "VLAN trunk",
			subtype: "subA",
			data: model.NetInterfaceInput{
				Name: "eth0",
				Ethernet: &model.NetEthernetInput{
					Vlan: &model.NetVLANInput{
						Trunk: &model.NetVLANTrunkInput{
							Vid: 100,
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedCreated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Trunk: &VLANTrunkInfo{
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
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Access: &VLANAccessInfo{
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
							AcceptedVIDs:  []int{1, 2, 3},
							NativeVlanvid: &nativeVID,
						},
					},
				},
			},
			createdAt: &createdAt,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Access: &VLANAccessInfo{
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
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Access: &VLANAccessInfo{
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
							Vid: 100,
						},
					},
				},
			},
			createdAt:       &createdAt,
			expectedUpdated: true,
			expectedNode: createNodeWithTime(
				getInterfaceID("", "eth0", "subA"),
				graph.Metadata{
					"Name":    "eth0",
					"Type":    "interface",
					"SubType": "subA",
					"NetworkInfo": NetworkInfo{
						Ethernet: &EthernetInfo{
							VLAN: &VLANInfo{
								Trunk: &VLANTrunkInfo{
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

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	err := g.AddNode(deviceNode)
	assert.Nil(t, err)

	ifaceInput := model.NetInterfaceInput{Name: "foo"}
	_, _, _, err = r.netAddLogicalToInterface(deviceNode, &ifaceInput, nil)
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
		getInterfaceID("foo", "eth0", "subA"),
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

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	ifaceInput := model.NetInterfaceInput{Name: "foo"}
	_, _, _, err = r.netAddLogicalToInterface(deviceNode, &ifaceInput, nil)
	assert.EqualError(t, err, "link device with interface, interface node not created: unable to create edge type use, between '25ad6ba5-1905-5901-5d29-63f7928b2930' and 'device': Child node not found")

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

	deviceNode := createNode("device", graph.Metadata{"Name": "devA", "Type": "device"})
	_, _, _, err = r.netAddLogicalToInterface(deviceNode, nil, nil)
	assert.EqualErrorf(t, err, "interface data is not defined", "empty iface data")

	ifaceInput := model.NetInterfaceInput{}
	_, _, _, err = r.netAddLogicalToInterface(deviceNode, &ifaceInput, nil)
	assert.EqualErrorf(t, err, "unable to create logical interface: interface name could not be empty", "empty subtype")

	ifaceInput = model.NetInterfaceInput{Name: "foo"}
	_, _, _, err = r.netAddLogicalToInterface(deviceNode, &ifaceInput, nil)
	assert.EqualErrorf(t, err, "link device with interface, interface node not created: unable to create edge type use, between '25ad6ba5-1905-5901-5d29-63f7928b2930' and 'device': Child node not found", "empty subtype")

	// No nodes has been created in the graph
	assert.Lenf(t, g.GetNodes(nil), 0, "no nodes in graph")
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
		getInterfaceID("devA", "aggA", MetadataSubtypeAggregationInterface),
		graph.Metadata{
			"Name":        "aggA",
			"Type":        "interface",
			"SubType":     "aggregation",
			"NetworkInfo": NetworkInfo{},
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
		getInterfaceID("devA", "aggA", MetadataSubtypeAggregationInterface),
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
	assert.EqualErrorf(t, err, "unable to create aggregation interface: unable to link device with interface: unable to create edge type ownership, between 'device' and '374d1238-37a3-57e5-6175-a1666ef4ef2d': Parent node not found", "device not in graph")

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
	assert.EqualErrorf(t, err, "unable to link device with interface: unable to create edge type ownership, between 'device' and 'b1e4f178-8930-58e6-50fa-409f08673791': Parent node not found", "device node not in graph")

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
		"NetworkInfo": NetworkInfo{},
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
