//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder --package github.com/skydive-project/skydive/topology/probes/procs
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package procs implement a probe which discover software and register
// new nodes linked to the server. It also register the connection data
// (listen ports and open connections)
package procs

import (
	"context"
	json "encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
	"github.com/skydive-project/skydive/graffiti/getter"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes"
)

const (
	// Software group to store all the connection info of unknown processes
	othersSW     = "others"
	softwareType = "Software"
)

// ProbeHandler describes a memory probe handler.
// It implements the probe.Handler interface.
type ProbeHandler struct {
	Ctx probes.Context
	*graph.EventHandler
	software []SWGroup
	interval time.Duration
	// procGroups used to store last seen software groups
	procGroups map[string]NodeData
	// deleteThreshold is the duration after a SWGroup without new data is deleted
	deleteThreshold time.Duration
}

// NodeData contains connections and listeners for all the process mathing a regexp.
// Is the data sent to the analyzer as Metadata
// easyjson:json
// gendecoder
type NodeData struct {
	Name string
	// Software "class", defined in the config
	SubType   string
	TCPListen []Endpoint
	TCPConn   []Endpoint
	// LastSeen stores the last time a proc of the group was seen on the OS
	LastSeen time.Time
}

// SWGroup is the expressións used to map OS processes to know software
type SWGroup struct {
	Name   string
	Class  string
	Regexp *regexp.Regexp
	// Ignore avoids adding this software to the NodeData
	Ignore bool
}

// SoftwareConfig is the dict used in the config to define software
type SoftwareConfig struct {
	Name   string
	Class  string
	Regexp string
	Ignore bool
}

// Endpoint define an local or remote IP endpoint
// easyjson:json
// gendecoder
type Endpoint struct {
	// Proc have the name of the process owning this endpoint. Useful to know the owner in the othersSW group
	Proc string
	IP   string
	Port uint32
}

// IPs stores a list of IPs divided by version
type IPs struct {
	v4 []string
	v6 []string
}

// appendConnections adds endpoints to the proc basend on the connection info and local ips
// The main purpose of procName is to recognize the origin proc for the othersSW group
func (p *ProbeHandler) appendConnections(proc *NodeData, procName string, conn []net.ConnectionStat, localIps IPs) error {
	// Only process LISTEN and ESTABLISHED connections.
	// Get listen ports to avoid storing established connections to that listen port
	listenPorts := map[uint32]interface{}{}
	for _, c := range conn {
		if c.Status == "LISTEN" {
			listenPorts[c.Laddr.Port] = nil
		}
	}

	for _, c := range conn {
		if c.Status == "LISTEN" {
			// TODO si metemos las IPs internas (por ejemplo, la de la interfaz de docker)
			// podemos tener problemas por procesos conectando a estas IPs y no saber
			// a que host pertenece
			if c.Laddr.IP == "127.0.0.1" || c.Laddr.IP == "::1" {
				// Ignore local listeners
				continue
			} else if c.Laddr.IP == "0.0.0.0" {
				for _, ip := range localIps.v4 {
					proc.TCPListen = append(proc.TCPListen, Endpoint{procName, ip, c.Laddr.Port})
				}
			} else if c.Laddr.IP == "::" {
				// IPv6 "::" listen in both, IPv4 and IPv6
				for _, ip := range localIps.v4 {
					proc.TCPListen = append(proc.TCPListen, Endpoint{procName, ip, c.Laddr.Port})
				}
				for _, ip := range localIps.v6 {
					proc.TCPListen = append(proc.TCPListen, Endpoint{procName, ip, c.Laddr.Port})
				}
			} else {
				proc.TCPListen = append(proc.TCPListen, Endpoint{procName, c.Laddr.IP, c.Laddr.Port})
			}
		} else if c.Status == "ESTABLISHED" {
			// Ignore local connections
			if c.Laddr.IP == "127.0.0.1" || c.Laddr.IP == "::1" {
				continue
			}
			// Do not store connections made to this server
			if _, exists := listenPorts[c.Laddr.Port]; !exists {
				proc.TCPConn = append(proc.TCPConn, Endpoint{procName, c.Raddr.IP, c.Raddr.Port})
			}
		}
	}

	return nil
}

