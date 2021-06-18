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
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/safchain/insanelock"
	"go.opentelemetry.io/otel/attribute"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/getter"
)

const (
	maxEvents = 50
)

type graphEventType int

// Graph events
const (
	NodeUpdated graphEventType = iota + 1
	NodeAdded
	NodeDeleted
	EdgeUpdated
	EdgeAdded
	EdgeDeleted
)

// Identifier graph ID
type Identifier string

// EventListener describes the graph events interface mechanism
type EventListener interface {
	OnNodeUpdated(ctx context.Context, n *Node, ops []PartiallyUpdatedOp)
	OnNodeAdded(ctx context.Context, n *Node)
	OnNodeDeleted(ctx context.Context, n *Node)
	OnEdgeUpdated(ctx context.Context, e *Edge, ops []PartiallyUpdatedOp)
	OnEdgeAdded(ctx context.Context, e *Edge)
	OnEdgeDeleted(ctx context.Context, e *Edge)
}

type graphEvent struct {
	kind     graphEventType
	element  interface{}
	listener EventListener
	ops      []PartiallyUpdatedOp
}

type graphElement struct {
	ID        Identifier
	Metadata  Metadata
	Host      string
	Origin    string
	CreatedAt Time
	UpdatedAt Time
	DeletedAt Time `json:"DeletedAt,omitempty"`
	Revision  int64
}

// Node of the graph
type Node struct {
	graphElement `mapstructure:",squash"`
}

// Edge of the graph linked by a parent and a child
type Edge struct {
	graphElement `mapstructure:",squash"`
	Parent       Identifier
	Child        Identifier
}

// Graph errors
var (
	ErrElementNotFound = errors.New("Graph element not found")
	ErrNodeNotFound    = errors.New("Node not found")
	ErrEdgeNotFound    = errors.New("Edge not found")
	ErrParentNotFound  = errors.New("Parent node not found")
	ErrChildNotFound   = errors.New("Child node not found")
	ErrEdgeConflict    = errors.New("Edge ID conflict")
	ErrNodeConflict    = errors.New("Node ID conflict")
	ErrInternal        = errors.New("Internal backend error")
)

// Backend interface mechanism used as storage
type Backend interface {
	NodeAdded(ctx context.Context, n *Node) error
	NodeDeleted(ctx context.Context, n *Node) error
	GetNode(ctx context.Context, i Identifier, at Context) []*Node
	GetNodeEdges(ctx context.Context, n *Node, at Context, m ElementMatcher) []*Edge

	EdgeAdded(ctx context.Context, e *Edge) error
	EdgeDeleted(ctx context.Context, e *Edge) error
	GetEdge(ctx context.Context, i Identifier, at Context) []*Edge
	GetEdgeNodes(ctx context.Context, e *Edge, at Context, parentMetadata, childMetadata ElementMatcher) ([]*Node, []*Node)

	MetadataUpdated(ctx context.Context, e interface{}) error

	GetNodes(ctx context.Context, t Context, m ElementMatcher, e ElementMatcher) []*Node
	GetEdges(ctx context.Context, t Context, m ElementMatcher, e ElementMatcher) []*Edge

	IsHistorySupported() bool
}

type PersistentBackendListener interface {
	OnStarted()
}

// PersistentBackend describes the interface of a persistent storage backend
type PersistentBackend interface {
	Backend

	AddListener(listener PersistentBackendListener)
	FlushElements(ctx context.Context, m ElementMatcher) error
	Sync(context.Context, *Graph, *ElementFilter) error

	Start() error
	Stop()
}

// Context describes within time slice
type Context struct {
	TimeSlice *TimeSlice
	TimePoint bool
}

var liveContext = Context{TimePoint: true}

// Graph describes the graph object based on events and context mechanism
// An associated backend is used as storage
type Graph struct {
	insanelock.RWMutex
	eventHandler *EventHandler
	backend      Backend
	context      Context
	host         string
	origin       string
}

// Elements struct containing nodes and edges
type Elements struct {
	Nodes []*Node
	Edges []*Edge
}

// MetadataDecoder defines a json rawmessage decoder which has to return a object
// implementing the getter interface
type MetadataDecoder func(raw json.RawMessage) (getter.Getter, error)

var (
	// NodeMetadataDecoders is a map that owns special type metadata decoder
	NodeMetadataDecoders = make(map[string]MetadataDecoder)
	// EdgeMetadataDecoders is a map that owns special type metadata decoder
	EdgeMetadataDecoders = make(map[string]MetadataDecoder)
)

// DefaultGraphListener default implementation of a graph listener, can be used when not implementing
// the whole set of callbacks
type DefaultGraphListener struct {
}

// OnNodeUpdated event
func (c *DefaultGraphListener) OnNodeUpdated(ctx context.Context, n *Node, ops []PartiallyUpdatedOp) {
}

// OnNodeAdded event
func (c *DefaultGraphListener) OnNodeAdded(ctx context.Context, n *Node) {
}

// OnNodeDeleted event
func (c *DefaultGraphListener) OnNodeDeleted(ctx context.Context, n *Node) {
}

// OnEdgeUpdated event
func (c *DefaultGraphListener) OnEdgeUpdated(ctx context.Context, e *Edge, ops []PartiallyUpdatedOp) {
}

// OnEdgeAdded event
func (c *DefaultGraphListener) OnEdgeAdded(ctx context.Context, e *Edge) {
}

// OnEdgeDeleted event
func (c *DefaultGraphListener) OnEdgeDeleted(ctx context.Context, e *Edge) {
}

// ListenerHandler describes an other that manages a set of event listeners
type ListenerHandler interface {
	AddEventListener(l EventListener)
	RemoveEventListener(l EventListener)
}

// EventHandler describes an object that notifies listeners with graph events
type EventHandler struct {
	insanelock.RWMutex
	eventListeners       []EventListener
	eventChan            chan graphEvent
	eventConsumed        bool
	currentEventListener EventListener
}

// PartiallyUpdatedOpType operation type add/del
type PartiallyUpdatedOpType int

