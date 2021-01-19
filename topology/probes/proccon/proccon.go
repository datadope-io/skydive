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
	"fmt"
	"net/http"
	"strings"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

const (
	// ProcconOriginName defines the prefix set in nodes/edges Origin field
	ProcconOriginName = "proccon."
	// MetadataTypeKey key name to store the type of node in node's metadata
	MetadataTypeKey = "Type"
	// MetadataNameKey key name to store the name of the server in node's metadata
	MetadataNameKey = "Name"
	// MetadataTCPConnKey key name to store TCP connections in node's metadata
	MetadataTCPConnKey = "TCPConn"
	// MetadataListenEndpointKey key name to store TCP listening endpoints in node's metadata
	MetadataListenEndpointKey = "TCPListen"
	// MetadataRelationTypeKey key name to store the kind of relation in edge's metadata
	MetadataRelationTypeKey = "RelationType"
	// RelationTypeHasSoftware value of the key RelationType to mark an installed software in a server
	RelationTypeHasSoftware = "has_software"
	// OthersSoftwareNode name of the node type server where connection info is stored when there is not an specific sofware node
	OthersSoftwareNode = "others"
	// MetadataTypeServer value of key Type for nodes representing a server
	MetadataTypeServer = "Server"
	// MetadataTypeSoftware value of key Type for nodes representing a software
	MetadataTypeSoftware = "Software"
	// MetadataCmdlineKey key name to store the cmdline of known Software in nodes of type software
	MetadataCmdlineKey = "Cmdline"
)

// Probe describes this probe
type Probe struct {
	graph *graph.Graph
	// GCdone channel to stop garbageCollector ticker
	GCdone chan bool
}

// Fields is schema of the data with the connection info (inbound and outbound) of the process being added
type Fields struct {
	Conn   string `json:"conn"`
	Listen string `json:"listen"`
}

// Tags is schema of the data when the external agents define the process being added
type Tags struct {
	Cmdline     string `json:"cmdline"`
	Host        string `json:"host"`
	ProcessName string `json:"process_name"`
}

// Metric represents each of the processes found by the external agent
type Metric struct {
	Fields    Fields `json:"fields"`
	Name      string `json:"name"`
	Timestamp int    `json:"timestamp"`
	Tags      Tags   `json:"tags"`
}

// ProcInfo store info associated to each TCP connection or listen endpoint.
// It is used to delete old conections and be able to tell which connections
// are seen several times
type ProcInfo struct {
	CreatedAt int64
	UpdatedAt int64
	Revision  int64
}

// TelegrafPacket is schema of the data POSTed to this probe by external agents. Following the JSON format of Telegraf output plugin for the addapted procs plugins
type TelegrafPacket struct {
	Metrics []Metric `json:"metrics"`
}

// SerServeHTTP receive HTTP POST requests from Telegraf nodes with processes data
func (p *Probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var res TelegrafPacket

	// Decode the Telegraf JSON packet into the TelegrafPacket struct
	err := json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotAcceptable)
		return
	}

	// For each metric, found the matching node in the graph, or, if it does not exists, create one
	for _, metric := range res.Metrics {
		// Get lock between querying if the node exists and creating it
		p.graph.Lock()

		// Find server-type nodes with mathing name
		nodes := p.graph.GetNodes(graph.Metadata{
			MetadataTypeKey: MetadataTypeServer,
			MetadataNameKey: metric.Tags.Host,
		})

		var hostNode *graph.Node
		var err error

		if len(nodes) == 0 {
			logging.GetLogger().Debugf("Node not found with Metadata.Name '%s', creating it", metric.Tags.Host)

			logging.GetLogger().Debugf("newNode(Server, %v)", metric.Tags.Host)
			hostNode, err = p.newNode(metric.Tags.Host, graph.Metadata{
				MetadataNameKey: metric.Tags.Host,
				MetadataTypeKey: MetadataTypeServer,
			})
			if err != nil {
				logging.GetLogger().Errorf("Creating %s server node: %v.", metric.Tags.Host, err)
				continue
			}
		} else if len(nodes) > 1 {
			logging.GetLogger().Errorf("Found more than one node with Metadata.Name '%s'. This sould not happen. Ignoring", metric.Tags.Host)
			continue
		} else {
			hostNode = nodes[0]
		}
		// Release lock to allow others operations to progress
		p.graph.Unlock()

		// Get lock between querying if the node exists and creating it
		p.graph.Lock()
		// Get child Software nodes with matching cmdline
		childNodes := p.graph.LookupChildren(
			hostNode,
			graph.Metadata{MetadataTypeKey: MetadataTypeSoftware, MetadataCmdlineKey: metric.Tags.Cmdline},
			graph.Metadata{MetadataRelationTypeKey: RelationTypeHasSoftware},
		)
		// appendToOthers have its own lock handler
		p.graph.Unlock()

		if len(childNodes) == 0 {
			logging.GetLogger().Debugf("Software node not found for Server node '%v' and cmdline '%s', storing in others", nodeName(hostNode), metric.Tags.Cmdline)
			p.appendToOthers(hostNode, metric)
			continue
		} else if len(childNodes) > 1 {
			// This should not happen
			logging.GetLogger().Errorf("Found more than one Software node for Server node '%v' and cmdline '%s': %+v. Ignoring", nodeName(hostNode), metric.Tags.Cmdline, childNodes)
			continue
		}

		// Software node already exists

		// Delete this connections from "others" node (in case now we have a software node to put that connections into)
		// Maybe this function is too expensive? Another option is to leave the connections and the garbageCollector will take care, but we will
		// have duplicated edges till the garbageCollector cleans others
		err = p.removeFromOthers(hostNode, metric)
		if err != nil {
			logging.GetLogger().Errorf("Trying to delete connections from known software from 'others' software")
		}

		swNode := childNodes[0]

		// Avoid having an array with an empty element if the string received is the empty string
		tcpConn := []string{}
		if metric.Fields.Conn != "" {
			tcpConn = strings.Split(metric.Fields.Conn, ",")
		}

		tcpListen := []string{}
		if metric.Fields.Listen != "" {
			tcpListen = strings.Split(metric.Fields.Listen, ",")
		}

		err = p.addNetworkInfo(swNode, tcpConn, tcpListen)
		if err != nil {
			logging.GetLogger().Errorf("Not able to add network info to Software '%s' (host %+v)", nodeName(swNode), hostNode)
		}
	}
}

