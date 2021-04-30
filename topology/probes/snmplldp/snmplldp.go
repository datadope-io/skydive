package snmplldp

import (
	"strconv"
	"strings"
	"time"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
)

type Device struct {
	host      string
	port      uint16
	community string
}

type Probe struct {
	graph   *graph.Graph
	log     logging.Logger
	quit    chan bool
	devices []Device
}

func (p *Probe) buildGraph() {
	for _, dev := range p.devices {
		lldp, err := getLLDPInfo(dev.host, dev.port, dev.community)
		if err != nil {
			p.log.Errorf("Obtaining LLDP info from %+v: %v", dev, err)
			continue
		}

		// Create local device node
		localNode, err := p.newNode(lldp.name, lldp.chassisID, lldp.desc)
		if err != nil {
			p.log.Errorf("Creating a node: %v", err)
			continue
		}

		// Create remote devices nodes and connect each one to the local
		// device with an edge
		for _, remote := range lldp.remoteTable {
			remoteNode, err := p.newNode(remote.name, remote.chassisID, remote.desc)
			if err != nil {
				p.log.Errorf("Creating a node: %v", err)
				continue
			}

			err = p.newEdge(localNode, remoteNode)
			if err != nil {
				p.log.Errorf("Creating an edge: %v", err)
				continue
			}
		}
	}
}

func (p *Probe) newNode(name string, chassisID string, desc string) (*graph.Node, error) {
	p.graph.Lock()
	defer p.graph.Unlock()

	filter := graph.NewElementFilter(filters.NewTermStringFilter("ChassisID", chassisID))
	node := p.graph.LookupFirstNode(filter)
	var err error = nil

	if node == nil {
		m := graph.Metadata{
			"Name":        name,
			"ChassisID":   chassisID,
			"Description": desc,
			"Type":        "host",
		}
		orig := "snmplldp." + p.graph.GetOrigin()
		nodeID := graph.GenID()

		node = graph.CreateNode(nodeID, m, graph.TimeUTC(), name, orig)

		err = p.graph.AddNode(node)
		if err == nil {
			p.log.Debugf("New Node: %s (%s)", name, chassisID)
		}
	}

	return node, err
}

func (p *Probe) newEdge(localNode *graph.Node, remoteNode *graph.Node) error {
	var err error = nil

	p.graph.Lock()
	defer p.graph.Unlock()

	if !p.graph.AreLinked(localNode, remoteNode, nil) {
		m := graph.Metadata{"RelationType": "node"}
		orig := "snmplldp." + p.graph.GetOrigin()
		edgeID := graph.GenID()

		edge := graph.CreateEdge(edgeID, localNode, remoteNode, m, graph.TimeUTC(), "", orig)

		err = p.graph.AddEdge(edge)
		if err == nil {
			p.log.Debugf("New Edge: %s - %s", localNode.Host, remoteNode.Host)
		}
	}

	return err
}

// Start the probe
func (p *Probe) Start() error {
	intervalConfig := config.GetString("analyzer.topology.snmplldp.interval")
	interval, err := time.ParseDuration(intervalConfig)
	if err != nil {
		p.log.Fatalf("Invalid analyzer.topology.snmplldp.interval value: %v", err)
	}

	devices := config.GetStringSlice("analyzer.topology.snmplldp.devices")
	for _, dev := range devices {
		connTriplet := strings.Split(dev, ":")
		if len(connTriplet) != 3 {
			p.log.Fatalf("Invalid analyzer.topology.snmplldp.devices value")
		}

		port, err := strconv.ParseUint(connTriplet[1], 10, 16)
		if err != nil {
			p.log.Fatalf("Invalid analyzer.topology.snmplldp.devices value: %v", err)
		}

		p.devices = append(p.devices,
			Device{
				host:      connTriplet[0],
				port:      uint16(port),
				community: connTriplet[2],
			})
	}

	// First execution
	p.buildGraph()

	// Start recurrent execution every interval
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-p.quit:
				return
			case <-ticker.C:
				p.buildGraph()
			}
		}
	}()

	return nil
}

// Stop the probe
func (p *Probe) Stop() {
	p.quit <- true
}

func NewProbe(g *graph.Graph) (*Probe, error) {
	return &Probe{
		graph: g,
		log:   logging.GetLogger(),
		quit:  make(chan bool),
	}, nil
}