const (
	// PartiallyUpdatedAddOpType add metadata
	PartiallyUpdatedAddOpType PartiallyUpdatedOpType = iota + 1
	// PartiallyUpdatedDelOpType del metadata
	PartiallyUpdatedDelOpType
)

// PartiallyUpdatedOp describes a way to update partially node or edge
type PartiallyUpdatedOp struct {
	Type  PartiallyUpdatedOpType
	Key   string
	Value interface{}
}

// NodePartiallyUpdated partial updates of a node
type NodePartiallyUpdated struct {
	Node *Node
	Ops  []PartiallyUpdatedOp
}

// EdgePartiallyUpdated partial updates of a edge
type EdgePartiallyUpdated struct {
	Edge *Edge
	Ops  []PartiallyUpdatedOp
}

func (g *EventHandler) notifyListeners(ctx context.Context, ge graphEvent) {
	// notify only once per listener as if more than once we are in a recursion
	// and we wont to notify a listener which generated a graph element
	g.RLock()
	defer g.RUnlock()
	for _, g.currentEventListener = range g.eventListeners {
		// do not notify the listener which generated the event
		if g.currentEventListener == ge.listener {
			continue
		}

		switch ge.kind {
		case NodeAdded:
			g.currentEventListener.OnNodeAdded(ctx, ge.element.(*Node))
		case NodeUpdated:
			g.currentEventListener.OnNodeUpdated(ctx, ge.element.(*Node), ge.ops)
		case NodeDeleted:
			g.currentEventListener.OnNodeDeleted(ctx, ge.element.(*Node))
		case EdgeAdded:
			g.currentEventListener.OnEdgeAdded(ctx, ge.element.(*Edge))
		case EdgeUpdated:
			g.currentEventListener.OnEdgeUpdated(ctx, ge.element.(*Edge), ge.ops)
		case EdgeDeleted:
			g.currentEventListener.OnEdgeDeleted(ctx, ge.element.(*Edge))
		}
	}
}

// NotifyEvent notifies all the listeners of an event. NotifyEvent
// makes sure that we don't enter a notify endless loop.
func (g *EventHandler) NotifyEvent(ctx context.Context, kind graphEventType, element interface{}, ops ...PartiallyUpdatedOp) {
	// push event to chan so that nested notification will be sent in the
	// right order. Associate the event with the current event listener so
	// we can avoid loop by not triggering event for the current listener.
	ge := graphEvent{kind: kind, element: element, ops: ops}
	ge.listener = g.currentEventListener
	g.eventChan <- ge

	// already a consumer no need to run another consumer
	if g.eventConsumed {
		return
	}
	g.eventConsumed = true

	for len(g.eventChan) > 0 {
		ge = <-g.eventChan
		g.notifyListeners(ctx, ge)
	}
	g.currentEventListener = nil
	g.eventConsumed = false
}

// AddEventListener subscribe a new graph listener
func (g *EventHandler) AddEventListener(l EventListener) {
	g.Lock()
	defer g.Unlock()

	g.eventListeners = append(g.eventListeners, l)
}

// RemoveEventListener unsubscribe a graph listener
func (g *EventHandler) RemoveEventListener(l EventListener) {
	g.Lock()
	defer g.Unlock()

	for i, el := range g.eventListeners {
		if l == el {
			g.eventListeners = append(g.eventListeners[:i], g.eventListeners[i+1:]...)
			break
		}
	}
}

// NewEventHandler instantiate a new event handler
func NewEventHandler(maxEvents int) *EventHandler {
	return &EventHandler{
		eventChan: make(chan graphEvent, maxEvents),
	}
}

// GenID helper generate a node Identifier
func GenID(s ...string) Identifier {
	if len(s) > 0 {
		u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(strings.Join(s, "/")))
		return Identifier(u.String())
	}

	u, _ := uuid.NewV4()
	return Identifier(u.String())
}

func (e *graphElement) GetFieldBool(field string) (_ bool, err error) {
	return e.Metadata.GetFieldBool(field)
}

func (e *graphElement) GetFieldInt64(field string) (_ int64, err error) {
	switch field {
	case "CreatedAt":
		return e.CreatedAt.UnixMilli(), nil
	case "UpdatedAt":
		return e.UpdatedAt.UnixMilli(), nil
	case "DeletedAt":
		return e.DeletedAt.UnixMilli(), nil
	case "Revision":
		return e.Revision, nil
	default:
		return e.Metadata.GetFieldInt64(field)
	}
}

func (e *graphElement) GetFieldString(field string) (_ string, err error) {
	switch field {
	case "ID":
		return string(e.ID), nil
	case "Host":
		return e.Host, nil
	case "Origin":
		return e.Origin, nil
	default:
		return e.Metadata.GetFieldString(field)
	}
}

func (e *graphElement) GetField(name string) (interface{}, error) {
	if i, err := e.GetFieldInt64(name); err == nil {
		return i, nil
	}

	if s, err := e.GetFieldString(name); err == nil {
		return s, nil
	}

	return e.Metadata.GetField(name)
}

func (e *graphElement) GetFields(names []string) (interface{}, error) {
	values := make(map[string]interface{})

	for _, name := range names {
		if v, err := e.GetField(name); err == nil {
			values[name] = v
		}
	}

	if len(values) == 0 {
		return nil, getter.ErrFieldNotFound
	}

	return values, nil
}

var graphElementKeys = map[string]bool{"ID": false, "Host": false, "Origin": false, "CreatedAt": false, "UpdatedAt": false, "DeletedAt": false, "Revision": false}

func (e *graphElement) GetFieldKeys() []string {
	keys := make([]string, len(graphElementKeys))
	i := 0
	for key := range graphElementKeys {
		keys[i] = key
		i++
	}
	return append(keys, e.Metadata.GetFieldKeys()...)
}

func (e *graphElement) MatchBool(field string, predicate getter.BoolPredicate) bool {
	if index := strings.Index(field, "."); index != -1 {
		first := field[index+1:]
		if v, found := e.Metadata[first]; found {
			if getter, found := v.(getter.Getter); found {
				return getter.MatchBool(field[index+1:], predicate)
			}
		}
	}
	return e.Metadata.MatchBool(field, predicate)
}