// Do adds memory to the node metadata, updating periodically
// Implements probes.handler
func (p *ProbeHandler) Do(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	// start a goroutine in order to update the graph
	go func() {
		defer wg.Done()

		// update the graph each five seconds
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// got a message on the quit chan
				return
			case <-ticker.C:
				// TODO: Subscribe to new connections instead of polling?
				// ../socketinfo/socket_info_ebpf.go
				//
				// TODO instead of get procs and then connections.
				// Get connections and then filter procs?

				// Get the list of current procs
				// TODO how to handle containers? Here we got local process + procs insider containers
				procs, err := process.Processes()
				if err != nil {
					p.Ctx.Logger.Error("getting process list")
				}

				currentGroups := map[string]NodeData{}

				// Get hosts IPs to create endpoints when listeners listens in all interfaces
				localIps, err := p.getLocalIps()
				if err != nil {
					p.Ctx.Logger.Errorf("getting local interfaces: %v", err)
				}

			L1:
				for _, proc := range procs {
					// Ignore errors, because it the process has already finished, it will fail also in the next step, getting cmdline
					procName, _ := proc.Name()

					cmdline, err := proc.Cmdline()
					if err != nil {
						// Could return an error if the process has already finished
						p.Ctx.Logger.Debugf("getting the cmdline for proc %v: %v", proc.String(), err)
						// Proc ignored because we cannot match its cmdline against the regexps
						continue
					}

					// Agent should be run as root to get connection info of all procs
					conn, err := proc.Connections()
					if err != nil {
						p.Ctx.Logger.Errorf("getting connections for proc %v: %v", proc.String(), err)
					}

					// Check if the process match one of the known software
					// Stops at the first match
					for _, sw := range p.software {
						// TODO does this work with jboss? Or we have to take all the children once we discover the parent? Which proc have the listeners attached?
						if sw.Regexp.MatchString(cmdline) {
							if sw.Ignore {
								continue L1
							}

							// If match, create/update an entry in currentGroups
							swGroup, exists := currentGroups[sw.Name]
							if !exists {
								swGroup.Name = sw.Name
								swGroup.SubType = sw.Class
							}

							swGroup.LastSeen = time.Now()

							err = p.appendConnections(&swGroup, procName, conn, localIps)
							if err != nil {
								p.Ctx.Logger.Errorf("adding connections to proc '%v': %v", sw.Name, err)
							}

							currentGroups[sw.Name] = swGroup
							continue L1
						}
					}

					// If the process is not known, but it have listeners or active connections, add to the
					// general node othersSW
					swGroup, exists := currentGroups[othersSW]
					if !exists {
						swGroup.Name = othersSW
					}
					swGroup.LastSeen = time.Now()

					err = p.appendConnections(&swGroup, procName, conn, localIps)
					if err != nil {
						p.Ctx.Logger.Errorf("adding connections to group '%s': %v", othersSW, err)
					}

					// Only add the othersSW group if it has connection data
					if len(swGroup.TCPConn) > 0 || len(swGroup.TCPListen) > 0 {
						currentGroups[othersSW] = swGroup
					}
				}

				// Add procs to the graph
				for _, proc := range currentGroups {
					err := p.addProc(proc)
					if err != nil {
						p.Ctx.Logger.Errorf("unable to add proc '%s' node to the graph: %s", proc.Name, err)
					}
				}

				// Delete old nodes (nodes in p.procGroups not in currentGroups)
				// Only delete after after some time, to avoid flapping
				for k, v := range p.procGroups {
					if _, exists := currentGroups[k]; !exists && time.Since(v.LastSeen) > p.deleteThreshold {
						p.Ctx.Logger.Debugf("delete node '%v'. lastseen=%v", v.Name, v.LastSeen)
						p.Ctx.Graph.Lock()
						p.Ctx.Graph.DelNodes(graph.Metadata{
							"Name":    v.Name,
							"SubType": v.SubType,
							"Type":    softwareType,
						})
						// TODO what parameter should be passed?
						//p.NotifyEvent(graph.NodeDeleted, node)
						p.Ctx.Graph.Unlock()
					}
				}

				p.procGroups = currentGroups
			}
		}
	}()

	return nil
}

