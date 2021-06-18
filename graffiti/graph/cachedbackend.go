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

package graph

import (
	"context"
	"errors"
	"sync/atomic"

	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/service"
	"go.opentelemetry.io/otel/attribute"
)

// Define the running cache mode, memory and/or persistent
const (
	CacheOnlyMode int = iota
	DefaultMode
)

// Cachebackend graph errors
var (
	ErrCacheBackendModeUnknown = errors.New("Cache backend mode unknown")
)

// CachedBackend describes a cache mechanism in memory and/or persistent database
type CachedBackend struct {
	host           string
	serviceType    service.Type
	memory         *MemoryBackend
	persistent     PersistentBackend
	cacheMode      atomic.Value
	listeners      []PersistentBackendListener
	masterElection etcd.MasterElection
}

// SetMode set cache mode
func (c *CachedBackend) SetMode(mode int) {
	c.cacheMode.Store(mode)
}

// NodeAdded same the node in the cache
func (c *CachedBackend) NodeAdded(ctx context.Context, n *Node) error {
	mode := c.cacheMode.Load()

	if err := c.memory.NodeAdded(ctx, n); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.NodeAdded(ctx, n); err != nil {
			return err
		}
	}

	return nil
}

// NodeDeleted Delete the node in the cache
func (c *CachedBackend) NodeDeleted(ctx context.Context, n *Node) error {
	mode := c.cacheMode.Load()

	if err := c.memory.NodeDeleted(ctx, n); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.NodeDeleted(ctx, n); err != nil {
			return err
		}
	}

	return nil
}

// GetNode retrieve a node from the cache within a time slice
func (c *CachedBackend) GetNode(ctx context.Context, i Identifier, t Context) []*Node {
	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetNode(ctx, i, t)
	}

	return c.persistent.GetNode(ctx, i, t)
}

// GetNodeEdges retrieve a list of edges from a node within a time slice, matching metadata
func (c *CachedBackend) GetNodeEdges(ctx context.Context, n *Node, t Context, m ElementMatcher) (edges []*Edge) {
	ctx, span := tracer.Start(ctx, "CachedBackend.GetNodeEdges")
	span.SetAttributes(attribute.Key("node.id").String(string(n.ID)))
	addFilterAttribute(span, m, "metadata.filter")
	defer span.End()

	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetNodeEdges(ctx, n, t, m)
	}

	return c.persistent.GetNodeEdges(ctx, n, t, m)
}

// EdgeAdded add an edge in the cache
func (c *CachedBackend) EdgeAdded(ctx context.Context, e *Edge) error {
	ctx, span := tracer.Start(ctx, "CachedBackend.EdgeAdded")
	defer span.End()

	mode := c.cacheMode.Load()

	if err := c.memory.EdgeAdded(ctx, e); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.EdgeAdded(ctx, e); err != nil {
			return err
		}
	}

	return nil
}

// EdgeDeleted delete an edge in the cache
func (c *CachedBackend) EdgeDeleted(ctx context.Context, e *Edge) error {
	ctx, span := tracer.Start(ctx, "CachedBackend.EdgeDeleted")
	defer span.End()

	mode := c.cacheMode.Load()

	if err := c.memory.EdgeDeleted(ctx, e); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.EdgeDeleted(ctx, e); err != nil {
			return err
		}
	}

	return nil
}

// GetEdge retrieve an edge within a time slice
func (c *CachedBackend) GetEdge(ctx context.Context, i Identifier, t Context) []*Edge {
	ctx, span := tracer.Start(ctx, "CachedBackend.GetEdge")
	defer span.End()

	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetEdge(ctx, i, t)
	}

	return c.persistent.GetEdge(ctx, i, t)
}

// GetEdgeNodes retrieve a list of nodes from an edge within a time slice, matching metadata
func (c *CachedBackend) GetEdgeNodes(ctx context.Context, e *Edge, t Context, parentMetadata, childMetadata ElementMatcher) ([]*Node, []*Node) {
	ctx, span := tracer.Start(ctx, "CachedBackend.GetEdgeNodes")
	span.SetAttributes(attribute.Key("edge.id").String(string(e.ID)))
	addFilterAttribute(span, parentMetadata, "parent.metadata.filter")
	addFilterAttribute(span, childMetadata, "child.metadata.filter")
	defer span.End()

	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetEdgeNodes(ctx, e, t, parentMetadata, childMetadata)
	}

	return c.persistent.GetEdgeNodes(ctx, e, t, parentMetadata, childMetadata)
}

