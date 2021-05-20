/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package traversal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/topology"
)

// L3PathTraversalExtension describes a new extension to enhance the topology
type L3PathTraversalExtension struct {
	L3PathToken traversal.Token
}

// L3PathGremlinTraversalStep nexthops step
type L3PathGremlinTraversalStep struct {
	context traversal.GremlinTraversalContext
	ip      net.IP
}

// NewL3PathTraversalExtension returns a new graph traversal extension
func NewL3PathTraversalExtension() *L3PathTraversalExtension {
	return &L3PathTraversalExtension{
		L3PathToken: traversalL3PathToken,
	}
}

// ScanIdent returns an associated graph token
func (e *L3PathTraversalExtension) ScanIdent(s string) (traversal.Token, bool) {
	switch s {
	case "L3PATH":
		return e.L3PathToken, true
	}
	return traversal.IDENT, false
}

// ParseStep parses l3path step
func (e *L3PathTraversalExtension) ParseStep(t traversal.Token, p traversal.GremlinTraversalContext) (traversal.GremlinTraversalStep, error) {
	switch t {
	case e.L3PathToken:
	default:
		return nil, nil
	}

	if len(p.Params) != 1 {
		return nil, fmt.Errorf("L3Path accepts one parameter : %v", p.Params)
	}

	ipStr, ok := p.Params[0].(string)
	if !ok {
		return nil, errors.New("L3Path parameter have to be a string")
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, errors.New("L3Path parameter have to be a valid IP address")
	}

	return &L3PathGremlinTraversalStep{context: p, ip: ip}, nil
}

// Exec L3Path step
func (nh *L3PathGremlinTraversalStep) Exec(last traversal.GraphTraversalStep) (traversal.GraphTraversalStep, error) {
	switch tv := last.(type) {
	case *traversal.GraphTraversalV:
		nextHops := make(map[string]map[int]*graph.Node)
		tv.GraphTraversal.RLock()
		for _, node := range tv.GetNodes() {
			hops := make(map[int]*graph.Node)
			if err := topology.GetL3Path(node, nh.ip, hops); err == nil {
				nextHops[string(node.ID)] = hops
			}
		}
		tv.GraphTraversal.RUnlock()

		return NewL3PathTraversalStep(tv.GraphTraversal, nextHops), nil
	}
	return nil, traversal.ErrExecutionError
}

// Reduce l3path step
func (nh *L3PathGremlinTraversalStep) Reduce(next traversal.GremlinTraversalStep) (traversal.GremlinTraversalStep, error) {
	return next, nil
}

// Context L3Path step
func (nh *L3PathGremlinTraversalStep) Context() *traversal.GremlinTraversalContext {
	return &nh.context
}

// L3PathTraversalStep traversal step of l3path
type L3PathTraversalStep struct {
	GraphTraversal *traversal.GraphTraversal
	// For each node, return a map of hops to get to destination
	// The map is used to index each hop: 1:nodeA, 2:nodeB, etc
	nexthops map[string]map[int]*graph.Node
	error    error
}

// NewL3PathTraversalStep creates a new traversal l3path step
func NewL3PathTraversalStep(gt *traversal.GraphTraversal, value map[string]map[int]*graph.Node) *L3PathTraversalStep {
	tv := &L3PathTraversalStep{
		GraphTraversal: gt,
		nexthops:       value,
	}

	return tv
}

// NewL3PathTraversalStepFromError creates a new traversal l3path step
func NewL3PathTraversalStepFromError(err ...error) *L3PathTraversalStep {
	tv := &L3PathTraversalStep{}

	if len(err) > 0 {
		tv.error = err[0]
	}

	return tv
}

// Values return the l3path
func (t *L3PathTraversalStep) Values() []interface{} {
	if len(t.nexthops) == 0 {
		return []interface{}{}
	}
	return []interface{}{t.nexthops}
}

// MarshalJSON serialize in JSON
func (t *L3PathTraversalStep) MarshalJSON() ([]byte, error) {
	values := t.Values()
	t.GraphTraversal.RLock()
	defer t.GraphTraversal.RUnlock()
	return json.Marshal(values)
}

func (t *L3PathTraversalStep) Error() error {
	return t.error
}
