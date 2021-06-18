/*
 * Copyright (C) 2018 Orange
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

	"github.com/safchain/insanelock"
)

// NodeAction is a callback to perform on a node. The action is kept
// active as long as it returns true.
type NodeAction interface {
	ProcessNode(ctx context.Context, g *Graph, n *Node) bool
}

// deferred represents a node action with additional info if needed
// for cancellation.
type deferred struct {
	action NodeAction
}

// Processor encapsulates an indexer that will process NodeActions
// on the nodes that filter
type Processor struct {
	insanelock.RWMutex
	DefaultGraphListener
	*MetadataIndexer
	actions map[string][]deferred
}

// NewProcessor creates a Processor on the graph g, a stream of
// events controlled by listenerHandler, that match a first set
// of metadata m. Actions will be associated to a given set
// of values for indexes.
func NewProcessor(g *Graph, listenerHandler ListenerHandler, m ElementMatcher, indexes ...string) (processor *Processor) {
	processor = &Processor{
		MetadataIndexer: NewMetadataIndexer(g, listenerHandler, m, indexes...),
		actions:         make(map[string][]deferred),
	}
	processor.AddEventListener(processor)
	return
}

// DoAction will perform the action for nodes matching values.
func (processor *Processor) DoAction(ctx context.Context, action NodeAction, values ...interface{}) {
	nodes, _ := processor.Get(ctx, values...)
	kont := true
	for _, node := range nodes {
		kont = action.ProcessNode(ctx, processor.graph, node)
		if !kont {
			break
		}
	}
	if kont {
		act := deferred{action: action}
		hash := Hash(values...)
		processor.Lock()
		if actions, ok := processor.actions[hash]; ok {
			processor.actions[hash] = append(actions, act)
		} else {
			actions := []deferred{act}
			processor.actions[hash] = actions
		}
		processor.Unlock()
	}
}

// Cancel the actions attached to a given set of values.
func (processor *Processor) Cancel(values ...interface{}) {
	processor.Lock()
	delete(processor.actions, Hash(values...))
	processor.Unlock()
}

// OnNodeAdded event
func (processor *Processor) OnNodeAdded(ctx context.Context, n *Node) {
	ctx, span := tracer.Start(ctx, "Processor.OnNodeAdded")
	defer span.End()

	if vValues, err := getFieldsAsArray(n, processor.indexes); err == nil {
		for _, values := range vValues {
			hash := Hash(values...)
			processor.RLock()
			actions, ok := processor.actions[hash]
			processor.RUnlock()
			if ok {
				var keep []deferred
				for _, action := range actions {
					if action.action.ProcessNode(ctx, processor.graph, n) {
						keep = append(keep, action)
					}
				}
				processor.Lock()
				if len(keep) == 0 {
					delete(processor.actions, hash)
				} else {
					processor.actions[hash] = keep
				}
				processor.Unlock()
			}
		}
	}
}