func (e *graphElement) MatchInt64(field string, predicate getter.Int64Predicate) bool {
	if _, found := graphElementKeys[field]; found {
		if n, err := e.GetFieldInt64(field); err == nil {
			return predicate(n)
		}
	}

	if index := strings.Index(field, "."); index != -1 {
		first := field[index+1:]
		if v, found := e.Metadata[first]; found {
			if getter, found := v.(getter.Getter); found {
				return getter.MatchInt64(field[index+1:], predicate)
			}
		}
	}

	return e.Metadata.MatchInt64(field, predicate)
}

func (e *graphElement) MatchString(field string, predicate getter.StringPredicate) bool {
	if _, found := graphElementKeys[field]; found {
		if s, err := e.GetFieldString(field); err == nil {
			return predicate(s)
		}
	}

	if index := strings.Index(field, "."); index != -1 {
		first := field[index+1:]
		if v, found := e.Metadata[first]; found {
			if getter, found := v.(getter.Getter); found {
				return getter.MatchString(field[index+1:], predicate)
			}
		}
	}

	return e.Metadata.MatchString(field, predicate)
}

func (e *graphElement) GetFieldStringList(key string) ([]string, error) {
	value, err := e.GetField(key)
	if err != nil {
		return nil, err
	}

	switch value := value.(type) {
	case []interface{}:
		var strings []string
		for _, s := range value {
			if s, ok := s.(string); ok {
				strings = append(strings, s)
			} else {
				return nil, errors.New("one array entry could not be converted to string")
			}
		}
		return strings, nil
	case []string:
		return value, nil
	default:
		return nil, getter.ErrFieldNotFound
	}
}

// MatchMetadata returns whether a graph element matches with the provided filter or metadata
func (e *graphElement) MatchMetadata(f ElementMatcher) bool {
	if f == nil {
		return true
	}
	return f.Match(e)
}

func normalizeNumbers(raw interface{}) interface{} {
	switch t := raw.(type) {
	case float64:
		// float equal to its int version then convert it to int64
		i := int64(t)
		if float64(i) == t {
			return i
		}
	case []interface{}:
		for i, obj := range t {
			t[i] = normalizeNumbers(obj)
		}
	case map[string]interface{}:
		for k, v := range t {
			t[k] = normalizeNumbers(v)
		}
		return t
	}
	return raw
}

// normalize the graph element after partial deserialization
func (e *graphElement) normalize(metadata map[string]json.RawMessage, decoders map[string]MetadataDecoder) error {
	// sanity checks
	if e.ID == "" {
		return errors.New("No ID found for graph element")
	}

	e.Metadata = make(Metadata)

	if e.CreatedAt.IsZero() {
		e.CreatedAt = TimeUTC()
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = TimeUTC()
	}

	for k, r := range metadata {
		if len(r) == 0 {
			continue
		}

		if decoder, ok := decoders[k]; ok {
			v, err := decoder(r)
			if err != nil {
				return err
			}
			e.Metadata[k] = v
			continue
		}

		var i interface{}
		if err := json.Unmarshal(r, &i); err != nil {
			return err
		}

		e.Metadata[k] = normalizeNumbers(i)
	}

	return nil
}

func (n *Node) String() string {
	b, err := json.Marshal(n)
	if err != nil {
		return ""
	}
	return string(b)
}

// UnmarshalJSON custom unmarshal function
func (n *Node) UnmarshalJSON(b []byte) error {
	// wrapper to avoid unmarshal infinite loop
	type nodeWrapper Node

	nw := struct {
		nodeWrapper
		Metadata map[string]json.RawMessage
	}{}
	if err := json.Unmarshal(b, &nw); err != nil {
		return err
	}
	*n = Node(nw.nodeWrapper)

	// set time and unmarshal raw metadata
	if err := n.normalize(nw.Metadata, NodeMetadataDecoders); err != nil {
		return err
	}

	return nil
}

// MatchMetadata returns when an edge matches a specified filter or metadata
func (e *Edge) MatchMetadata(f ElementMatcher) bool {
	if f == nil {
		return true
	}
	return f.Match(e)
}

// MatchBool implements Getter interface
func (e *Edge) MatchBool(name string, predicate getter.BoolPredicate) bool {
	if b, err := e.GetFieldBool(name); err == nil {
		return predicate(b)
	}
	return false
}

// MatchInt64 implements Getter interface
func (e *Edge) MatchInt64(name string, predicate getter.Int64Predicate) bool {
	if n, err := e.GetFieldInt64(name); err == nil {
		return predicate(n)
	}
	return false
}

// MatchString implements Getter interface
func (e *Edge) MatchString(name string, predicate getter.StringPredicate) bool {
	if s, err := e.GetFieldString(name); err == nil {
		return predicate(s)
	}
	return false
}

// GetFieldString implements Getter interface
func (e *Edge) GetFieldString(name string) (string, error) {
	switch name {
	case "Parent":
		return string(e.Parent), nil
	case "Child":
		return string(e.Child), nil
	default:
		return e.graphElement.GetFieldString(name)
	}
}

func (e *Edge) String() string {
	b, err := json.Marshal(e)
	if err != nil {
		return ""
	}
	return string(b)
}

// UnmarshalJSON custom unmarshal function
func (e *Edge) UnmarshalJSON(b []byte) error {
	// wrapper to avoid unmarshal infinite loop
	type edgeWrapper Edge

	ew := struct {
		edgeWrapper
		Metadata map[string]json.RawMessage
	}{}
	if err := json.Unmarshal(b, &ew); err != nil {
		return err
	}
	*e = Edge(ew.edgeWrapper)

	// sanity checks
	if e.Parent == "" {
		return errors.New("parent ID wrong format")
	}
	if e.Child == "" {
		return errors.New("child ID wrong format")
	}

	// set time and unmarshal raw metadata
	if err := e.normalize(ew.Metadata, EdgeMetadataDecoders); err != nil {
		return err
	}

	return nil
}

func dedupNodes(nodes []*Node) []*Node {
	latests := make(map[Identifier]*Node)
	for _, node := range nodes {
		if n, found := latests[node.ID]; !found || node.Revision > n.Revision {
			latests[node.ID] = node
		}
	}

	uniq := make([]*Node, len(latests))
	i := 0
	for _, node := range latests {
		uniq[i] = node
		i++
	}
	return uniq
}

