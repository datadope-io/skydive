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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/getter"
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

// processMetrics get an array of metrics, process each one and return the map with conn info
// which doesn't have a specific Software server.
// Return the number of metrics not proccessed or which returned errors
func (p *Probe) processMetrics(metrics []Metric) int {
	numberOfErrors := 0

	// Group metrics by host
	hostMetrics := map[string][]Metric{}
	for _, m := range metrics {
		host, ok := m.Tags["host"]
		if !ok {
			logging.GetLogger().Warningf("Metric without host tag: %+v. Ignored", m)
			numberOfErrors++
			continue
		}
		hostMetrics[host] = append(hostMetrics[host], m)
	}

	// For each host, process each metrics
	for host, metrics := range hostMetrics {
		numErr, err := p.processHost(host, metrics)
		if err != nil {
			logging.GetLogger().Errorf("Processing metrics for host %s: %v", host, err)
		}
		numberOfErrors += numErr
	}
	return numberOfErrors
}

// processHost handle a group of metrics belonging to the same server.
// Get or create a Server node from the host string and add/update metrics to that
// server node.
func (p *Probe) processHost(host string, metrics []Metric) (int, error) {
	logging.GetLogger().Debugf("processHost, host=%s, metrics=%+v", host, metrics)

	p.graph.Lock() // Avoid race condition createing twice the same server node
	defer p.graph.Unlock()

	// Get server node
	hostNode := p.graph.GetNode(getIdentifier(graph.Metadata{
		MetadataTypeKey: MetadataTypeServer,
		MetadataNameKey: host,
	}))

	var err error

	// Server node does not exists. Create it
	if hostNode == nil {
		logging.GetLogger().Debugf("Node not found with Metadata.Name '%s', creating it", host)

		logging.GetLogger().Debugf("newNode(Server, %v)", host)
		hostNode, err = p.newNode(host, graph.Metadata{
			MetadataNameKey: host,
			MetadataTypeKey: MetadataTypeServer,
		})
		if err != nil {
			return len(metrics), fmt.Errorf("creating %s server node: %v", host, err)
		}
	}

	numberOfErrors := p.processHostMetrics(hostNode, metrics)

	return numberOfErrors, nil
}

// processHostMetrics given a Server node and a list of metrics of that node, for each metric
// try to find a Software node with the same cmdline.
// If found, add the network info of the metric to that node and remove it from "others" node.
// If not found, append that network info to "others".
// Return the number of errors seen
func (p *Probe) processHostMetrics(serverNode *graph.Node, metrics []Metric) int {
	addToOthers := []Metric{}
	removeFromOthers := []Metric{}
	numberOfErrors := 0

	for _, metric := range metrics {
		// Get child Software nodes with matching cmdline
		childNodes := p.graph.LookupChildren(
			serverNode,
			graph.Metadata{MetadataTypeKey: MetadataTypeSoftware, MetadataCmdlineKey: metric.Tags["cmdline"]},
			graph.Metadata{MetadataRelationTypeKey: RelationTypeHasSoftware},
		)

		if len(childNodes) == 0 {
			logging.GetLogger().Debugf("Software node not found for Server node '%v' and cmdline '%s', storing in others", nodeName(serverNode), metric.Tags["cmdline"])
			// Accumulate changes to "others" to make only one change to the node
			addToOthers = append(addToOthers, metric)
			continue
		} else if len(childNodes) > 1 {
			// This should not happen
			logging.GetLogger().Errorf("Found more than one Software node for Server node '%v' and cmdline '%s': %+v. Ignoring", nodeName(serverNode), metric.Tags["cmdline"], childNodes)
			numberOfErrors++
			continue
		}

		// Software node already exists
		swNode := childNodes[0]

		// Append this metric to the list to be deleted from others
		removeFromOthers = append(removeFromOthers, metric)

		// Attach that network information to the software node
		err := p.addNetworkInfo(swNode, []Metric{metric})
		if err != nil {
			logging.GetLogger().Errorf("Not able to add network info to Software '%s' (host %+v)", nodeName(swNode), serverNode)
			numberOfErrors++
		}
	}

	err := p.handleOthers(serverNode, addToOthers, removeFromOthers)
	if err != nil {
		logging.GetLogger().Errorf("Not able to handle 'others' Software node: %v", err)
		numberOfErrors++
	}

	return numberOfErrors
}

