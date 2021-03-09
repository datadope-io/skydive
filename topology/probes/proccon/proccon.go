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

// Package proccon provides an additional API to add metrics to Skydive.
// It accept HTTP-JSON POST like this:
// {
//   "metrics": [
//     {
//       "fields": {
//         "conn": "1.2.3.4:80,9.9.9.9:53",
//         "listen": "192.168.1.36:8000,192.168.1.22:8000"
//       },
//       "name": "procstat_test",
//       "tags": {
//         "cmdline": "nc -kl 8000",
//         "host": "fooBar",
//         "process_name": "nc",
//         "conn_prefix": ""  // optional value
//       },
//       "timestamp": 1603890543
//     }
//   ]
// }
//
// This will create, if does not already exists, a node Type=Server and other
// node Type=Software linked to the previous one.
// It will find if there is already a software who matches the cmdline.
// If not, it will use one with name "others".
//
// In the software node it will add the network info in the Metadata like:
// "Metadata": {
//   "TCPConn": {
//     "1.2.3.4:80": {
//       "CreatedAt": 161114359940
//       "UpdatedAt": 161114367803
//       "Revision": 2
//     },
//     "9.9.9.9:53": {
//       "CreatedAt": 161114359940
//       "UpdatedAt": 161114367803
//       "Revision": 2
//     }
//   },
//   "TCPListen": {
//     "192.168.1.36:8000": {
//       "CreatedAt": 161114359940
//       "UpdatedAt": 161114367803
//       "Revision": 2
//     },
//     "192.168.1.22:8000": {
//       "CreatedAt": 161114359940
//       "UpdatedAt": 161114367803
//       "Revision": 2
//     }
//   },
//   "Name": "others",
//   "Type": "Software"
// }
//
// This plugin it is thought to work with procpeering, which will create edges
// between connected nodes (one node having a TCPConn matching a TCPListen of
// another node).
package proccon

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/tinylib/msgp/msgp"
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
	// MetricFieldConn is the field key where connection info is received
	MetricFieldConn = "conn"
	// MetricFieldListen the field key where listen info is received
	MetricFieldListen = "listen"
	// MetricTagConnPrefix is an optional tag value to prefix network information with, to be able to
	// distinguish between same private IPs in different network partitions
	MetricTagConnPrefix = "conn_prefix"
)

// Probe describes this probe
type Probe struct {
	graph *graph.Graph
	// GCdone channel to stop garbageCollector ticker
	GCdone chan bool
	// nodeRevisionForceFlush defines after how many node updates it is synced to the backend.
	// A small number will produce a lot of modifications in the backend because of network info updates.
	// A big number will leave behind the backend, loosing info in case of a restart of skydive
	nodeRevisionForceFlush int64
}