func dedupEdges(edges []*Edge) []*Edge {
	latests := make(map[Identifier]*Edge)
	for _, edge := range edges {
		if e, found := latests[edge.ID]; !found || edge.Revision > e.Revision {
			latests[edge.ID] = edge
		}
	}

	uniq := make([]*Edge, len(latests))
	i := 0
	for _, edge := range latests {
		uniq[i] = edge
		i++
	}
	return uniq
}

// NodeUpdated updates a node
func (g *Graph) NodeUpdated(ctx context.Context, n *Node) error {
	ctx, span := tracer.Start(ctx, "Graph.NodeUpdated")
	defer span.End()

	if node := g.GetNode(ctx, n.ID); node != nil {
		if node.Revision < n.Revision {
			node.Metadata = n.Metadata
			node.UpdatedAt = n.UpdatedAt
			node.Revision = n.Revision

			if err := g.backend.MetadataUpdated(ctx, node); err != nil {
				return err
			}

			g.eventHandler.NotifyEvent(ctx, NodeUpdated, node)
		}
		return nil
	}
	return ErrNodeNotFound
}

// NodePartiallyUpdated partially updates a node
func (g *Graph) NodePartiallyUpdated(ctx context.Context, id Identifier, revision int64, updatedAt Time, ops ...PartiallyUpdatedOp) error {
	ctx, span := tracer.Start(ctx, "Graph.NodePartiallyUpdated")
	defer span.End()

	if node := g.GetNode(ctx, id); node != nil {
		if node.Revision < revision {
			node.Revision = revision
			node.UpdatedAt = updatedAt
			node.Metadata.ApplyUpdates(ops...)

			if err := g.backend.MetadataUpdated(ctx, node); err != nil {
				return err
			}

			g.eventHandler.NotifyEvent(ctx, NodeUpdated, node, ops...)
		}
		return nil
	}
	return ErrNodeNotFound
}

// EdgeUpdated updates an edge
func (g *Graph) EdgeUpdated(ctx context.Context, e *Edge, ops ...PartiallyUpdatedOp) error {
	ctx, span := tracer.Start(ctx, "Graph.EdgeUpdated")
	defer span.End()

	if edge := g.GetEdge(ctx, e.ID); edge != nil {
		if edge.Revision < e.Revision {
			edge.Metadata = e.Metadata
			edge.UpdatedAt = e.UpdatedAt

			if err := g.backend.MetadataUpdated(ctx, edge); err != nil {
				return err
			}

			g.eventHandler.NotifyEvent(ctx, EdgeUpdated, edge, ops...)
			return nil
		}
	}
	return ErrEdgeNotFound
}

// EdgePartiallyUpdated partially updates an edge
func (g *Graph) EdgePartiallyUpdated(ctx context.Context, id Identifier, revision int64, updatedAt Time, ops ...PartiallyUpdatedOp) error {
	ctx, span := tracer.Start(ctx, "Graph.EdgePartiallyUpdated")
	defer span.End()

	if edge := g.GetEdge(ctx, id); edge != nil {
		if edge.Revision < revision {
			edge.Revision = revision
			edge.UpdatedAt = updatedAt
			edge.Metadata.ApplyUpdates(ops...)

			if err := g.backend.MetadataUpdated(ctx, edge); err != nil {
				return err
			}

			g.eventHandler.NotifyEvent(ctx, EdgeUpdated, edge, ops...)
		}
		return nil
	}
	return ErrEdgeNotFound
}

// SetMetadata associate metadata to an edge or node
func (g *Graph) SetMetadata(ctx context.Context, i interface{}, m Metadata) error {
	ctx, span := tracer.Start(ctx, "Graph.SetMetadata")
	defer span.End()

	var e *graphElement
	var kind graphEventType

	switch i := i.(type) {
	case *Node:
		e = &i.graphElement
		kind = NodeUpdated
	case *Edge:
		e = &i.graphElement
		kind = EdgeUpdated
	}

	if reflect.DeepEqual(m, e.Metadata) {
		return nil
	}

	e.Metadata = m
	e.UpdatedAt = TimeUTC()
	e.Revision++

	if err := g.backend.MetadataUpdated(ctx, i); err != nil {
		return err
	}

	g.eventHandler.NotifyEvent(ctx, kind, i)
	return nil
}

// DelMetadata delete a metadata to an associated edge or node
func (g *Graph) DelMetadata(ctx context.Context, i interface{}, k string) error {
	ctx, span := tracer.Start(ctx, "Graph.DelMetadata")
	defer span.End()

	var e *graphElement
	var kind graphEventType

	switch i.(type) {
	case *Node:
		e = &i.(*Node).graphElement
		kind = NodeUpdated
	case *Edge:
		e = &i.(*Edge).graphElement
		kind = EdgeUpdated
	}

	if updated := e.Metadata.DelField(k); !updated {
		return nil
	}

	e.UpdatedAt = TimeUTC()
	e.Revision++

	if err := g.backend.MetadataUpdated(ctx, i); err != nil {
		return err
	}

	op := PartiallyUpdatedOp{Type: PartiallyUpdatedDelOpType, Key: k}
	g.eventHandler.NotifyEvent(ctx, kind, i, op)
	return nil
}

func (g *Graph) addMetadata(ctx context.Context, i interface{}, k string, v interface{}, t Time) error {
	ctx, span := tracer.Start(ctx, "Graph.addMetadata")
	defer span.End()

	var e *graphElement
	var kind graphEventType

	switch i.(type) {
	case *Node:
		e = &i.(*Node).graphElement
		kind = NodeUpdated
	case *Edge:
		e = &i.(*Edge).graphElement
		kind = EdgeUpdated
	}

	if o, ok := e.Metadata[k]; ok && reflect.DeepEqual(o, v) {
		return nil
	}

	if !e.Metadata.SetField(k, v) {
		return nil
	}

	e.UpdatedAt = t
	e.Revision++

	if err := g.backend.MetadataUpdated(ctx, i); err != nil {
		return err
	}

	op := PartiallyUpdatedOp{Type: PartiallyUpdatedAddOpType, Key: k, Value: v}
	g.eventHandler.NotifyEvent(ctx, kind, i, op)
	return nil
}

