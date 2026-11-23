/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"github.com/olivere/elastic/v7"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/storage"
	es "github.com/skydive-project/skydive/graffiti/storage/elasticsearch"
)

type fakeESIndex struct {
	entries map[string]interface{}
}

type fakeESClient struct {
	indices      map[string]*fakeESIndex
	searches     []elastic.Query
	searchResult elastic.SearchResult
	searchErr    error
}

func (f *fakeESClient) Scroll(hits chan<- *elastic.SearchHit, query elastic.Query, pagination filters.SearchQuery, indices ...string) error {
	return nil
}

func (f *fakeESClient) resetIndices() {
	f.indices = make(map[string]*fakeESIndex)
}

func (f *fakeESClient) Index(index es.Index, id string, data interface{}) error {
	if _, ok := f.indices[index.Name]; !ok {
		f.indices[index.Name] = &fakeESIndex{
			entries: make(map[string]interface{}, 0),
		}
	}

	// reconvert data into map as it is in json.RawMessage
	rd := make(map[string]interface{})
	if err := json.Unmarshal([]byte(data.(json.RawMessage)), &rd); err != nil {
		return err
	}

	// fake id if not given with the revision number
	if id == "" {
		id = fmt.Sprintf("%s-%d", rd["ID"].(string), int(rd["Revision"].(float64)))
	}

	f.indices[index.Name].entries[id] = rd
	return nil
}
func (f *fakeESClient) BulkIndex(index es.Index, id string, data interface{}) error {
	return f.Index(index, id, data)
}
func (f *fakeESClient) Get(index es.Index, id string) (*elastic.GetResult, error) {
	return &elastic.GetResult{}, nil
}
func (f *fakeESClient) Delete(index es.Index, id string) (*elastic.DeleteResponse, error) {
	delete(f.indices[index.Name].entries, id)
	return &elastic.DeleteResponse{}, nil
}
func (f *fakeESClient) BulkDelete(index es.Index, id string) error {
	_, ok := f.Delete(index, id)
	return ok
}
func (f *fakeESClient) Search(query elastic.Query, fsq filters.SearchQuery, indices ...string) (*elastic.SearchResult, error) {
	f.searches = append(f.searches, query)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return &f.searchResult, nil
}
func (f *fakeESClient) SearchCollapsed(query elastic.Query, fsq filters.SearchQuery, collapseField string, indices ...string) (*elastic.SearchResult, error) {
	f.searches = append(f.searches, query)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return &f.searchResult, nil
}
func (f *fakeESClient) Start() {
}
func (f *fakeESClient) AddEventListener(l storage.EventListener) {
}
func (f *fakeESClient) UpdateByScript(query elastic.Query, script *elastic.Script, indices ...string) error {
	return nil
}

func newElasticsearchGraph(t *testing.T) (*Graph, *fakeESClient) {
	client := &fakeESClient{
		indices: make(map[string]*fakeESIndex),
	}
	b := newElasticSearchBackendFromClient(client, "skydive_", es.Index{Name: "topology_live"}, es.Index{Name: "topology_archive"}, nil)
	client.searchResult.Hits = &elastic.SearchHits{}
	return NewGraph("host1", b, "graph-test.host1"), client
}