// SerServeHTTP receive HTTP POST requests from Telegraf nodes with processes data
func (p *Probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var metrics []Metric

	defer r.Body.Close()

	// Decode the Telegraf JSON packet into the TelegrafPacket struct
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get X-Real-IP or client IP for error logging purposes
	source := r.Header.Get("X-Real-IP")
	if source == "" {
		source = strings.Split(r.RemoteAddr, ":")[0]
	}

	for {
		var metric Metric
		leftovervalues, err := metric.UnmarshalMsg(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			logging.GetLogger().Warningf("invalid metric: %v (client: %+v)", err, source)
			return
		}
		metrics = append(metrics, metric)
		body = leftovervalues
		if len(body) == 0 {
			break
		}
	}

	// Accumulate "others" connections to generate only one modification in the node
	others := map[*graph.Node][]Metric{}

	// For each metric, found the matching node in the graph, or, if it does not exists, create one
	for _, metric := range metrics {
		// Get lock between querying if the node exists and creating it
		p.graph.Lock()

		// Find server-type nodes with mathing name
		nodes := p.graph.GetNodes(graph.Metadata{
			MetadataTypeKey: MetadataTypeServer,
			MetadataNameKey: metric.Tags["host"],
		})

		var hostNode *graph.Node
		var err error

		if len(nodes) == 0 {
			logging.GetLogger().Debugf("Node not found with Metadata.Name '%s', creating it", metric.Tags["host"])

			logging.GetLogger().Debugf("newNode(Server, %v)", metric.Tags["host"])
			hostNode, err = p.newNode(metric.Tags["host"], graph.Metadata{
				MetadataNameKey: metric.Tags["host"],
				MetadataTypeKey: MetadataTypeServer,
			})
			if err != nil {
				logging.GetLogger().Errorf("Creating %s server node: %v.", metric.Tags["host"], err)
				continue
			}
		} else if len(nodes) > 1 {
			logging.GetLogger().Errorf("Found more than one node with Metadata.Name '%s'. This sould not happen. Ignoring", metric.Tags["host"])
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
			graph.Metadata{MetadataTypeKey: MetadataTypeSoftware, MetadataCmdlineKey: metric.Tags["cmdline"]},
			graph.Metadata{MetadataRelationTypeKey: RelationTypeHasSoftware},
		)
		// appendToOthers have its own lock handler
		// TODO: quitar el unlock? O bajar al final de la función?
		p.graph.Unlock()

		if len(childNodes) == 0 {
			logging.GetLogger().Debugf("Software node not found for Server node '%v' and cmdline '%s', storing in others", nodeName(hostNode), metric.Tags["cmdline"])
			// Accumulate changes to "others" to make only one change to the node
			others[hostNode] = append(others[hostNode], metric)
			continue
		} else if len(childNodes) > 1 {
			// This should not happen
			logging.GetLogger().Errorf("Found more than one Software node for Server node '%v' and cmdline '%s': %+v. Ignoring", nodeName(hostNode), metric.Tags["cmdline"], childNodes)
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

		// Attach that network information to the software node
		err = p.addNetworkInfo(swNode, []Metric{metric})
		if err != nil {
			logging.GetLogger().Errorf("Not able to add network info to Software '%s' (host %+v)", nodeName(swNode), hostNode)
		}
	}

	// Handle accumulated "others" software connections
	p.generateOthers(others)

	w.Write([]byte("OK"))
}

// generateOthers get the accumulated values of metrics that should be written to the "others" node.
// This node stores all connections that does not have a custom Software node.
// This funcion will create the "others" software node if needed.
func (p *Probe) generateOthers(others map[*graph.Node][]Metric) {
	for hostNode, metrics := range others {
		host := hostNode.Host

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
			otherNode, err = p.newNode(host, graph.Metadata{
				MetadataNameKey: OthersSoftwareNode,
				MetadataTypeKey: MetadataTypeSoftware,
			})
			if err != nil {
				logging.GetLogger().Errorf("Creating 'others' Software node for Server node '%+v': %v.", hostNode, err)
				return
			}

			// Create edge to link to the host
			logging.GetLogger().Debugf("newEdge(%v, %v, has_software)", hostNode, otherNode)
			_, err = p.newEdge(host, hostNode, otherNode, graph.Metadata{
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

		err = p.addNetworkInfo(otherNode, metrics)
		if err != nil {
			logging.GetLogger().Errorf("Not able to add network info to Software '%s' (host %v): %v", nodeName(otherNode), nodeName(hostNode), err)
		}
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
		info := *field.(*NetworkInfo)
		for _, v := range metrics {
			if _, ok := info[v]; ok {
				delete(info, v)
				ret = true
			}
		}
		return ret
	}

	removeTCPConn := func(field interface{}) (ret bool) {
		metricsTCPConn := strings.Split(metric.Fields[MetricFieldConn], ",")
		return removeKeysFromList(field, metricsTCPConn)
	}

	err := p.graph.UpdateMetadata(otherNode, MetadataTCPConnKey, removeTCPConn)
	if err != nil {
		return fmt.Errorf("unable to delete old TCP connections: %v", err)
	}

	removeListenEndpoints := func(field interface{}) (ret bool) {
		metricsListenersEndpoints := strings.Split(metric.Fields[MetricFieldListen], ",")
		return removeKeysFromList(field, metricsListenersEndpoints)
	}

	err = p.graph.UpdateMetadata(otherNode, MetadataListenEndpointKey, removeListenEndpoints)
	if err != nil {
		return fmt.Errorf("unable to delete old listen endpoints: %v", err)
	}

	return nil
}

// appendProcInfoData given a list of connetions/endpoints, for each element append it, with
// the appropiate struct, to the netInfo variable.
// Revision is initialized to 1
func appendProcInfoData(conn []string, metricTimestamp int64, netInfo NetworkInfo) {
	for _, c := range conn {
		netInfo[c] = ProcInfo{
			CreatedAt: metricTimestamp,
			UpdatedAt: metricTimestamp,
			Revision:  1,
		}
	}
}

// updateNetworkMetadata adds new network info and update current stored one.
// Only return true if there is new data.
// Modifying UpdatedAt and Revision fields are not considered modifications (save flushes to the backed)
// To don't leave the backend too behind, each N node Revisions consider it a modification.
func (p *Probe) updateNetworkMetadata(field interface{}, newData NetworkInfo, nodeRevision int64) (ret bool) {
	currentData := *field.(*NetworkInfo)
	for k, v := range newData {
		netData, ok := currentData[k]
		if ok {
			// If network info exists, update Revision and UpdatedAt
			netData.UpdatedAt = v.UpdatedAt
			netData.Revision++
			currentData[k] = netData
		} else {
			// If the network info did not exists, assign the new values
			currentData[k] = v
			ret = true
		}
	}
	// If node p.nodeRevisionForceFlush is 100, when nodeRevision is 100, 200, etc, the function
	// will return true, forcing an update in the backend.
	// Avoid panic if p.nodeRevisionForceFlush is 0
	return ret || (p.nodeRevisionForceFlush != 0 && (nodeRevision%p.nodeRevisionForceFlush == 0))
}

// addNetworkInfo append connection and listen endpoints to the metadata of the server
func (p *Probe) addNetworkInfo(node *graph.Node, metrics []Metric) error {
	// Accumulate conn/endpoint of all metrics in this vars
	tcpConnStruct := NetworkInfo{}
	listenEndpointsStruct := NetworkInfo{}

	for _, metric := range metrics {
		// Generate an slice from the comma separated list in the metric received
		// Avoid having an array with an empty element if the string received is the empty string
		tcpConn := []string{}
		if metric.Fields[MetricFieldConn] != "" {
			tcpConn = strings.Split(metric.Fields[MetricFieldConn], ",")
			// Add ConnPrefix if defined
			if metric.Tags[MetricTagConnPrefix] != "" {
				for i := 0; i < len(tcpConn); i++ {
					tcpConn[i] = metric.Tags[MetricTagConnPrefix] + tcpConn[i]
				}
			}
		}

		// Same for the listen endpoints
		listenEndpoints := []string{}
		if metric.Fields[MetricFieldListen] != "" {
			listenEndpoints = strings.Split(metric.Fields[MetricFieldListen], ",")
			// Add ConnPrefix if defined
			if metric.Tags[MetricTagConnPrefix] != "" {
				for i := 0; i < len(listenEndpoints); i++ {
					listenEndpoints[i] = metric.Tags[MetricTagConnPrefix] + listenEndpoints[i]
				}
			}
		}

		// Convert metric timestamp into ms in int64
		timestampMs := metric.Time.time.UnixNano() / int64(time.Millisecond)

		// Network info converted to the data structure stored in the node metadata
		appendProcInfoData(tcpConn, timestampMs, tcpConnStruct)
		appendProcInfoData(listenEndpoints, timestampMs, listenEndpointsStruct)
	}

	p.graph.Lock()
	defer p.graph.Unlock()

	// Updates nodes metadata
	errTCPConn := p.graph.UpdateMetadata(node, MetadataTCPConnKey, func(field interface{}) bool {
		return p.updateNetworkMetadata(field, tcpConnStruct, node.Revision)
	})

	errListenEndpoint := p.graph.UpdateMetadata(node, MetadataListenEndpointKey, func(field interface{}) bool {
		return p.updateNetworkMetadata(field, listenEndpointsStruct, node.Revision)
	})

	// UpdateMetadata will fail if the metadata key does not exists
	// In that case, use AddMetadata to create that keys
	if errTCPConn != nil || errListenEndpoint != nil {
		tr := p.graph.StartMetadataTransaction(node)
		if errTCPConn != nil {
			tr.AddMetadata(MetadataTCPConnKey, &tcpConnStruct)
		}
		if errListenEndpoint != nil {
			tr.AddMetadata(MetadataListenEndpointKey, &listenEndpointsStruct)
		}
		if err := tr.Commit(); err != nil {
			return fmt.Errorf("unable to set metadata in node %s: %v", nodeName(node), err)
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
		info, ok := field.(*NetworkInfo)
		if !ok {
			logging.GetLogger().Warningf("Unable to convert %v (%T) to *NetworkInfo", field, field)
			return false

		}
		for k, v := range *info {
			if v.UpdatedAt < tt.UnixMilli() {
				delete(*info, k)
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
	// Register msgp MessagePackTime extension
	msgp.RegisterExtension(-1, func() msgp.Extension { return new(MessagePackTime) })

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

	p.nodeRevisionForceFlush = int64(config.GetInt("analyzer.topology.proccon.revision_flush"))

	go p.garbageCollector(interval, deleteDuration)

	return nil
}

// Stop garbageCollector
func (p *Probe) Stop() {
	p.GCdone <- true
}

// NewProbe initialize the probe with the parameters from the config file
func NewProbe(g *graph.Graph) (*Probe, error) {
	probe := &Probe{
		graph: g,
	}

	return probe, nil
}

// Register called at initialization to register metadata decoders
func Register() {
	graph.NodeMetadataDecoders[MetadataTCPConnKey] = MetadataDecoder
	graph.NodeMetadataDecoders[MetadataListenEndpointKey] = MetadataDecoder
}