// MetadataUpdated updates metadata
func (c *CachedBackend) MetadataUpdated(ctx context.Context, i interface{}) error {
	ctx, span := tracer.Start(ctx, "CachedBackend.MetadataUpdated")
	defer span.End()

	mode := c.cacheMode.Load()

	if err := c.memory.MetadataUpdated(ctx, i); err != nil {
		return err
	}

	if mode != CacheOnlyMode && c.persistent != nil {
		if err := c.persistent.MetadataUpdated(ctx, i); err != nil {
			return err
		}
	}
	return nil
}

// GetNodes returns a list of nodes with a time slice, matching metadata
func (c *CachedBackend) GetNodes(ctx context.Context, t Context, m ElementMatcher, e ElementMatcher) []*Node {
	ctx, span := tracer.Start(ctx, "CachedBackend.GetNodes")
	addFilterAttribute(span, m, "metadata.filter")
	addFilterAttribute(span, e, "metadata.filter.element")
	defer span.End()

	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetNodes(ctx, t, m, e)
	}

	return c.persistent.GetNodes(ctx, t, m, e)
}

// GetEdges returns a list of edges with a time slice, matching metadata
func (c *CachedBackend) GetEdges(ctx context.Context, t Context, m ElementMatcher, e ElementMatcher) []*Edge {
	ctx, span := tracer.Start(ctx, "CachedBackend.GetEdges")
	addFilterAttribute(span, m, "metadata.filter")
	addFilterAttribute(span, e, "metadata.filter.element")
	defer span.End()

	mode := c.cacheMode.Load()

	if t.TimeSlice == nil || mode == CacheOnlyMode || c.persistent == nil {
		return c.memory.GetEdges(ctx, t, m, e)
	}
	return c.persistent.GetEdges(ctx, t, m, e)
}

// IsHistorySupported returns whether the persistent backend supports history
func (c *CachedBackend) IsHistorySupported() bool {
	return c.persistent != nil && c.persistent.IsHistorySupported()
}

// OnStarted implements PersistentBackendListener interface
func (c *CachedBackend) OnStarted() {
	ctx, span := tracer.Start(context.Background(), "CachedBackend.OnStarted")
	defer span.End()

	if c.persistent != nil && c.masterElection.IsMaster() {
		regexFilter, _ := filters.NewRegexFilter("Origin", string(c.serviceType)+"\\..*")
		originFilter := &filters.Filter{RegexFilter: regexFilter}

		if err := c.persistent.FlushElements(ctx, NewElementFilter(originFilter)); err != nil {
			logging.GetLogger().Errorf("failed to flush elements: %s", err)
		}

		g := NewGraph(c.host, c.memory, Origin(c.host, c.serviceType))
		elementFilter := NewElementFilter(filters.NewNotFilter(originFilter))
		if err := c.persistent.Sync(ctx, g, elementFilter); err != nil {
			logging.GetLogger().Errorf("failed to synchronize persistent backend with in memory graph: %s", err)
		}
	}

	for _, listener := range c.listeners {
		listener.OnStarted()
	}
}

// Start the Backend
func (c *CachedBackend) Start() error {
	c.masterElection.StartAndWait()

	if c.persistent != nil {
		return c.persistent.Start()
	} else {
		for _, listener := range c.listeners {
			listener.OnStarted()
		}
	}
	return nil
}

// Stop the backend
func (c *CachedBackend) Stop() {
	if c.persistent != nil {
		c.persistent.Stop()
	}
}

// AddListener registers a listener to the cached backend
func (c *CachedBackend) AddListener(listener PersistentBackendListener) {
	c.listeners = append(c.listeners, listener)
}

// NewCachedBackend creates new graph cache mechanism
func NewCachedBackend(persistent PersistentBackend, electionService etcd.MasterElectionService, host string, serviceType service.Type) (*CachedBackend, error) {
	memory, err := NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	sb := &CachedBackend{
		host:           host,
		serviceType:    serviceType,
		persistent:     persistent,
		memory:         memory,
		masterElection: electionService.NewElection("/elections/cachedbackend-persistent"),
	}

	sb.cacheMode.Store(DefaultMode)

	if persistent != nil {
		persistent.AddListener(sb)
	}

	return sb, nil
}