// AddMetadata add a metadata to an associated edge or node
func (g *Graph) AddMetadata(ctx context.Context, i interface{}, k string, v interface{}) error {
	return g.addMetadata(ctx, i, k, v, TimeUTC())
}

// UpdateMetadata retrieves a value and calls a callback that can modify it then notify listeners of the update
func (g *Graph) UpdateMetadata(ctx context.Context, i interface{}, key string, mutator func(obj interface{}) bool) error {
	ctx, span := tracer.Start(ctx, "Graph.UpdateMetadata")
	defer span.End()

	var e *graphElement

	switch i.(type) {
	case *Node:
		e = &i.(*Node).graphElement
	case *Edge:
		e = &i.(*Edge).graphElement
	}

	field, err := e.GetField(key)
	if err != nil {
		return err
	}

	e.UpdatedAt = TimeUTC()
	e.Revision++

	if updated := mutator(field); updated {
		if err := g.backend.MetadataUpdated(ctx, i); err != nil {
			return err
		}
		g.eventHandler.NotifyEvent(ctx, NodeUpdated, i)
	}

	return nil
}

// StartMetadataTransaction start a new transaction
func (g *Graph) StartMetadataTransaction(i interface{}) *MetadataTransaction {
	t := MetadataTransaction{
		graph:        g,
		graphElement: i,
		adds:         make(map[string]interface{}),
	}

	return &t
}

func (g *Graph) getNeighborNodes(ctx context.Context, n *Node, em ElementMatcher) (nodes []*Node) {
	ctx, span := tracer.Start(ctx, "Graph.getNeighborNodes")
	defer span.End()

	for _, e := range g.backend.GetNodeEdges(ctx, n, g.context, em) {
		parents, children := g.backend.GetEdgeNodes(ctx, e, g.context, nil, nil)
		nodes = append(nodes, parents...)
		nodes = append(nodes, children...)
	}
	return nodes
}

func (g *Graph) findNodeMatchMetadata(nodesMap map[Identifier]*Node, m ElementMatcher) *Node {
	for _, n := range nodesMap {
		if n.MatchMetadata(m) {
			return n
		}
	}
	return nil
}

func getNodeMinDistance(nodesMap map[Identifier]*Node, distance map[Identifier]uint) *Node {
	min := ^uint(0)
	var minID Identifier
	for ID, d := range distance {
		_, ok := nodesMap[ID]
		if !ok {
			continue
		}
		if string(minID) == "" {
			minID = ID
		}

		if d < min {
			min = d
			minID = ID
		}
	}
	n, ok := nodesMap[minID]
	if !ok {
		return nil
	}
	return n
}

// GetNodesMap returns a map of nodes within a time slice
func (g *Graph) GetNodesMap(ctx context.Context, t Context) map[Identifier]*Node {
	ctx, span := tracer.Start(ctx, "Graph.GetNodesMap")
	defer span.End()

	nodes := g.backend.GetNodes(ctx, t, nil, nil)
	nodesMap := make(map[Identifier]*Node, len(nodes))
	for _, n := range nodes {
		nodesMap[n.ID] = n
	}
	return nodesMap
}

// LookupShortestPath based on Dijkstra algorithm
func (g *Graph) LookupShortestPath(ctx context.Context, n *Node, m ElementMatcher, em ElementMatcher) []*Node {
	ctx, span := tracer.Start(ctx, "Graph.LookupShortestPath")
	defer span.End()

	nodesMap := g.GetNodesMap(ctx, g.context)
	target := g.findNodeMatchMetadata(nodesMap, m)
	if target == nil {
		return []*Node{}
	}
	distance := make(map[Identifier]uint, len(nodesMap))
	previous := make(map[Identifier]*Node, len(nodesMap))

	for _, v := range nodesMap {
		distance[v.ID] = ^uint(0)
	}
	distance[target.ID] = uint(0)

	for len(nodesMap) > 0 {
		u := getNodeMinDistance(nodesMap, distance)
		if u == nil {
			break
		}
		delete(nodesMap, u.ID)

		for _, v := range g.getNeighborNodes(ctx, u, em) {
			if _, ok := nodesMap[v.ID]; !ok {
				continue
			}
			alt := distance[u.ID] + 1
			if alt < distance[v.ID] {
				distance[v.ID] = alt
				previous[v.ID] = u
			}
		}
	}

	retNodes := []*Node{}
	node := n
	for {
		retNodes = append(retNodes, node)

		prevNode, ok := previous[node.ID]
		if !ok || node.ID == prevNode.ID {
			break
		}
		node = prevNode
	}

	if node.ID != target.ID {
		return []*Node{}
	}

	return retNodes
}

// LookupParents returns the associated parents edge of a node
func (g *Graph) LookupParents(ctx context.Context, n *Node, f ElementMatcher, em ElementMatcher) (nodes []*Node) {
	ctx, span := tracer.Start(ctx, "Graph.LookupParents")
	defer span.End()

	for _, e := range g.backend.GetNodeEdges(ctx, n, g.context, em) {
		if e.Child == n.ID {
			parents, _ := g.backend.GetEdgeNodes(ctx, e, g.context, f, nil)
			for _, parent := range parents {
				nodes = append(nodes, parent)
			}
		}
	}

	return
}

