/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package tests

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/avast/retry-go"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	shttp "github.com/skydive-project/skydive/graffiti/http"
	g "github.com/skydive-project/skydive/gremlin"
)

func getCrudClient() (c *shttp.CrudClient, err error) {
	retry.Do(func() error {
		c, err = client.NewCrudClientFromConfig(&shttp.AuthenticationOpts{})
		return err
	})
	return
}
func TestAlertAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	alert := types.NewAlert()
	alert.Expression = g.G.V().Has("MTU", g.Gt(1500)).String()
	if err := client.Create("alert", alert, nil); err != nil {
		t.Errorf("Failed to create alert: %s", err.Error())
	}

	alert2 := types.NewAlert()
	alert2.Expression = g.G.V().Has("MTU", g.Gt(1500)).String()
	if err := client.Get("alert", alert.UUID, &alert2); err != nil {
		t.Error(err)
	}

	if *alert != *alert2 {
		t.Errorf("Alert corrupted: %+v != %+v", alert, alert2)
	}

	var alerts map[string]types.Alert
	if err := client.List("alert", &alerts); err != nil {
		t.Error(err)
	} else {
		if len(alerts) != 1 {
			t.Errorf("Wrong number of alerts: got %d, expected 1 (%+v)", len(alerts), alerts)
		}
	}

	if alerts[alert.UUID] != *alert {
		t.Errorf("Alert corrupted: %+v != %+v", alerts[alert.UUID], alert)
	}

	if err := client.Delete("alert", alert.UUID); err != nil {
		t.Errorf("Failed to delete alert: %s", err.Error())
	}

	var alerts2 map[string]types.Alert
	if err := client.List("alert", &alerts2); err != nil {
		t.Errorf("Failed to list alerts: %s", err.Error())
	} else {
		if len(alerts2) != 0 {
			t.Errorf("Wrong number of alerts: got %d, expected 0 (%+v)", len(alerts2), alerts2)
		}
	}
}

func TestCaptureAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	var captures map[string]types.Capture
	if err := client.List("capture", &captures); err != nil {
		t.Error(err)
	}
	nbCaptures := len(captures)

	capture := types.NewCapture(g.G.V().Has("Name", "br-int").String(), "port 80")
	if err := client.Create("capture", capture, nil); err != nil {
		t.Fatalf("Failed to create alert: %s", err.Error())
	}

	capture2 := &types.Capture{}
	if err := client.Get("capture", capture.GetID(), &capture2); err != nil {
		t.Error(err)
	}

	if *capture != *capture2 {
		t.Errorf("Capture corrupted: %+v != %+v", capture, capture2)
	}

	if err := client.List("capture", &captures); err != nil {
		t.Error(err)
	} else {
		if (len(captures) - nbCaptures) != 1 {
			t.Errorf("Wrong number of captures: got %d, expected 1", len(captures))
		}
	}

	if captures[capture.GetID()] != *capture {
		t.Errorf("Capture corrupted: %+v != %+v", captures[capture.GetID()], capture)
	}

	if err := client.Delete("capture", capture.GetID()); err != nil {
		t.Errorf("Failed to delete capture: %s", err.Error())
	}

	var captures2 map[string]types.Capture
	if err := client.List("capture", &captures2); err != nil {
		t.Errorf("Failed to list captures: %s", err.Error())
	} else {
		if (len(captures2) - nbCaptures) != 0 {
			t.Errorf("Wrong number of captures: got %d, expected 0 (%+v)", len(captures2)-nbCaptures, captures2)
		}
	}

	if err := client.Get("capture", capture.GetID(), &capture2); err == nil {
		t.Errorf("Found delete capture: %s", capture.GetID())
	}
}

func TestNodeAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	node := new(types.Node)

	if err := client.Create("node", node, nil); err == nil {
		t.Errorf("Expected error when creating a node without ID")
	}

	node.ID = graph.GenID()
	node.Metadata = graph.Metadata{"Type": "mytype"}

	if err := client.Create("node", node, nil); err == nil {
		t.Errorf("Expected error when creating a node without name")
	}

	node.Metadata["Name"] = "myname"
	if err := client.Create("node", node, nil); err != nil {
		t.Error(err)
	}

	if err := client.Get("node", string(node.ID), &node); err != nil {
		t.Error(err)
	}

	name, _ := node.GetFieldString("Name")
	typ, _ := node.GetFieldString("Type")
	if name != "myname" || typ != "mytype" {
		t.Errorf("Expected node with name 'myname' and type 'mytype'")
	}
}

func TestEdgeAPI(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	node1 := new(types.Node)
	node1.ID = graph.GenID()
	node1.Metadata = graph.Metadata{"Type": "mytype", "Name": "node1"}

	node2 := new(types.Node)
	node2.ID = graph.GenID()
	node2.Metadata = graph.Metadata{"Type": "mytype", "Name": "node2"}

	if err := client.Create("node", node1, nil); err != nil {
		t.Fatal(err)
	}

	if err := client.Create("node", node2, nil); err != nil {
		t.Fatal(err)
	}

	edge := new(types.Edge)

	if err := client.Create("edge", edge, nil); err == nil {
		t.Errorf("Expected error when creating a edge without ID")
	}

	edge.ID = graph.GenID()

	if err := client.Create("edge", edge, nil); err == nil {
		t.Errorf("Expected error when creating a edge without relation type")
	}

	edge.Metadata = graph.Metadata{"RelationType": "mylink"}

	if err := client.Create("edge", edge, nil); err == nil {
		t.Errorf("Expected error when creating a edge without parent and child")
	}

	edge.Parent = node1.ID
	edge.Child = node2.ID

	if err := client.Create("edge", edge, nil); err != nil {
		t.Error(err)
	}

	if err := client.Get("edge", string(edge.ID), &edge); err != nil {
		t.Error(err)
	}

	if relationType, _ := edge.GetFieldString("RelationType"); relationType != "mylink" {
		t.Errorf("Expected edge with relation type 'mylink'")
	}
}

// PATCH
type JSONPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