// appendToOthers add the metric to a "fake" software node used to store all connections
// from unknown software.
// Create the node if neccessary.
// The software node will be called "others"
func (p *Probe) appendToOthers(hostNode *graph.Node, metric Metric) {
	// Get the lock to avoid a race condition between asking if the node exists and creating it
	p.graph.Lock()

	othersNodes := p.graph.LookupChildren(
		hostNode,
		graph.Metadata{MetadataTypeKey: MetadataTypeSoftware, MetadataNameKey: OthersSoftwareNode},
		graph.Metadata{MetadataRelationTypeKey: RelationTypeHasSoftware},
	)

	var otherNode *graph.Node
	var err error

	if len(othersNodes) == 0 {
		// Create the node
		logging.GetLogger().Debugf("newNode(Software, others)")
		otherNode, err = p.newNode(metric.Tags.Host, graph.Metadata{
			MetadataNameKey: OthersSoftwareNode,
			MetadataTypeKey: MetadataTypeSoftware,
		})
		if err != nil {
			logging.GetLogger().Errorf("Creating 'others' Software node for Server node '%+v': %v.", hostNode, err)
			return
		}

		// Create edge to link to the host
		logging.GetLogger().Debugf("newEdge(%v, %v, has_software)", hostNode, otherNode)
		_, err = p.newEdge(metric.Tags.Host, hostNode, otherNode, graph.Metadata{
			MetadataRelationTypeKey: RelationTypeHasSoftware,
		})
		if err != nil {
			logging.GetLogger().Errorf("Linking 'others' Software node to host Server node '%+v': %v.", hostNode, err)
			return
		}

	} else if len(othersNodes) > 1 {
		logging.GetLogger().Errorf("Found more than one 'others' Software node for Server node '%+v'. This should not happen. Not adding metrics", hostNode)
		return

	} else {
		// Append info
		otherNode = othersNodes[0]
	}

	// Frees graph lock before addNetworkInfo, as that function grab also the lock
	p.graph.Unlock()

	// Avoid having an array with an empty element if the string received is the empty string
	tcpConn := []string{}
	if metric.Fields.Conn != "" {
		tcpConn = strings.Split(metric.Fields.Conn, ",")
	}

	tcpListen := []string{}
	if metric.Fields.Listen != "" {
		tcpListen = strings.Split(metric.Fields.Listen, ",")
	}

	err = p.addNetworkInfo(otherNode, tcpConn, tcpListen)
	if err != nil {
		logging.GetLogger().Errorf("Not able to add network info to Software '%+s' (host %+v)", otherNode, hostNode)
	}
}