// LookupFirstChild returns the child
func (g *Graph) LookupFirstChild(ctx context.Context, n *Node, f ElementMatcher) *Node {
	ctx, span := tracer.Start(ctx, "Graph.LookupFirstChild")
	defer span.End()

	nodes := g.LookupChildren(ctx, n, f, nil)
	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

// LookupChildren returns a list of children nodes
func (g *Graph) LookupChildren(ctx context.Context, n *Node, f ElementMatcher, em ElementMatcher) (nodes []*Node) {
	ctx, span := tracer.Start(ctx, "Graph.LookupChildren")
	span.SetAttributes(attribute.Key("node.id").String(string(n.ID)))
	addFilterAttribute(span, em, "edge.metadata.filter")
	addFilterAttribute(span, f, "child.metadata.filter")
	defer span.End()

	for _, e := range g.backend.GetNodeEdges(ctx, n, g.context, em) {
		if e.Parent == n.ID {
			_, children := g.backend.GetEdgeNodes(ctx, e, g.context, nil, f)
			for _, child := range children {
				nodes = append(nodes, child)
			}
		}
	}

	return nodes
}

// LookupNeighbor returns a list of neighbor nodes
func (g *Graph) LookupNeighbor(ctx context.Context, n *Node, f ElementMatcher, em ElementMatcher) (nodes []*Node) {
	ctx, span := tracer.Start(ctx, "Graph.LookupNeighbor")
	defer span.End()

	for _, e := range g.backend.GetNodeEdges(ctx, n, g.context, em) {
		if e.Parent == n.ID {
			_, children := g.backend.GetEdgeNodes(ctx, e, g.context, nil, f)
			for _, child := range children {
				nodes = append(nodes, child)
			}
		} else {
			parents, _ := g.backend.GetEdgeNodes(ctx, e, g.context, f, nil)
			for _, parent := range parents {
				nodes = append(nodes, parent)
			}
		}
	}

	return nodes
}

// LookupNeighborIgnoreVisited returns a list of neighbor nodes ignoring the ones in the visited slice
func (g *Graph) LookupNeighborIgnoreVisited(ctx context.Context, n *Node, f ElementMatcher, em ElementMatcher, visited map[Identifier]bool) (nodes []*Node) {
	ctx, span := tracer.Start(ctx, "Graph.LookupNeighborIgnoreVisited")
	span.SetAttributes(attribute.Key("node.id").String(string(n.ID)))
	addFilterAttribute(span, em, "edge.metadata.filter")
	addFilterAttribute(span, f, "child.metadata.filter")
	addTimeFilterAttribute(span, g.context)
	defer span.End()

	for _, e := range g.backend.GetNodeEdges(ctx, n, g.context, em) {
		// Only get neighbor nodes if the node is not yet in the visited list
		// If our node is the parent, we skip the visit if the child is in the visited list
		if _, ok := visited[e.Child]; !ok && e.Parent == n.ID {
			_, children := g.backend.GetEdgeNodes(ctx, e, g.context, nil, f)
			for _, child := range children {
				nodes = append(nodes, child)
			}
		} else if _, ok := visited[e.Parent]; !ok && e.Child == n.ID {
			parents, _ := g.backend.GetEdgeNodes(ctx, e, g.context, f, nil)
			for _, parent := range parents {
				nodes = append(nodes, parent)
			}
		}
	}

	return nodes
}

// AreLinked returns true if nodes n1, n2 are linked
func (g *Graph) AreLinked(ctx context.Context, n1 *Node, n2 *Node, m ElementMatcher) bool {
	ctx, span := tracer.Start(ctx, "Graph.AreLinked")
	defer span.End()

	for _, e := range g.backend.GetNodeEdges(ctx, n1, g.context, m) {
		parents, children := g.backend.GetEdgeNodes(ctx, e, g.context, nil, nil)
		if len(parents) == 0 || len(children) == 0 {
			continue
		}

		for i, parent := range parents {
			if children[i].ID == n2.ID || parent.ID == n2.ID {
				return true
			}
		}
	}

	return false
}

// Link the nodes n1, n2 with a new edge
func (g *Graph) Link(ctx context.Context, n1 *Node, n2 *Node, m Metadata, h ...string) (*Edge, error) {
	ctx, span := tracer.Start(ctx, "Graph.Link")
	defer span.End()

	if len(m) > 0 {
		return g.NewEdge(ctx, GenID(), n1, n2, m, h...)
	}
	return g.NewEdge(ctx, GenID(), n1, n2, nil, h...)
}

// Unlink the nodes n1, n2 ; delete the associated edge
func (g *Graph) Unlink(ctx context.Context, n1 *Node, n2 *Node) error {
	ctx, span := tracer.Start(ctx, "Graph.Unlink")
	defer span.End()

	var err error

	for _, e := range g.backend.GetNodeEdges(ctx, n1, liveContext, nil) {
		parents, children := g.backend.GetEdgeNodes(ctx, e, liveContext, nil, nil)
		if len(parents) == 0 || len(children) == 0 {
			continue
		}

		parent, child := parents[0], children[0]
		if child.ID == n2.ID || parent.ID == n2.ID {
			if currErr := g.DelEdge(ctx, e); currErr != nil {
				err = currErr
			}
		}
	}

	return err
}

// GetFirstLink get Link between the parent and the child node
func (g *Graph) GetFirstLink(ctx context.Context, parent, child *Node, m ElementMatcher) *Edge {
	ctx, span := tracer.Start(ctx, "Graph.GetFirstLink")
	defer span.End()

	for _, e := range g.GetNodeEdges(ctx, parent, m) {
		if e.Child == child.ID {
			return e
		}
	}
	return nil
}

// LookupFirstNode returns the fist node matching metadata
func (g *Graph) LookupFirstNode(ctx context.Context, m ElementMatcher) *Node {
	ctx, span := tracer.Start(ctx, "Graph.LookupFirstNode")
	defer span.End()

	nodes := g.GetNodes(ctx, m)
	if len(nodes) > 0 {
		return nodes[0]
	}

	return nil
}

// EdgeAdded add an edge
func (g *Graph) EdgeAdded(ctx context.Context, e *Edge) error {
	ctx, span := tracer.Start(ctx, "Graph.EdgeAdded")
	defer span.End()

	if g.GetEdge(ctx, e.ID) == nil {
		return g.AddEdge(ctx, e)
	}
	return nil
}

// AddEdge in the graph
func (g *Graph) AddEdge(ctx context.Context, e *Edge) error {
	ctx, span := tracer.Start(ctx, "Graph.AddEdge")
	defer span.End()

	if err := g.backend.EdgeAdded(ctx, e); err != nil {
		return err
	}
	g.eventHandler.NotifyEvent(ctx, EdgeAdded, e)

	return nil
}

// GetEdge with Identifier i
func (g *Graph) GetEdge(ctx context.Context, i Identifier) *Edge {
	ctx, span := tracer.Start(ctx, "Graph.GetEdge")
	defer span.End()

	if edges := g.backend.GetEdge(ctx, i, g.context); len(edges) != 0 {
		return edges[0]
	}
	return nil
}

// NodeAdded in the graph
func (g *Graph) NodeAdded(ctx context.Context, n *Node) error {
	ctx, span := tracer.Start(ctx, "Graph.NodeAdded")
	defer span.End()

	if g.GetNode(ctx, n.ID) == nil {
		return g.AddNode(ctx, n)
	}
	return nil
}

// AddNode in the graph
func (g *Graph) AddNode(ctx context.Context, n *Node) error {
	ctx, span := tracer.Start(ctx, "Graph.AddNode")
	defer span.End()

	if err := g.backend.NodeAdded(ctx, n); err != nil {
		return err
	}
	g.eventHandler.NotifyEvent(ctx, NodeAdded, n)

	return nil
}

// GetNode from Identifier
func (g *Graph) GetNode(ctx context.Context, i Identifier) *Node {
	ctx, span := tracer.Start(ctx, "Graph.GetNode")
	defer span.End()

	if nodes := g.backend.GetNode(ctx, i, g.context); len(nodes) != 0 {
		return nodes[0]
	}
	return nil
}

// GetNodeAll from Identifier, all revisions
func (g *Graph) GetNodeAll(ctx context.Context, i Identifier) []*Node {
	ctx, span := tracer.Start(ctx, "Graph.GetNodeAll")
	defer span.End()

	return g.backend.GetNode(ctx, i, g.context)
}

// CreateNode returns a new node not bound to a graph
func CreateNode(i Identifier, m Metadata, t Time, h string, o string) *Node {
	n := &Node{
		graphElement: graphElement{
			ID:        i,
			Host:      h,
			Origin:    o,
			CreatedAt: t,
			UpdatedAt: t,
			Revision:  1,
		},
	}

	if m != nil {
		n.Metadata = m
	} else {
		n.Metadata = make(Metadata)
	}

	return n
}

// CreateNode creates a new node and adds it to the graph
func (g *Graph) CreateNode(i Identifier, m Metadata, t Time, h ...string) *Node {
	hostname := g.host
	if len(h) > 0 {
		hostname = h[0]
	}

	return CreateNode(i, m, t, hostname, g.origin)
}

// NewNode creates a new node in the graph with attached metadata
func (g *Graph) NewNode(ctx context.Context, i Identifier, m Metadata, h ...string) (*Node, error) {
	ctx, span := tracer.Start(ctx, "Graph.NewNode")
	defer span.End()

	n := g.CreateNode(i, m, TimeUTC(), h...)

	if err := g.AddNode(ctx, n); err != nil {
		return nil, err
	}

	return n, nil
}

// CreateEdge returns a new edge not bound to any graph
func CreateEdge(i Identifier, p *Node, c *Node, m Metadata, t Time, h string, o string) *Edge {
	e := &Edge{
		Parent: p.ID,
		Child:  c.ID,
		graphElement: graphElement{
			ID:        i,
			Host:      h,
			Origin:    o,
			CreatedAt: t,
			UpdatedAt: t,
			Revision:  1,
		},
	}

	if m != nil {
		e.Metadata = m
	} else {
		e.Metadata = make(Metadata)
	}

	return e
}

// CreateEdge creates a new edge and adds it to the graph
func (g *Graph) CreateEdge(i Identifier, p *Node, c *Node, m Metadata, t Time, h ...string) *Edge {
	hostname := g.host
	if len(h) > 0 {
		hostname = h[0]
	}

	if i == "" {
		u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(p.ID+c.ID))
		i = Identifier(u.String())
	}

	return CreateEdge(i, p, c, m, t, hostname, g.origin)
}

