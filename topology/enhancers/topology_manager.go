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

package usertopology

import (
	"context"
	"errors"
	"strings"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/topology"
	"go.opentelemetry.io/otel"
)

// TopologyManager describes topology manager
type TopologyManager struct {
	etcd.MasterElection
	graph.DefaultGraphListener
	watcher1    rest.StoppableWatcher
	watcher2    rest.StoppableWatcher
	nodeHandler *api.NodeRuleAPI
	edgeHandler *api.EdgeRuleAPI
	graph       *graph.Graph
}

var tracer = otel.Tracer("topology.enhancers.usertopology")

// OnStartAsMaster event
func (tm *TopologyManager) OnStartAsMaster() {
}

// OnStartAsSlave event
func (tm *TopologyManager) OnStartAsSlave() {
}

// OnSwitchToMaster event
func (tm *TopologyManager) OnSwitchToMaster() {
	ctx, span := tracer.Start(context.Background(), "TopologyManager.OnSwitchToMaster")
	defer span.End()

	tm.syncTopology(ctx)
}

// OnSwitchToSlave event
func (tm *TopologyManager) OnSwitchToSlave() {
}

func (tm *TopologyManager) syncTopology(ctx context.Context) {
	nodes := tm.nodeHandler.Index(ctx)
	edges := tm.edgeHandler.Index(ctx)

	for _, node := range nodes {
		n := node.(*types.NodeRule)
		tm.handleCreateNode(ctx, n)
	}

	for _, edge := range edges {
		e := edge.(*types.EdgeRule)
		tm.createEdge(ctx, e)
	}
}