func TestElasticsearchNode(t *testing.T) {
	g, client := newElasticsearchGraph(t)

	node := g.CreateNode("aaa", Metadata{"MTU": 1500}, Unix(1, 0), "host1")
	g.AddNode(node)
	g.addMetadata(node, "MTU", 1510, Unix(2, 0))

	origin := "graph-test.host1"

	expectedLive := map[string]interface{}{
		"aaa": map[string]interface{}{
			"_Type":     "node",
			"CreatedAt": float64(1000),
			"Host":      "host1",
			"Origin":    origin,
			"ID":        "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1510),
			},
			"Revision":  float64(2),
			"UpdatedAt": float64(2000),
		},
	}

	live := client.indices["topology_live"].entries
	if diff := deep.Equal(live, expectedLive); diff != nil {
		t.Fatalf("Expected elasticsearch live records not found: %s", diff)
	}

	expectedArchive := map[string]interface{}{
		"aaa-1": map[string]interface{}{
			"_Type":      "node",
			"ArchivedAt": float64(2000),
			"CreatedAt":  float64(1000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1500),
			},
			"Revision":  float64(1),
			"UpdatedAt": float64(1000),
		},
	}
	archive := client.indices["topology_archive"].entries
	if diff := deep.Equal(archive, expectedArchive); diff != nil {
		t.Fatalf("Expected elasticsearch archived records not found: %s", diff)
	}

	// generate another revision
	g.addMetadata(node, "MTU", 1520, Unix(3, 0))

	expectedLive = map[string]interface{}{
		"aaa": map[string]interface{}{
			"_Type":     "node",
			"CreatedAt": float64(1000),
			"Host":      "host1",
			"Origin":    origin,
			"ID":        "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1520),
			},
			"Revision":  float64(3),
			"UpdatedAt": float64(3000),
		},
	}
	live = client.indices["topology_live"].entries
	if diff := deep.Equal(live, expectedLive); diff != nil {
		t.Fatalf("Expected elasticsearch live records not found: %s", diff)
	}

	expectedArchive = map[string]interface{}{
		"aaa-1": map[string]interface{}{
			"_Type":      "node",
			"ArchivedAt": float64(2000),
			"CreatedAt":  float64(1000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1500),
			},
			"Revision":  float64(1),
			"UpdatedAt": float64(1000),
		},
		"aaa-2": map[string]interface{}{
			"_Type":      "node",
			"ArchivedAt": float64(3000),
			"CreatedAt":  float64(1000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1510),
			},
			"Revision":  float64(2),
			"UpdatedAt": float64(2000),
		},
	}
	archive = client.indices["topology_archive"].entries
	if diff := deep.Equal(archive, expectedArchive); diff != nil {
		t.Fatalf("Expected elasticsearch archived records not found: %s", diff)
	}

	// delete node, should be deleted from live and generate another
	// archive
	g.delNode(node, Unix(4, 0))

	if _, ok := client.indices["topology_live"].entries["aaa"]; ok {
		t.Fatal("The entry should not be in the index after deletion")
	}

	expectedArchive = map[string]interface{}{
		"aaa-1": map[string]interface{}{
			"_Type":      "node",
			"ArchivedAt": float64(2000),
			"CreatedAt":  float64(1000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1500),
			},
			"Revision":  float64(1),
			"UpdatedAt": float64(1000),
		},
		"aaa-2": map[string]interface{}{
			"_Type":      "node",
			"ArchivedAt": float64(3000),
			"CreatedAt":  float64(1000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1510),
			},
			"Revision":  float64(2),
			"UpdatedAt": float64(2000),
		},
		"aaa-3": map[string]interface{}{
			"_Type":      "node",
			"ArchivedAt": float64(4000),
			"DeletedAt":  float64(4000),
			"CreatedAt":  float64(1000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1520),
			},
			"Revision":  float64(3),
			"UpdatedAt": float64(3000),
		},
	}
	archive = client.indices["topology_archive"].entries
	if diff := deep.Equal(archive, expectedArchive); diff != nil {
		t.Fatalf("Expected elasticsearch archived records not found: %s", diff)
	}
}