// NewEdge creates a new edge in the graph based on Identifier, parent, child nodes and metadata
func (g *Graph) NewEdge(ctx context.Context, i Identifier, p *Node, c *Node, m Metadata, h ...string) (*Edge, error) {
	ctx, span := tracer.Start(ctx, "Graph.NewEdge")
	defer span.End()

	e := g.CreateEdge(i, p, c, m, TimeUTC(), h...)

	if err := g.AddEdge(ctx, e); err != nil {
		return nil, err
	}

	return e, nil
}

// EdgeDeleted event
func (g *Graph) EdgeDeleted(ctx context.Context, e *Edge) error {
	ctx, span := tracer.Start(ctx, "Graph.EdgeDeleted")
	defer span.End()

	if err := g.backend.EdgeDeleted(ctx, e); err != nil {
		return err
	}

	g.eventHandler.NotifyEvent(ctx, EdgeDeleted, e)

	return nil
}

func (g *Graph) delEdge(ctx context.Context, e *Edge, t Time) error {
	ctx, span := tracer.Start(ctx, "Graph.delEdge")
	defer span.End()

	e.DeletedAt = t
	if err := g.backend.EdgeDeleted(ctx, e); err != nil {
		return err

	}

	g.eventHandler.NotifyEvent(ctx, EdgeDeleted, e)

	return nil
}

// DelEdge delete an edge
func (g *Graph) DelEdge(ctx context.Context, e *Edge) error {
	ctx, span := tracer.Start(ctx, "Graph.DelEdge")
	defer span.End()

	return g.delEdge(ctx, e, TimeUTC())
}

// NodeDeleted event
func (g *Graph) NodeDeleted(ctx context.Context, n *Node) error {
	ctx, span := tracer.Start(ctx, "Graph.NodeDeleted")
	defer span.End()

	return g.delNode(ctx, n, n.DeletedAt)
}

func (g *Graph) delNode(ctx context.Context, n *Node, t Time) error {
	ctx, span := tracer.Start(ctx, "Graph.delNode")
	defer span.End()

	for _, e := range g.backend.GetNodeEdges(ctx, n, liveContext, nil) {
		if err := g.delEdge(ctx, e, t); err != nil {
			return err
		}
	}

	n.DeletedAt = t
	if err := g.backend.NodeDeleted(ctx, n); err != nil {
		return err
	}

	g.eventHandler.NotifyEvent(ctx, NodeDeleted, n)

	return nil
}