// Create node, patch node, check modification has been made and updatedAt/revision updated
func TestAPIPatchNode(t *testing.T) {
	tests := []struct {
		name         string
		originalNode []byte
		patch        []JSONPatch
		expectedNode []byte
	}{
		{
			name: "PATCH node, add new attribute",
			originalNode: []byte(`{
					"ID": "test1",
					"Metadata": {
						"TID": "test1",
						"Name": "name1",
						"Type": "type1"
					},
					"Host": "host1",
					"Origin": "origin1",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:    "add",
					Path:  "/Metadata/foo",
					Value: "bar",
				},
			},
			expectedNode: []byte(`{
					"ID": "test1",
					"Metadata": {
						"TID":  "test1",
						"Name": "name1",
						"Type": "type1",
						"foo": "bar"
					},
					"Host": "host1",
					"Origin": "origin1",
					"CreatedAt": 0,
					"UpdatedAt": 1,
					"Revision":  0
				}`),
		},
		{
			name: "PATCH node, modify attribute",
			originalNode: []byte(`{
					"ID": "test2",
					"Metadata": {
						"TID": "test2",
						"Name": "name2",
						"Type": "type2",
						"foo": "bar"
					},
					"Host": "host2",
					"Origin": "origin2",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/foo",
					Value: "foo",
				},
			},
			expectedNode: []byte(`{
					"ID": "test2",
					"Metadata": {
						"TID":  "test2",
						"Name": "name2",
						"Type": "type2",
						"foo": "foo"
					},
					"Host": "host2",
					"Origin": "origin2",
					"CreatedAt": 0,
					"UpdatedAt": 1,
					"Revision":  0
				}`),
		},
		{
			name: "PATCH node, delete attribute",
			originalNode: []byte(`{
					"ID": "test3",
					"Metadata": {
						"TID": "test3",
						"Name": "name3",
						"Type": "type3",
						"foo": "bar"
					},
					"Host": "host3",
					"Origin": "origin3",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:   "remove",
					Path: "/Metadata/foo",
				},
			},
			expectedNode: []byte(`{
					"ID": "test3",
					"Metadata": {
						"TID":  "test3",
						"Name": "name3",
						"Type": "type3"
					},
					"Host": "host3",
					"Origin": "origin3",
					"CreatedAt": 0,
					"UpdatedAt": 1,
					"Revision":  0
				}`),
		},
		{
			name: "PATCH node, modify attribute Metadata.TID is ignored",
			originalNode: []byte(`{
					"ID": "test4",
					"Metadata": {
						"TID": "test4",
						"Name": "name4",
						"Type": "type4"
					},
					"Host": "host4",
					"Origin": "origin4",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/TID",
					Value: "123",
				},
			},
			expectedNode: []byte(`{
					"ID": "test4",
					"Metadata": {
						"TID":  "test4",
						"Name": "name4",
						"Type": "type4"
					},
					"Host": "host4",
					"Origin": "origin4",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
		},
		{
			name: "PATCH node, modify attribute Metadata.Name is ignored",
			originalNode: []byte(`{
					"ID": "test5",
					"Metadata": {
						"TID": "test5",
						"Name": "name5",
						"Type": "type5"
					},
					"Host": "host5",
					"Origin": "origin5",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/Name",
					Value: "noop",
				},
			},
			expectedNode: []byte(`{
					"ID": "test5",
					"Metadata": {
						"TID":  "test5",
						"Name": "name5",
						"Type": "type5"
					},
					"Host": "host5",
					"Origin": "origin5",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
		},
		{
			name: "PATCH node, modify attribute Metadata.Type is ignored",
			originalNode: []byte(`{
					"ID": "test6",
					"Metadata": {
						"TID": "test6",
						"Name": "name6",
						"Type": "type6"
					},
					"Host": "host6",
					"Origin": "origin6",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Metadata/Type",
					Value: "noop",
				},
			},
			expectedNode: []byte(`{
					"ID": "test6",
					"Metadata": {
						"TID":  "test6",
						"Name": "name6",
						"Type": "type6"
					},
					"Host": "host6",
					"Origin": "origin6",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
		},
		{
			name: "PATCH node, delete attributes Metadata.TID is ignored",
			originalNode: []byte(`{
					"ID": "test7",
					"Metadata": {
						"TID": "test7",
						"Name": "name7",
						"Type": "type7"
					},
					"Host": "host7",
					"Origin": "origin7",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:   "remove",
					Path: "/Metadata/TID",
				},
			},
			expectedNode: []byte(`{
					"ID": "test7",
					"Metadata": {
						"TID":  "test7",
						"Name": "name7",
						"Type": "type7"
					},
					"Host": "host7",
					"Origin": "origin7",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
		},
		{
			name: "PATCH node, modifying values outside Metadata are ignored",
			originalNode: []byte(`{
					"ID": "test8",
					"Metadata": {
						"TID": "test8",
						"Name": "name8",
						"Type": "type8"
					},
					"Host": "host8",
					"Origin": "origin8",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
			patch: []JSONPatch{
				{
					Op:    "replace",
					Path:  "/Host",
					Value: "foo",
				},
			},
			expectedNode: []byte(`{
					"ID": "test8",
					"Metadata": {
						"TID":  "test8",
						"Name": "name8",
						"Type": "type8"
					},
					"Host": "host8",
					"Origin": "origin8",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := getCrudClient()
			if err != nil {
				t.Fatal(err)
			}

			// Add originalNode to the graph
			originalNode := types.Node{}
			expectedNode := types.Node{}
			patchedNode := types.Node{}
			getPatchedNode := types.Node{}

			if err := json.Unmarshal(test.originalNode, &originalNode); err != nil {
				t.Fatalf("error unmarshal originalNode: %v", err)
			}

			if err := json.Unmarshal(test.expectedNode, &expectedNode); err != nil {
				t.Fatalf("error unmarshal expectedNode: %v", err)
			}

			// Create original node
			if err := client.Create("node", &originalNode, nil); err != nil {
				t.Fatalf("Failed to create originalNode: %s", err.Error())
			}

			// Patch node
			err = client.Update("node", originalNode.GetID(), test.patch, &patchedNode)
			if err != nil {
				t.Fatalf("Failed to apply patch: %s", err.Error())
			}

			err = client.Get("node", originalNode.GetID(), &getPatchedNode)
			if err != nil {
				t.Fatalf("Failed to get node after patching: %s", err.Error())
			}

			// Compare nodes
			if !reflect.DeepEqual(expectedNode.Metadata, patchedNode.Metadata) {
				t.Errorf("JSON Patch was not applied.\nMetadata original: %+v\nMetadata patched:  %+v", originalNode.Metadata, patchedNode.Metadata)
			}

			if !reflect.DeepEqual(patchedNode.Metadata, getPatchedNode.Metadata) {
				t.Errorf("Response from PATCH method and node stored are not the same.\nMetadata PATCH response: %+v\nMetadata node stored:  %+v", patchedNode.Metadata, getPatchedNode.Metadata)
			}

			if expectedNode.Revision != patchedNode.Revision {
				t.Errorf("Revision version is not being updated by PATCH")
			}

			// If expectedNode.UpdatedAt is "1", we expect a change in the UpdatedAt field
			if expectedNode.UpdatedAt.Unix() == 1 && patchedNode.UpdatedAt.IsZero() {
				t.Error("UpdatedAt is not being updated by PATCH")
			}
		})
	}
}

// Try to patch a non existing node
func TestAPIPatchNodeNonExistingNode(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	// Add originalNode to the graph
	patchedNode := types.Node{}

	patch := []JSONPatch{
		{
			Op:    "add",
			Path:  "/Metadata/Foo",
			Value: "bar",
		},
	}

	// Patch node
	err = client.Update("node", "foo", patch, &patchedNode)
	if err == nil {
		t.Fatal("Error should be returned if node id does not exists")
	}
}

// Try to patch deleting Metadata.Name generates a validation error
func TestAPIPatchNodeNameDeleteValidationError(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	// Add originalNode to the graph
	originalNode := types.Node{}
	patchedNode := types.Node{}

	if err := json.Unmarshal([]byte(`{
					"ID": "test1",
					"Metadata": {
						"TID": "test1",
						"Name": "name1",
						"Type": "type1"
					},
					"Host": "host1",
					"Origin": "origin1",
					"CreatedAt": 0,
					"UpdatedAt": 0,
					"Revision":  0
				}`), &originalNode); err != nil {
		t.Fatalf("error unmarshal originalNode: %v", err)
	}

	patch := []JSONPatch{
		{
			Op:   "remove",
			Path: "/Metadata/Name",
		},
	}

	// Patch node
	err = client.Update("node", "foo", patch, &patchedNode)
	if err == nil {
		t.Fatal("Error should be returned because trying to remove Metadata.Name violates validation")
	}
}

// Erroneous patch
func TestAPIPatchNodeErroneousPatch(t *testing.T) {
	client, err := getCrudClient()
	if err != nil {
		t.Fatal(err)
	}

	// Add originalNode to the graph
	patchedNode := types.Node{}

	patch := []JSONPatch{
		{
			Op:    "foobar",
			Path:  "/Metadata/Foo",
			Value: "bar",
		},
	}

	// Patch node
	err = client.Update("node", "foo", patch, &patchedNode)
	if err == nil {
		t.Fatal("Error should be returned if patch is incorrect")
	}
}
