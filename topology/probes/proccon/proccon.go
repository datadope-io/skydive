// +build linux

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

package proccon

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
)

// Probe describes a Docker topology graph that enhance the graph
type Probe struct {
	graph.DefaultGraphListener
	graph *graph.Graph
}

// Fields TODO
type Fields struct {
	Conn   string `json:"conn"`
	Listen string `json:"listen"`
}

// Tags TODO
type Tags struct {
	Cmdline     string `json:"cmdline"`
	Host        string `json:"host"`
	ProcessName string `json:"process_name"`
}

// Metric TODO
type Metric struct {
	Fields    Fields `json:"fields"`
	Name      string `json:"name"`
	Timestamp int    `json:"timestamp"`
	Tags      Tags   `json:"tags"`
}

// TelegrafPacket TODO
type TelegrafPacket struct {
	Metrics []Metric `json:"metrics"`
}

// SerServeHTTP TODO
func (p *Probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var res TelegrafPacket

	err := json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logging.GetLogger().Debugf("Metrica recibida! %+v", res)

	for _, metric := range res.Metrics {
		// Por cada métrica buscamos el nodo al que puede hacer referencia
		// Creo que podríamos usar el "name" (hostname) para encontrar el nodo parent
		// y luego solo intentar hacer match de la cmdline entre sus hijos.
		// Por ahora buscamos sobre todos los nodos el match del cmdline
		nodes := p.graph.GetNodes(graph.Metadata{"Cmdline": metric.Tags.Cmdline})

		if len(nodes) == 0 {
			logging.GetLogger().Debugf("No encontrado ningun node para cmdline '%s'", metric.Tags.Cmdline)
			// TODO en este caso meter al nodo "others" (al saco donde van todas las conex que no sabemos donde meter)
			continue
		} else if len(nodes) > 1 {
			// TODO esto no debería suceder
			logging.GetLogger().Warningf("Encontrados más de un nodo para cmdline '%s': %v. Ignoramos", metric.Tags.Cmdline, nodes)
			continue
		}

		tcpConn := strings.Split(metric.Fields.Conn, ",")
		tcpListen := strings.Split(metric.Fields.Listen, ",")

		node := nodes[0]
		// Pisamos los valores anteriores.
		// TODO: Hacer union?
		node.Metadata.SetField("TCPConn", tcpConn)
		node.Metadata.SetField("TCPListen", tcpListen)
	}

}

// Start TODO
func (p *Probe) Start() error {
	// TODO como gestionar la parada de este en el Stop()
	go http.ListenAndServe("localhost:4000", p)

	return nil
}

// Stop TODO
func (p *Probe) Stop() {
}

// NewProbe TODO
func NewProbe(g *graph.Graph) *Probe {
	probe := &Probe{
		graph: g,
	}

	return probe
}

// Register TODO borrar?
func Register() {
	graph.NodeMetadataDecoders["ProcCon"] = MetadataDecoder
}