// ServeHTTP receive HTTP POST requests from Telegraf nodes with processes data
func (p *Probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var metrics []Metric

	defer r.Body.Close()

	// Read the Telegraf packet
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get original client from X-Forwarded-For for error logging purposes
	source := ""
	if h := r.Header.Get("X-Forwarded-For"); h != "" {
		source = strings.Split(h, ",")[0]
	} else {
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

	numberOfErrors := p.processMetrics(metrics)

	w.Write([]byte(fmt.Sprintf("total:%d error:%d", len(metrics), numberOfErrors)))
}

// handleOthers given a Server and two list of metrics, one with the metrics to be
// added to the linked 'others' Software node, and the other list with the metrics to
// be removed from that 'others' node.
// Create the 'others' node if needed.
func (p *Probe) handleOthers(
	hostNode *graph.Node,
	metricsToBeAdded []Metric,
	metricsToBeDeleted []Metric,
) error {
	var otherNode *graph.Node
	var err error

	logging.GetLogger().Debugf("Handling %d adds and %d removes to other host of Server %s",
		len(metricsToBeAdded),
		len(metricsToBeDeleted),
		nodeName(hostNode),
	)

	othersNodes := p.graph.LookupChildren(
		hostNode,
		graph.Metadata{MetadataTypeKey: MetadataTypeSoftware, MetadataNameKey: OthersSoftwareNode},
		graph.Metadata{MetadataRelationTypeKey: RelationTypeHasSoftware},
	)

	if len(othersNodes) == 0 {
		if len(metricsToBeAdded) == 0 {
			// do nothing if there is no 'others' node and nothing to add
			return nil
		}

		// create 'others' node
		logging.GetLogger().Debugf("newNode(Software, others)")
		otherNode, err = p.newNode(hostNode.Host, graph.Metadata{
			MetadataNameKey: OthersSoftwareNode,
			MetadataTypeKey: MetadataTypeSoftware,
		})
		if err != nil {
			return fmt.Errorf("creating 'others' Software node for Server node '%+v': %v", hostNode, err)
		}

		// Create edge to link to the host
		logging.GetLogger().Debugf("newEdge(%v, %v, has_software)", hostNode, otherNode)
		_, err = p.newEdge(hostNode.Host, hostNode, otherNode, graph.Metadata{
			MetadataRelationTypeKey: RelationTypeHasSoftware,
		})
		if err != nil {
			return fmt.Errorf("linking 'others' Software node to host Server node '%+v': %v", hostNode, err)
		}
	} else if len(othersNodes) > 1 {
		// This should not happen
		return fmt.Errorf("Found more than one 'others' Software node for Server node '%+v'. Not removing metrics", hostNode)
	} else {
		otherNode = othersNodes[0]
	}

	err = p.removeFromOthers(otherNode, metricsToBeDeleted)
	if err != nil {
		return fmt.Errorf("removing connections from the other node of Server %s: %v", nodeName(hostNode), err)
	}

	// If removeFromOthers fail, this function will not be executed
	err = p.addNetworkInfo(otherNode, metricsToBeAdded)
	if err != nil {
		return fmt.Errorf("adding connections to the other node of Server %s: %v", nodeName(hostNode), err)
	}

	return nil
}

// removeFromOthers remove connections/listeners from the "others" software node
func (p *Probe) removeFromOthers(otherNode *graph.Node, metricsToBeDeleted []Metric) error {
	// Remove from connections/listeners lists in otherNode the values found in "metric"
	removeKeysFromList := func(field interface{}, metrics []string) (ret bool) {
		infoPtr, ok := field.(*NetworkInfo)
		if !ok {
			logging.GetLogger().Warningf("Unable to convert %v (%T) to *NetworkInfo", field, field)
			return false
		}
		info := *infoPtr

		for _, v := range metrics {
			if _, ok := info[v]; ok {
				delete(info, v)
				ret = true
			}
		}
		return ret
	}

	for _, metric := range metricsToBeDeleted {
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
	currentDataPtr, ok := field.(*NetworkInfo)
	if !ok {
		logging.GetLogger().Warningf("Unable to convert %v (%T) to *NetworkInfo", field, field)
		return false
	}
	currentData := *currentDataPtr

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
	logging.GetLogger().Debugf("addNetworkInfo, node:%s, metrics:%+v", nodeName(node), metrics)

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
		if errors.Is(err, getter.ErrFieldNotFound) {
			logging.GetLogger().Debugf("Unable to delete old network info because Metadata.%v does not exists", MetadataTCPConnKey)
			return nil
		}
		return fmt.Errorf("unable to delete old TCP connections: %v", err)
	}
	err = p.graph.UpdateMetadata(node, MetadataListenEndpointKey, removeOld)
	if err != nil {
		if errors.Is(err, getter.ErrFieldNotFound) {
			logging.GetLogger().Debugf("Unable to delete old network info because Metadata.%v does not exists", MetadataListenEndpointKey)
			return nil
		}
		return fmt.Errorf("unable to delete old listen endpoints: %v", err)
	}
	return nil
}

// cleanSoftwareNodes delete old data from type Software nodes
// "now" is the current time (parametrized to be able to test this function)
// "expiredConnection" is used to determine if connections/listeners are too old
func (p *Probe) cleanSoftwareNodes(expiredConnectionThreshold time.Time) {
	// Lock the graph while cleaning.
	// Once we get the nodes, we are using that info to update its metadata.
	// We have to avoid changes in those hosts while running this cleaner.
	p.graph.Lock()
	defer p.graph.Unlock()

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

// newNode creates and inserts a new node in the graph, using known ID, setting the
// Origin field to "proccon"+g.Origin and the Host field to the param "host"
func (p *Probe) newNode(host string, m graph.Metadata) (*graph.Node, error) {
	i := getIdentifier(m)
	n := graph.CreateNode(i, m, graph.TimeUTC(), host, ProcconOriginName+p.graph.GetOrigin())

	if err := p.graph.AddNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

// getIdentifier generate a node identifier. Server nodes will get always the same identifier, generated from the values of the metadata. The rets of nodes will get a random one.
// Server node identifier is generated with: {Metadata.Type}__{Metadata.Name}
func getIdentifier(metadata graph.Metadata) graph.Identifier {
	name, err := metadata.GetField(MetadataNameKey)
	if err != nil {
		panic("node metadata should always have 'Name'")
	}
	mType, err := metadata.GetField(MetadataTypeKey)
	if err != nil {
		panic("node metadata should always have 'Type'")
	}

	// Nodes with fixed identifiers
	switch mType {
	case MetadataTypeServer:
		return graph.Identifier(mType.(string) + "__" + name.(string))
	}

	return graph.GenID()
}

// Start initilizates the proccon probe, starting a web server to receive data and the garbage collector to delete old info
func (p *Probe) Start() error {
	// Register msgp MessagePackTime extension
	msgp.RegisterExtension(-1, func() msgp.Extension { return new(MessagePackTime) })

	listenEndpoint := config.GetString("analyzer.topology.proccon.listen")
	http.Handle("/", p)
	go http.ListenAndServe(listenEndpoint, nil)
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