func TestElasticsearchEdge(t *testing.T) {
	g, client := newElasticsearchGraph(t)

	node1 := g.CreateNode("aaa", Metadata{"MTU": 1500}, Unix(1, 0), "host1")
	node2 := g.CreateNode("bbb", Metadata{"MTU": 1500}, Unix(1, 0), "host1")
	g.AddNode(node1)
	g.AddNode(node2)

	edge := g.CreateEdge("eee", node1, node2, Metadata{"Name": "eee"}, Unix(1, 0), "host1")
	g.AddEdge(edge)
	g.addMetadata(edge, "Type", "veth", Unix(2, 0))

	origin := "graph-test.host1"

	expectedLive := map[string]interface{}{
		"aaa": map[string]interface{}{
			"_Type":     "node",
			"CreatedAt": float64(1000),
			"Host":      "host1",
			"Origin":    origin,
			"ID":        "aaa",
			"Metadata": map[string]interface{}{
				"MTU": float64(1500),
			},
			"Revision":  float64(1),
			"UpdatedAt": float64(1000),
		},
		"bbb": map[string]interface{}{
			"_Type":     "node",
			"CreatedAt": float64(1000),
			"Host":      "host1",
			"Origin":    origin,
			"ID":        "bbb",
			"Metadata": map[string]interface{}{
				"MTU": float64(1500),
			},
			"Revision":  float64(1),
			"UpdatedAt": float64(1000),
		},
		"eee": map[string]interface{}{
			"_Type":     "edge",
			"CreatedAt": float64(1000),
			"Host":      "host1",
			"Origin":    origin,
			"ID":        "eee",
			"Metadata": map[string]interface{}{
				"Type": "veth",
				"Name": "eee",
			},
			"Parent":    "aaa",
			"Child":     "bbb",
			"Revision":  float64(2),
			"UpdatedAt": float64(2000),
		},
	}

	live := client.indices["topology_live"].entries
	if diff := deep.Equal(live, expectedLive); diff != nil {
		t.Fatalf("Expected elasticsearch live records not found: %s", diff)
	}

	expectedArchive := map[string]interface{}{
		"eee-1": map[string]interface{}{
			"_Type":      "edge",
			"CreatedAt":  float64(1000),
			"ArchivedAt": float64(2000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "eee",
			"Metadata": map[string]interface{}{
				"Name": "eee",
			},
			"Parent":    "aaa",
			"Child":     "bbb",
			"Revision":  float64(1),
			"UpdatedAt": float64(1000),
		},
	}
	archive := client.indices["topology_archive"].entries
	if diff := deep.Equal(archive, expectedArchive); diff != nil {
		t.Fatalf("Expected elasticsearch archived records not found: %s", diff)
	}

	g.delEdge(edge, Unix(3, 0))

	if _, ok := client.indices["topology_live"].entries["eee"]; ok {
		t.Fatal("The entry should not be in the index after deletion")
	}

	expectedArchive = map[string]interface{}{
		"eee-2": map[string]interface{}{
			"_Type":      "edge",
			"CreatedAt":  float64(1000),
			"DeletedAt":  float64(3000),
			"ArchivedAt": float64(3000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "eee",
			"Metadata": map[string]interface{}{
				"Name": "eee",
				"Type": "veth",
			},
			"Parent":    "aaa",
			"Child":     "bbb",
			"Revision":  float64(2),
			"UpdatedAt": float64(2000),
		},
		"eee-1": map[string]interface{}{
			"_Type":      "edge",
			"CreatedAt":  float64(1000),
			"ArchivedAt": float64(2000),
			"Host":       "host1",
			"Origin":     origin,
			"ID":         "eee",
			"Metadata": map[string]interface{}{
				"Name": "eee",
			},
			"Parent":    "aaa",
			"Child":     "bbb",
			"Revision":  float64(1),
			"UpdatedAt": float64(1000),
		},
	}
	archive = client.indices["topology_archive"].entries
	if diff := deep.Equal(archive, expectedArchive); diff != nil {
		t.Fatalf("Expected elasticsearch archived records not found: %s", diff)
	}
}

// TestElasticsearchQueryErrors checks failed searches are added to the
// context errors, so the query fails instead of returning an empty result
func TestElasticsearchQueryErrors(t *testing.T) {
	g, client := newElasticsearchGraph(t)
	b := g.backend
	client.searchErr = errors.New("connection refused")

	errs := &QueryErrors{}
	ctx := Context{TimeSlice: NewTimeSlice(0, 1000), Errors: errs}

	if nodes := b.GetNodes(ctx, nil); len(nodes) != 0 {
		t.Fatalf("expected no nodes, got %d", len(nodes))
	}
	b.GetNodeEdges(CreateNode("aaa", Metadata{}, Unix(1, 0), "host1", "graph-test.host1"), ctx, nil)
	if _, ok := b.(*ElasticSearchBackend).GetNodesOneRevision([]Identifier{"aaa"}, ctx); !ok {
		t.Fatal("GetNodesOneRevision must be supported")
	}

	err := errs.Err()
	if !errors.Is(err, ErrBackendQuery) {
		t.Fatalf("expected ErrBackendQuery, got %v", err)
	}
	if !strings.Contains(err.Error(), "3 request(s) failed") || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// without collector (live context, Sync...) errors are only logged
	b.GetNodes(Context{TimeSlice: NewTimeSlice(0, 1000)}, nil)

	// no error when the searches succeed
	client.searchErr = nil
	errs = &QueryErrors{}
	b.GetNodes(Context{TimeSlice: NewTimeSlice(0, 1000), Errors: errs}, nil)
	if err := errs.Err(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
