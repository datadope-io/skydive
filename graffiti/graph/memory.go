/*
 * Copyright (C) 2015 Red Hat, Inc.
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
	"context"
	"fmt"
)

// MemoryBackendNode a memory backend node
type MemoryBackendNode struct {
	*Node
	edges map[Identifier]*MemoryBackendEdge
}

// MemoryBackendEdge a memory backend edge
type MemoryBackendEdge struct {
	*Edge
}

// MemoryBackend describes the memory backend
type MemoryBackend struct {
	Backend
	nodes map[Identifier]*MemoryBackendNode
	edges map[Identifier]*MemoryBackendEdge
}

// MetadataUpdated return true
func (m *MemoryBackend) MetadataUpdated(ctx context.Context, i interface{}) error {
	switch i := i.(type) {
	case *Node:
		if _, ok := m.nodes[i.ID]; !ok {
			return ErrNodeNotFound
		}
	case *Edge:
		if _, ok := m.edges[i.ID]; !ok {
			return ErrEdgeNotFound
		}
	}

	return nil
}

// EdgeAdded event add an edge in the memory backend
func (m *MemoryBackend) EdgeAdded(ctx context.Context, e *Edge) error {
	if _, ok := m.edges[e.ID]; ok {
		return ErrEdgeConflict
	}

	edge := &MemoryBackendEdge{
		Edge: e,
	}

	parent, ok := m.nodes[e.Parent]
	if !ok {
		return ErrParentNotFound
	}

	child, ok := m.nodes[e.Child]
	if !ok {
		return ErrChildNotFound
	}

	m.edges[e.ID] = edge
	parent.edges[e.ID] = edge
	child.edges[e.ID] = edge

	return nil
}

// GetEdge in the graph backend
func (m *MemoryBackend) GetEdge(ctx context.Context, i Identifier, t Context) []*Edge {
	if e, ok := m.edges[i]; ok {
		return []*Edge{e.Edge}
	}
	return nil
}

// GetEdgeNodes returns a list of nodes of an edge
func (m *MemoryBackend) GetEdgeNodes(ctx context.Context, e *Edge, t Context, parentMetadata, childMetadata ElementMatcher) ([]*Node, []*Node) {
	var parent, child *MemoryBackendNode

	p, ok := m.nodes[e.Parent]
	if !ok {
		panic(fmt.Errorf("not able to find parent node for edge: %+v", e))
	}
	if p.MatchMetadata(parentMetadata) {
		parent = p
	}

	c, ok := m.nodes[e.Child]
	if !ok {
		panic(fmt.Errorf("not able to find child node for edge: %+v", e))
	}

	if c.MatchMetadata(childMetadata) {
		child = c
	}

	if parent == nil || child == nil {
		return nil, nil
	}

	return []*Node{parent.Node}, []*Node{child.Node}
}

// NodeAdded in the graph backend
func (m *MemoryBackend) NodeAdded(ctx context.Context, n *Node) error {
	if _, ok := m.nodes[n.ID]; ok {
		return ErrNodeConflict
	}

	m.nodes[n.ID] = &MemoryBackendNode{
		Node:  n,
		edges: make(map[Identifier]*MemoryBackendEdge),
	}

	return nil
}

// GetNode from the graph backend
func (m *MemoryBackend) GetNode(ctx context.Context, i Identifier, t Context) []*Node {
	if n, ok := m.nodes[i]; ok {
		return []*Node{n.Node}
	}
	return nil
}

// GetNodeEdges returns a list of edges of a node
func (m *MemoryBackend) GetNodeEdges(ctx context.Context, n *Node, t Context, meta ElementMatcher) []*Edge {
	edges := []*Edge{}

	if n, ok := m.nodes[n.ID]; ok {
		for _, e := range n.edges {
			if e.MatchMetadata(meta) {
				edges = append(edges, e.Edge)
			}
		}
	}

	return edges
}

// EdgeDeleted in the graph backend
func (m *MemoryBackend) EdgeDeleted(ctx context.Context, e *Edge) error {
	if _, ok := m.edges[e.ID]; !ok {
		return ErrEdgeNotFound
	}

	if parent, ok := m.nodes[e.Parent]; ok {
		delete(parent.edges, e.ID)
	}

	if child, ok := m.nodes[e.Child]; ok {
		delete(child.edges, e.ID)
	}

	delete(m.edges, e.ID)

	return nil
}

// NodeDeleted in the graph backend
func (m *MemoryBackend) NodeDeleted(ctx context.Context, n *Node) error {
	if _, ok := m.nodes[n.ID]; !ok {
		return ErrNodeNotFound
	}

	delete(m.nodes, n.ID)

	return nil
}

// GetNodes from the graph backend
func (m MemoryBackend) GetNodes(ctx context.Context, t Context, metadata ElementMatcher, element ElementMatcher) (nodes []*Node) {
	for _, n := range m.nodes {
		if n.MatchMetadata(metadata) && n.MatchMetadata(element) {
			nodes = append(nodes, n.Node)
		}
	}
	return
}

// GetEdges from the graph backend
func (m MemoryBackend) GetEdges(ctx context.Context, t Context, metadata ElementMatcher, element ElementMatcher) (edges []*Edge) {
	for _, e := range m.edges {
		if e.MatchMetadata(metadata) && e.MatchMetadata(element) {
			edges = append(edges, e.Edge)
		}
	}
	return
}

// IsHistorySupported returns that this backend doesn't support history
func (m *MemoryBackend) IsHistorySupported() bool {
	return false
}

// NewMemoryBackend creates a new graph memory backend
func NewMemoryBackend() (*MemoryBackend, error) {
	return &MemoryBackend{
		nodes: make(map[Identifier]*MemoryBackendNode),
		edges: make(map[Identifier]*MemoryBackendEdge),
	}, nil
}