// removeFromOthers remove connections/listeners from the "others" software node
func (p *Probe) removeFromOthers(hostNode *graph.Node, metric Metric) error {
	p.graph.Lock()
	defer p.graph.Unlock()

	othersNodes := p.graph.LookupChildren(
		hostNode,
		graph.Metadata{MetadataTypeKey: MetadataTypeSoftware, MetadataNameKey: OthersSoftwareNode},
		graph.Metadata{MetadataRelationTypeKey: RelationTypeHasSoftware},
	)

	if len(othersNodes) == 0 {
		// do nothing
		return nil
	} else if len(othersNodes) > 1 {
		// This should not happen
		return fmt.Errorf("Found more than one 'others' Software node for Server node '%+v'. Not removing metrics", hostNode)
	}

	otherNode := othersNodes[0]

	// Remove from connections/listeners lists in otherNode the values found in "metric"
	removeKeysFromList := func(field interface{}, metrics []string) (ret bool) {
		info := field.(map[string]ProcInfo)
		for _, v := range metrics {
			if _, ok := info[v]; ok {
				delete(info, v)
				ret = true
			}
		}
		return ret
	}

	removeTCPConn := func(field interface{}) (ret bool) {
		metricsTCPConn := strings.Split(metric.Fields.Conn, ",")
		return removeKeysFromList(field, metricsTCPConn)
	}

	err := p.graph.UpdateMetadata(otherNode, MetadataTCPConnKey, removeTCPConn)
	if err != nil {
		return fmt.Errorf("unable to delete old TCP connections: %v", err)
	}

	removeListenEndpoints := func(field interface{}) (ret bool) {
		metricsListenersEndpoints := strings.Split(metric.Fields.Listen, ",")
		return removeKeysFromList(field, metricsListenersEndpoints)
	}

	err = p.graph.UpdateMetadata(otherNode, MetadataListenEndpointKey, removeListenEndpoints)
	if err != nil {
		return fmt.Errorf("unable to delete old listen endpoints: %v", err)
	}

	return nil
}

// generateProcInfoData from a list of connections or endpoints, return a dict being the key
// the connection string and the value a ProcInfo struct initializated to now a 0
func generateProcInfoData(conn []string) map[string]ProcInfo {
	ret := map[string]ProcInfo{}
	for _, c := range conn {
		ret[c] = ProcInfo{
			CreatedAt: graph.TimeNow().UnixMilli(),
			UpdatedAt: graph.TimeNow().UnixMilli(),
			Revision:  1,
		}
	}

	return ret
}

// addNetworkInfo append connection and listen endpoints to the metadata of the server
// Only one function for both (tcpConn and tcpListen) to avoid two modifications of the metadata, avoiding extra work for the backend
func (p *Probe) addNetworkInfo(node *graph.Node, tcpConn []string, listenEndpoints []string) error {
	p.graph.Lock()
	defer p.graph.Unlock()

	// Add new network info and update current stored one
	updateNetworkMetadata := func(field interface{}, newData map[string]ProcInfo) bool {
		currentData := field.(map[string]ProcInfo)
		for k, v := range newData {
			netData, ok := currentData[k]
			if ok {
				// If network info exists, update Revision and UpdatedAt
				netData.UpdatedAt = graph.TimeNow().UnixMilli()
				netData.Revision++
				currentData[k] = netData
			} else {
				// If the network info did not exists, assign the new values
				currentData[k] = v
			}
		}
		return len(newData) > 0
	}

	// Connection info converted to the data structure stored in the node metadata
	tcpConnStruct := generateProcInfoData(tcpConn)

	errTCPConn := p.graph.UpdateMetadata(node, MetadataTCPConnKey, func(field interface{}) bool {
		return updateNetworkMetadata(field, tcpConnStruct)
	})

	listenEndpointsStruct := generateProcInfoData(listenEndpoints)

	errListenEndpoint := p.graph.UpdateMetadata(node, MetadataListenEndpointKey, func(field interface{}) bool {
		return updateNetworkMetadata(field, listenEndpointsStruct)
	})

	// UpdateMetadata will fail if the metadata key does not exists
	// In that case, use AddMetadata to create that keys
	if errTCPConn != nil || errListenEndpoint != nil {
		tr := p.graph.StartMetadataTransaction(node)
		if errTCPConn != nil {
			tr.AddMetadata(MetadataTCPConnKey, tcpConnStruct)
		}
		if errListenEndpoint != nil {
			tr.AddMetadata(MetadataListenEndpointKey, listenEndpointsStruct)
		}
		if err := tr.Commit(); err != nil {
			return fmt.Errorf("Unable to set metadata in node %s: %v", nodeName(node), err)
		}
	}

	return nil
}

