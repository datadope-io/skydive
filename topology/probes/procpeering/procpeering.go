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

package procpeering

import (
	"fmt"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// Probe describes graph peering based on MAC address and graph events
type Probe struct {
	graph.DefaultGraphListener
	graph         *graph.Graph
	listenIndexer *graph.Indexer
	connIndexer   *graph.Indexer
	linker        *graph.ResourceLinker
}

// Start the MAC peering resolver probe
func (p *Probe) Start() error {
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
	p.listenIndexer.Stop()
	p.connIndexer.Stop()
	p.linker.Stop()
}

// OnError implements the LinkerEventListener interface
func (p *Probe) OnError(err error) {
	// TODO having errors: "(*Probe).OnError  archer: Edge ID conflict"
	logging.GetLogger().Error(err)
}

// Prueba linker custom
type simpleLinker struct {
	probe *Probe
}

// TODO podríamos tener problemas por linkear IPs privadas reutilizadas.
// Por ejemplo, muchos hosts pueden tener configurada la IP 172.17.0.1 para la interfaz
// de docker.
// Cosas que estuviesen conectando localmente a esa IP podrían acabar enganchando a
// cualquier host que tuviese docker con esa interfaz

// GetABLinks get nodes with new connections and have to return edges to listeners
func (l *simpleLinker) GetABLinks(n *graph.Node) (edges []*graph.Edge) {
	tcpConn, err := n.Metadata.GetField("TCPConn")
	if err != nil {
		logging.GetLogger().Debugf("incorrect node '%v' metadata, expecting TCPConn key: %v", n.Host, err)
	}

	// Iterate over node connections and try to find a listener match
	for _, c := range tcpConn.([]interface{}) {
		cc, ok := c.(map[string]interface{})
		if !ok {
			logging.GetLogger().Errorf("incorrect metadata format for TCPConn in node '%v': %v", n.Host, c)
		}
		// Only find using the hash with ip and port, ignore process as it will be different in the other side of the connection
		nodes, _ := l.probe.listenIndexer.Get(cc["IP"], cc["port"])

		// Create link from our node to the listener
		for _, n2 := range nodes {
			edges = append(edges, l.probe.graph.CreateEdge("", n, n2, graph.Metadata{
				"RelationType": "tcp_conn",
				"Destination":  fmt.Sprintf("%v:%v", cc["IP"], cc["port"]),
				// We do not have the origin connection info, it is not stored in the node metadata
			}, graph.TimeUTC()))
		}

		// Show an error if we find more than one listener
		// TODO how to handle this?
		if len(nodes) > 1 {
			logging.GetLogger().Errorf("node '%+v' connection %v has found more than one listener endpoint: %v", n, c, nodes)
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
	tcpConnIface, err := n.Metadata.GetField("TCPConn")
	if err != nil {
		return nil
	}
	tcpConn := tcpConnIface.([]interface{})

	kv := make(map[string]interface{}, len(tcpConn))
	for _, v := range tcpConn {
		vv, ok := v.(map[string]interface{})
		if !ok {
			logging.GetLogger().Errorf("incorrect metadata format for TCPConn in node %v: %v", n.Host, v)
		}
		// Only create the hash with ip and port, ignore process as it will be different in the other side of the connection
		kv[graph.Hash(vv["IP"], vv["port"])] = fmt.Sprintf("%v:%v", vv["IP"], vv["port"])
	}

	return kv
}

// listenEndpointHasher creates an index of listeners
func listenEndpointHasher(n *graph.Node) map[string]interface{} {
	tcpListenIface, err := n.Metadata.GetField("TCPListen")
	if err != nil {
		return nil
	}
	tcpListen := tcpListenIface.([]interface{})

	kv := make(map[string]interface{}, len(tcpListen))
	for _, v := range tcpListen {
		vv, ok := v.(map[string]interface{})
		if !ok {
			logging.GetLogger().Errorf("incorrect metadata format for TCPListen in node %v: %v", n.Host, v)
		}
		// Only create the hash with ip and port, ignore process as it will be different in the other side of the connection
		kv[graph.Hash(vv["IP"], vv["port"])] = nil
	}

	return kv
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

	return probe
}