func (tm *TopologyManager) createEdge(ctx context.Context, edge *types.EdgeRule) error {
	src := tm.getNodes(edge.Src)
	dst := tm.getNodes(edge.Dst)
	if len(src) < 1 || len(dst) < 1 {
		logging.GetLogger().Errorf("Source or Destination node not found")
		return errors.New("Source or Destination node not found")
	}

	switch edge.Metadata["RelationType"] {
	case "layer2":
		if !topology.HaveLayer2Link(ctx, tm.graph, src[0], dst[0]) {
			topology.AddLayer2Link(ctx, tm.graph, src[0], dst[0], edge.Metadata)
		}
	case "ownership":
		if !topology.HaveOwnershipLink(ctx, tm.graph, src[0], dst[0]) {
			topology.AddOwnershipLink(ctx, tm.graph, src[0], dst[0], nil)
		}
	default:
		// check nodes are already linked
		if tm.graph.AreLinked(ctx, src[0], dst[0], graph.Metadata{"RelationType": edge.Metadata["RelationType"]}) {
			return errors.New("Nodes are already linked")
		}
		id := graph.GenID(string(src[0].ID) + string(dst[0].ID) + edge.Metadata["RelationType"].(string))
		_, err := tm.graph.NewEdge(ctx, id, src[0], dst[0], edge.Metadata)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tm *TopologyManager) nodeID(node *types.NodeRule) graph.Identifier {
	return graph.GenID(node.Metadata["Type"].(string), node.Metadata["Name"].(string))
}

func (tm *TopologyManager) createNode(ctx context.Context, node *types.NodeRule) error {
	id := tm.nodeID(node)
	node.Metadata.SetField("TID", string(id))

	//check node already exist
	if n := tm.graph.GetNode(ctx, id); n != nil {
		return nil
	}

	if node.Metadata["Type"] == "fabric" {
		node.Metadata["Probe"] = "fabric"
	}

	tm.graph.NewNode(ctx, id, node.Metadata, "")
	return nil
}

func (tm *TopologyManager) updateMetadata(ctx context.Context, query string, mdata graph.Metadata) error {
	nodes := tm.getNodes(query)
	for _, n := range nodes {
		mt := tm.graph.StartMetadataTransaction(n)
		for k, v := range mdata {
			mt.AddMetadata(k, v)
		}
		mt.Commit(ctx)
	}
	return nil
}

func (tm *TopologyManager) deleteMetadata(ctx context.Context, query string, mdata graph.Metadata) error {
	nodes := tm.getNodes(query)
	for _, n := range nodes {
		mt := tm.graph.StartMetadataTransaction(n)
		for k := range mdata {
			mt.DelMetadata(k)
		}
		mt.Commit(ctx)
	}

	tm.syncTopology(ctx)
	return nil
}

func (tm *TopologyManager) handleCreateNode(ctx context.Context, node *types.NodeRule) error {
	switch strings.ToLower(node.Action) {
	case "create":
		return tm.createNode(ctx, node)
	case "update":
		return tm.updateMetadata(ctx, node.Query, node.Metadata)
	default:
		logging.GetLogger().Errorf("Query format is wrong. supported prefixes: create and update")
		return errors.New("Query format is wrong")
	}
}

/*This needs to be replaced by gremlin + JS query*/
func (tm *TopologyManager) getNodes(gremlinQuery string) []*graph.Node {
	// TODO esto son los node/edge rules. Donde considerar que empieza la petición?
	res, err := ge.TopologyGremlinQuery(context.Background(), tm.graph, gremlinQuery)
	if err != nil {
		return nil
	}

	var nodes []*graph.Node
	for _, value := range res.Values() {
		switch value.(type) {
		case *graph.Node:
			nodes = append(nodes, value.(*graph.Node))
		case []*graph.Node:
			nodes = append(nodes, value.([]*graph.Node)...)
		}
	}
	return nodes
}

func (tm *TopologyManager) handleNodeRuleRequest(ctx context.Context, action string, resource rest.Resource) error {
	node := resource.(*types.NodeRule)
	switch action {
	case "create", "set":
		return tm.handleCreateNode(ctx, node)
	case "delete":
		switch strings.ToLower(node.Action) {
		case "create":
			id := tm.nodeID(node)
			if n := tm.graph.GetNode(ctx, id); n != nil {
				tm.graph.DelNode(ctx, n)
			}
		case "update":
			tm.deleteMetadata(ctx, node.Query, node.Metadata)
		}
	}
	return nil
}

func (tm *TopologyManager) handleEdgeRuleRequest(ctx context.Context, action string, resource rest.Resource) error {
	edge := resource.(*types.EdgeRule)
	switch action {
	case "create", "set":
		return tm.createEdge(ctx, edge)
	case "delete":
		src := tm.getNodes(edge.Src)
		dst := tm.getNodes(edge.Dst)
		if len(src) < 1 || len(dst) < 1 {
			logging.GetLogger().Errorf("Source or Destination node not found")
			return nil
		}

		if link := tm.graph.GetFirstLink(ctx, src[0], dst[0], edge.Metadata); link != nil {
			if err := tm.graph.DelEdge(ctx, link); err != nil {
				logging.GetLogger().Errorf("Delete Edge failed, error: %v", err)
				return nil
			}
		} else {
			return nil
		}
	}
	return nil
}

func (tm *TopologyManager) onAPIWatcherEvent(action string, id string, resource rest.Resource) {
	ctx, span := tracer.Start(context.Background(), "onAPIWatcherEvent")
	defer span.End()

	switch resource.(type) {
	case *types.NodeRule:
		tm.graph.Lock()
		tm.handleNodeRuleRequest(ctx, action, resource)
		tm.graph.Unlock()
	case *types.EdgeRule:
		logging.GetLogger().Debugf("onAPIWatcherEvent edgerule")
		tm.graph.Lock()
		tm.handleEdgeRuleRequest(ctx, action, resource)
		tm.graph.Unlock()
	}
}

// OnNodeAdded event
func (tm *TopologyManager) OnNodeAdded(ctx context.Context, n *graph.Node) {
	ctx, span := tracer.Start(context.Background(), "OnNodeAdded")
	defer span.End()

	tm.syncTopology(ctx)
}

// OnNodeUpdated event
func (tm *TopologyManager) OnNodeUpdated(ctx context.Context, n *graph.Node, ops []graph.PartiallyUpdatedOp) {
	ctx, span := tracer.Start(context.Background(), "OnNodeUpdated")
	defer span.End()

	tm.syncTopology(ctx)
}

// Start start the topology manager
func (tm *TopologyManager) Start() {
	tm.MasterElection.StartAndWait()

	tm.watcher1 = tm.nodeHandler.AsyncWatch(tm.onAPIWatcherEvent)
	tm.watcher2 = tm.edgeHandler.AsyncWatch(tm.onAPIWatcherEvent)

	tm.graph.AddEventListener(tm)
}

// Stop stop the topology manager
func (tm *TopologyManager) Stop() {
	tm.watcher1.Stop()
	tm.watcher2.Stop()

	tm.MasterElection.Stop()

	tm.graph.RemoveEventListener(tm)
}

// NewTopologyManager returns new topology manager
func NewTopologyManager(etcdClient *etcd.Client, nodeHandler *api.NodeRuleAPI, edgeHandler *api.EdgeRuleAPI, g *graph.Graph) *TopologyManager {
	ctx, span := tracer.Start(context.Background(), "NewTopologyManager")
	defer span.End()

	tm := &TopologyManager{
		nodeHandler: nodeHandler,
		edgeHandler: edgeHandler,
		graph:       g,
	}

	tm.MasterElection = etcdClient.NewElection("/elections/topology-manager")
	tm.MasterElection.AddEventListener(tm)

	tm.graph.Lock()
	tm.syncTopology(ctx)
	tm.graph.Unlock()
	return tm
}