// DelNode delete the node n in the graph
func (g *Graph) DelNode(ctx context.Context, n *Node) error {
	ctx, span := tracer.Start(ctx, "Graph.DelNode")
	defer span.End()

	return g.delNode(ctx, n, TimeUTC())
}

// DelNodes deletes nodes for given matcher
func (g *Graph) DelNodes(ctx context.Context, m ElementMatcher) error {
	ctx, span := tracer.Start(ctx, "Graph.DelNodes")
	defer span.End()

	t := TimeUTC()
	for _, node := range g.GetNodes(ctx, m) {
		if err := g.delNode(ctx, node, t); err != nil {
			return err
		}
	}

	return nil
}

// GetNodes returns a list of nodes
func (g *Graph) GetNodes(ctx context.Context, m ElementMatcher) []*Node {
	ctx, span := tracer.Start(ctx, "Graph.GetNodes")
	addFilterAttribute(span, m, "metadata.filter")
	addTimeFilterAttribute(span, g.context)
	defer span.End()

	nodes := g.backend.GetNodes(ctx, g.context, m, nil)
	span.SetAttributes(attribute.Key("num.nodes.returned").Int(len(nodes)))
	return nodes
}

// GetEdges returns a list of edges
func (g *Graph) GetEdges(ctx context.Context, m ElementMatcher) []*Edge {
	ctx, span := tracer.Start(ctx, "Graph.GetEdges")
	addFilterAttribute(span, m, "metadata.filter")
	addTimeFilterAttribute(span, g.context)
	defer span.End()

	edges := g.backend.GetEdges(ctx, g.context, m, nil)
	span.SetAttributes(attribute.Key("num.edges.returned").Int(len(edges)))
	return edges
}

// GetEdgeNodes returns a list of nodes of an edge
func (g *Graph) GetEdgeNodes(ctx context.Context, e *Edge, parentMetadata, childMetadata ElementMatcher) ([]*Node, []*Node) {
	ctx, span := tracer.Start(ctx, "Graph.GetEdgeNodes")
	addFilterAttribute(span, parentMetadata, "parent.metadata.filter")
	addFilterAttribute(span, childMetadata, "child.metadata.filter")
	addTimeFilterAttribute(span, g.context)
	defer span.End()

	return g.backend.GetEdgeNodes(ctx, e, g.context, parentMetadata, childMetadata)
}

// GetNodeEdges returns a list of edges of a node
func (g *Graph) GetNodeEdges(ctx context.Context, n *Node, m ElementMatcher) []*Edge {
	ctx, span := tracer.Start(ctx, "Graph.GetNodeEdges")
	span.SetAttributes(attribute.Key("node.id").String(string(n.ID)))
	addFilterAttribute(span, m, "metadata.filter")
	addTimeFilterAttribute(span, g.context)
	defer span.End()

	return g.backend.GetNodeEdges(ctx, n, g.context, m)
}

func (g *Graph) String() string {
	j, _ := json.Marshal(g)
	return string(j)
}

// GetOrigin returns the origin of a graph
func (g *Graph) GetOrigin() string {
	return g.origin
}

// Elements returns graph elements
func (g *Graph) Elements(ctx context.Context) *Elements {
	ctx, span := tracer.Start(ctx, "Graph.Elements")
	defer span.End()

	nodes := g.GetNodes(ctx, nil)
	SortNodes(nodes, "CreatedAt", filters.SortOrder_Ascending)

	edges := g.GetEdges(ctx, nil)
	SortEdges(edges, "CreatedAt", filters.SortOrder_Ascending)

	return &Elements{
		Nodes: nodes,
		Edges: edges,
	}
}

// MarshalJSON serialize the graph in JSON
func (g *Graph) MarshalJSON(ctx context.Context) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "Graph.MarshalJSON")
	defer span.End()

	return json.Marshal(g.Elements(ctx))
}

// CloneWithContext creates a new graph based on the given one and the given context
func (g *Graph) CloneWithContext(context Context) (*Graph, error) {
	ng := NewGraph(g.host, g.backend, g.origin)
	if context.TimeSlice != nil && !g.backend.IsHistorySupported() {
		return nil, errors.New("Backend does not support history")
	}
	ng.context = context

	return ng, nil
}

// GetContext returns the current context
func (g *Graph) GetContext() Context {
	return g.context
}

// GetHost returns the graph host
func (g *Graph) GetHost() string {
	return g.host
}

// Diff computes the difference between two graphs
func (g *Graph) Diff(ctx context.Context, newGraph *Graph) (addedNodes []*Node, removedNodes []*Node, addedEdges []*Edge, removedEdges []*Edge) {
	ctx, span := tracer.Start(ctx, "Graph.Diff")
	defer span.End()

	for _, e := range newGraph.GetEdges(ctx, nil) {
		if g.GetEdge(ctx, e.ID) == nil {
			addedEdges = append(addedEdges, e)
		}
	}

	for _, e := range g.GetEdges(ctx, nil) {
		if newGraph.GetEdge(ctx, e.ID) == nil {
			removedEdges = append(removedEdges, e)
		}
	}

	for _, n := range newGraph.GetNodes(ctx, nil) {
		if g.GetNode(ctx, n.ID) == nil {
			addedNodes = append(addedNodes, n)
		}
	}

	for _, n := range g.GetNodes(ctx, nil) {
		if newGraph.GetNode(ctx, n.ID) == nil {
			removedNodes = append(removedNodes, n)
		}
	}

	return
}

// AddEventListener subscribe a new graph listener
func (g *Graph) AddEventListener(l EventListener) {
	g.eventHandler.AddEventListener(l)
}

// RemoveEventListener unsubscribe a graph listener
func (g *Graph) RemoveEventListener(l EventListener) {
	g.eventHandler.RemoveEventListener(l)
}

// NewGraph creates a new graph based on the backend
func NewGraph(host string, backend Backend, origin string) *Graph {
	return &Graph{
		eventHandler: NewEventHandler(maxEvents),
		backend:      backend,
		host:         host,
		context:      Context{TimePoint: true},
		origin:       origin,
	}
}