// getLocalIps get host IPs
func (p *ProbeHandler) getLocalIps() (IPs, error) {
	localIps := IPs{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return IPs{}, fmt.Errorf("getting local interfaces: %v", err)
	}
	for _, i := range ifaces {
		for _, addr := range i.Addrs {
			// Remove network mask
			ip := strings.Split(addr.Addr, "/")[0]
			if strings.Contains(ip, ":") {
				// Ignore localhost
				if ip != "::1" {
					localIps.v6 = append(localIps.v6, ip)
				}
			} else {
				// Ignore localhost
				if ip != "127.0.0.1" {
					localIps.v4 = append(localIps.v4, ip)
				}
			}
		}
	}

	return localIps, nil
}

// addProc create a new node owned by the root node with the process
// name and connection info, or update metadata if it already exists
func (p *ProbeHandler) addProc(proc NodeData) error {
	// lock the graph for modification
	// TODO lock the graph the whole function?
	p.Ctx.Graph.Lock()
	// release the graph lock
	defer p.Ctx.Graph.Unlock()

	var node *graph.Node
	node = p.Ctx.Graph.LookupFirstChild(p.Ctx.RootNode, graph.Metadata{
		"Name": proc.Name,
		"Type": softwareType,
	})

	// Normalize Proc to metadata
	metadata, ok := graph.NormalizeValue(proc).(map[string]interface{})
	if !ok {
		return fmt.Errorf("proc metadata has wrong format: %v", proc)
	}

	// Tag all software nodes with Type softwareType
	metadata["Type"] = softwareType

	if node == nil {
		id := graph.GenID()

		var err error
		p.Ctx.Logger.Debug("creating new node", "metadata", metadata, "id", id)

		node, err = p.Ctx.Graph.NewNode(id, metadata)
		if err != nil {
			return fmt.Errorf("unable to add the new node to the graph: %s", err)
		}
		p.NotifyEvent(graph.NodeAdded, node)
	} else {
		// Update metadata
		// TODO is this operation costly? Maybe is better to store old state and just send changes?
		p.Ctx.Graph.SetMetadata(node, metadata)
		p.NotifyEvent(graph.NodeUpdated, node)
	}

	if !topology.HaveOwnershipLink(p.Ctx.Graph, p.Ctx.RootNode, node) {
		topology.AddOwnershipLink(p.Ctx.Graph, p.Ctx.RootNode, node, nil)
	}

	return nil
}

// NewProbe returns a new topology memory probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probe.Handler, error) {
	interval := ctx.Config.GetInt("agent.topology.procs.interval")
	if interval == 0 {
		return nil, fmt.Errorf("agent.topology.procs.interval should be defined and not 0")
	}

	deleteCount := ctx.Config.GetInt("agent.topology.procs.delete_count")

	configSW := ctx.Config.Get("agent.topology.procs.software")
	softwareConfig := []SoftwareConfig{}
	if err := mapstructure.Decode(configSW, &softwareConfig); err != nil {
		return nil, fmt.Errorf("unable to read agent.topology.procs.software: %s", err)
	}

	// Convert and verify config regexp expressions to regexp values
	software := []SWGroup{}
	for _, v := range softwareConfig {
		r, err := regexp.Compile(v.Regexp)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp for software %v: %v", v.Name, err)
		}

		software = append(software, SWGroup{
			Name:   v.Name,
			Class:  v.Class,
			Regexp: r,
			Ignore: v.Ignore,
		})
	}

	p := &ProbeHandler{
		Ctx:             ctx,
		EventHandler:    graph.NewEventHandler(100),
		software:        software,
		interval:        time.Duration(interval) * time.Second,
		deleteThreshold: time.Duration(interval) * time.Duration(deleteCount) * time.Second,
	}

	return probes.NewProbeWrapper(p), nil
}

// MetadataDecoder implements a json message raw decoder. It is used
// by the analyzer to decode properly the JSON message with the proper types.
// the Getter interface functions will be generated by the gendecoder generator.
// See the first line and the tag at the struct declaration.
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var proc NodeData
	if err := json.Unmarshal(raw, &proc); err != nil {
		return nil, fmt.Errorf("unable to unmarshal proc data '%s': %s", string(raw), err)
	}

	return &proc, nil
}

// Register registers metadata decoders
func Register() {
	// register the MemoryDecoder so that the graph knows how to decode the
	// Memory metadata field.
	// TODO what to implement here?
	graph.NodeMetadataDecoders["Procs"] = MetadataDecoder
}
