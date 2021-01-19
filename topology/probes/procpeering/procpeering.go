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

// Package procpeering generate edges (type tcp_conn) between nodes having a TCP connection between them.
// Nodes should have the fields Metadata.TCPConn / Metadata.TCPListen
// The match will be produced when a node have a connection endpoint (the destination) already seen
// in another node (in its TCPListen).
// It means that the first node is connecting to the IP:port of the second node.
//
// This is thought to work with "proccon", which will be the one in charge to add the network info to nodes
//
// To be able to create matchings, when a node with listeners is created or modified, an index is updated.
// When another node with connections is created/modified, it tries to match the listeners.
//
// If we start Skydive with data already present in the backend, those listeners will not be available in the
// index until the nodes are modified.
// TODO load data in startup
package procpeering

import (
	"fmt"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/proccon"
)

const (
	// RelationTypeConnection value for edges connecting two software nodes connected by TCP
	RelationTypeConnection = "tcp_conn"
)

// Probe describes graph peering based on TCP connections
type Probe struct {
	graph.DefaultGraphListener
	graph         *graph.Graph
	listenIndexer *graph.Indexer
	connIndexer   *graph.Indexer
	linker        *graph.ResourceLinker
}

// Start the TCP peering resolver probe
func (p *Probe) Start() error {
	logging.GetLogger().Debug("TCP connections peering")

	err := p.listenIndexer.Start()
	if err != nil {
		return fmt.Errorf("starting listenIndexer: %v", err)
	}

	err = p.connIndexer.Start()
	if err != nil {
		return fmt.Errorf("starting connIndexer: %v", err)
	}

	err = p.linker.Start()
	if err != nil {
		return fmt.Errorf("starting linker: %v", err)
	}

	return nil
}

// Stop the probe
func (p *Probe) Stop() {
	logging.GetLogger().Debug("TCP connections peering")
	p.listenIndexer.Stop()
	p.connIndexer.Stop()
	p.linker.Stop()
}

// OnError implements the LinkerEventListener interface
func (p *Probe) OnError(err error) {
	logging.GetLogger().Error(err)
}

type simpleLinker struct {
	probe *Probe
}

// GetABLinks get nodes with new connections and have to return edges to listeners
func (l *simpleLinker) GetABLinks(nodeConnection *graph.Node) (edges []*graph.Edge) {
	tcpConnIface, err := nodeConnection.Metadata.GetField(proccon.MetadataTCPConnKey)
	if err != nil {
		logging.GetLogger().Debugf("incorrect node '%v' metadata, expecting TCPConn key: %v", nodeConnection, err)
	}

	tcpConnPtr, ok := tcpConnIface.(*proccon.NetworkInfo)
	if !ok {
		return []*graph.Edge{}
	}

	tcpConn := *tcpConnPtr

	// Iterate over node connections and try to find a listener match
	for outgoingConn := range tcpConn { // cambiado de string a interface
		// Only find using the hash with ip and port, ignore process as it will be different in the other side of the connection
		nodesListeners, _ := l.probe.listenIndexer.Get(outgoingConn)

		// Create link from our node to the listener
		for _, nodeListener := range nodesListeners {
			logging.GetLogger().Debugf("Match %s -> %s (%s)", nodeName(nodeConnection), outgoingConn, nodeName(nodeListener))
			edges = append(edges, l.probe.graph.CreateEdge("", nodeConnection, nodeListener, graph.Metadata{
				"RelationType": RelationTypeConnection,
				"Destination":  outgoingConn,
				// We do not have the origin connection info, it is not stored in the node metadata, we only store the destination endpoint
			}, graph.TimeUTC()))
		}

		// Show an error if we find more than one listener
		if len(nodesListeners) > 1 {
			logging.GetLogger().Warningf("node '%+v' connection %v has found more than one listener endpoint: %v", nodeName(nodeConnection), outgoingConn, nodesListeners)
		}
	}

	return edges
}

// GetBALinks not used, we only one listener handler for new connections
func (l *simpleLinker) GetBALinks(n *graph.Node) (edges []*graph.Edge) {
	return nil
}

// connectionsEndpointHasher creates an index of outgoing connections
func connectionsEndpointHasher(n *graph.Node) map[string]interface{} {
	tcpConnIface, err := n.Metadata.GetField(proccon.MetadataTCPConnKey)
	if err != nil {
		return nil
	}
	tcpConnPtr, ok := tcpConnIface.(*proccon.NetworkInfo)
	if !ok {
		return map[string]interface{}{}
	}

	tcpConn := *tcpConnPtr

	kv := make(map[string]interface{}, len(tcpConn))
	for k := range tcpConn {
		// Only create the hash with ip and port, ignore process as it will be different in the other side of the connection
		kv[graph.Hash(k)] = k
	}

	logging.GetLogger().Debugf("Connection index for node %s: %v", nodeName(n), kv)

	return kv
}

// listenEndpointHasher creates an index of listeners
func listenEndpointHasher(n *graph.Node) map[string]interface{} {
	tcpListenIface, err := n.Metadata.GetField(proccon.MetadataListenEndpointKey)
	if err != nil {
		return nil
	}
	tcpListen := *tcpListenIface.(*proccon.NetworkInfo)

	kv := make(map[string]interface{}, len(tcpListen))
	for k := range tcpListen {
		// Only create the hash with ip and port, ignore process as it will be different in the other side of the connection
		kv[graph.Hash(k)] = nil
	}

	logging.GetLogger().Debugf("Listen index for node %s: %v", nodeName(n), kv)
	return kv
}

// nodeName return the Metadata.Name value or ID
// Used in loggers and errors to show a representation of the node
func nodeName(n *graph.Node) string {
	name, err := n.Metadata.GetFieldString(proccon.MetadataNameKey)
	if err == nil {
		return name
	}

	return string(n.ID)
}

// NewProbe creates a new graph node peering probe
func NewProbe(g *graph.Graph) *Probe {
	probe := &Probe{
		graph:         g,
		listenIndexer: graph.NewIndexer(g, g, listenEndpointHasher, false),
		connIndexer:   graph.NewIndexer(g, g, connectionsEndpointHasher, false),
	}

	// Only generate events with connections, not with listeners
	probe.linker = graph.NewResourceLinker(
		g,
		[]graph.ListenerHandler{probe.connIndexer},
		nil,
		&simpleLinker{probe: probe},
		nil,
	)

	// Notify errors using OnError
	probe.linker.AddEventListener(probe)

	// Subscribirnos para obtener eventos de node updated
	g.AddEventListener(probe)

	return probe
}