// removeOldNetworkInformation delete TCP connections and listen endpoints which update time
// is less than "thresholdTime"
func (p *Probe) removeOldNetworkInformation(node *graph.Node, thresholdTime time.Time) error {
	p.graph.Lock()
	defer p.graph.Unlock()

	tt := graph.Time(thresholdTime)

	removeOld := func(field interface{}) (ret bool) {
		info := field.(map[string]ProcInfo)
		for k, v := range info {
			if v.UpdatedAt < tt.UnixMilli() {
				delete(info, k)
				ret = true
			}
		}
		return ret
	}

	err := p.graph.UpdateMetadata(node, MetadataTCPConnKey, removeOld)
	if err != nil {
		return fmt.Errorf("unable to delete old TCP connections: %v", err)
	}
	err = p.graph.UpdateMetadata(node, MetadataListenEndpointKey, removeOld)
	if err != nil {
		return fmt.Errorf("unable to delete old listen endpoints: %v", err)
	}
	return nil
}

// cleanSoftwareNodes delete old data from type Software nodes
// "now" is the current time (parametrized to be able to test this function)
// "expiredConnection" is used to determine if connections/listeners are too old
func (p *Probe) cleanSoftwareNodes(expiredConnectionThreshold time.Time) {
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	softwareNodes := p.graph.GetNodes(softwareNodeFilter)

	logging.GetLogger().Debugf("GarbageCollector, delete network info older than %v. Processing %v nodes", expiredConnectionThreshold, len(softwareNodes))

	for _, n := range softwareNodes {
		err := p.removeOldNetworkInformation(n, expiredConnectionThreshold)
		if err != nil {
			logging.GetLogger().Warningf("Deleting old network information on node %+v: %v", n, err)
		}
	}
}

// garbageCollector executes periodically functions to clean old data
// interval is how often will be executed the garbageCollector
// expiredConnection is how old have to be a connection to be deleted
func (p *Probe) garbageCollector(interval, expiredConnection time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.GCdone:
			logging.GetLogger().Debug("Closing garbageCollector")
			return
		case <-ticker.C:
			logging.GetLogger().Debug("Executing garbageCollector")
			p.cleanSoftwareNodes(time.Now().Add(-expiredConnection))
		}
	}
}

// nodeName return the Metadata.Name value or ID
// Used in loggers and errors to show a representation of the node
func nodeName(n *graph.Node) string {
	name, err := n.Metadata.GetFieldString(MetadataNameKey)
	if err == nil {
		return name
	}

	return string(n.ID)
}

// newEdge creates and inserts a new edge in the graph, using a random ID, setting the
// Origin field to "proccon"+g.Origin and the Host field to the param "host"
func (p *Probe) newEdge(host string, n *graph.Node, c *graph.Node, m graph.Metadata) (*graph.Edge, error) {
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(n.ID+c.ID))
	i := graph.Identifier(u.String())

	e := graph.CreateEdge(i, n, c, m, graph.TimeUTC(), host, ProcconOriginName+p.graph.GetOrigin())

	if err := p.graph.AddEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// newNode creates and inserts a new node in the graph, using a random ID, setting the
// Origin field to "proccon"+g.Origin and the Host field to the param "host"
func (p *Probe) newNode(host string, m graph.Metadata) (*graph.Node, error) {
	i := graph.GenID()
	n := graph.CreateNode(i, m, graph.TimeUTC(), host, ProcconOriginName+p.graph.GetOrigin())

	if err := p.graph.AddNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

// Start initilizates the proccon probe, starting a web server to receive data and the garbage collector to delete old info
func (p *Probe) Start() error {
	listenEndpoint := config.GetString("analyzer.topology.proccon.listen")
	go http.ListenAndServe(listenEndpoint, p)
	logging.GetLogger().Infof("Listening for new network metrics on %v", listenEndpoint)

	p.GCdone = make(chan bool)

	intervalConfig := config.GetString("analyzer.topology.proccon.garbage_collector.interval")
	interval, err := time.ParseDuration(intervalConfig)
	if err != nil {
		logging.GetLogger().Fatalf("Invalid analyzer.topology.proccon.garbage_collector.interval value: %v", err)
	}

	deleteDurationConfig := config.GetString("analyzer.topology.proccon.garbage_collector.delete_duration")
	deleteDuration, err := time.ParseDuration(deleteDurationConfig)
	if err != nil {
		logging.GetLogger().Fatalf("Invalid analyzer.topology.proccon.garbage_collector.delete_duration value: %v", err)
	}

	go p.garbageCollector(interval, deleteDuration)

	return nil
}

// Stop garbageCollector
func (p *Probe) Stop() {
	p.GCdone <- true
}

// NewProbe initialize the probe with the parameters from the config file
func NewProbe(g *graph.Graph) *Probe {
	probe := &Probe{
		graph: g,
	}

	return probe
}

// Register is part of the setup
func Register() {
	// not used
	graph.NodeMetadataDecoders["ProcCon"] = MetadataDecoder
}
