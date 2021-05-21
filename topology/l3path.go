/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package topology

import (
	"errors"
	"fmt"
	"net"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/netexternal"
)

// ErrNoL3Path is returned when no next hop could be determined
var ErrNoL3Path = errors.New("no next hop")

// GetL3Path returns all the nodes to reach a specified IP
// This find the first subnet matching.
// TODO: cisco coge la primera ruta definida o la más específica.
// Si definimos varias rutas es como hacer HA, como gestionamos eso?
func GetL3Path(node *graph.Node, ip net.IP, hops map[int]*graph.Node) error {
	logging.GetLogger().Debugf("GetL3Path , iface=%+v, ip=%v", node, ip)
	tables, err := node.GetField(netexternal.MetadataRoutingTableKey)
	if err != nil {
		return err
	}

	rts, ok := tables.(netexternal.RouteTable)
	if !ok {
		return fmt.Errorf("invalid RouteTable metadata: %+v", tables)
	}

	var defaultRouteIP net.IP
	var defaultRouteDev string
	var routeIP net.IP
	var routeDev string

	for _, t := range rts {
		ipnet := net.IPNet(t.Network)
		// If the route is the default one, store it in case we did not found any specific
		if t.Network.IsDefaultRoute() {
			defaultRouteIP = t.NextHop
			defaultRouteDev = t.DeviceNextHop
		} else if ipnet.Contains(ip) {
			// If the destination IP is contained in the network, use this hop
			// Find the node with that IP and continue the path
			// TODO como saber quien es el siguiente salto?
			// Si busco simplemente por una IP puedo caer en otra VRF.
			// Buscar la IP en el mismo device o misma network?
			routeIP = t.NextHop
			routeDev = t.DeviceNextHop
			break
		}
	}

	// No specific route found, use the default
	if routeIP == nil && routeDev == "" {
		if defaultRouteIP == nil && defaultRouteDev == "" {
			// No route found
			return ErrNoL3Path
		}
		routeIP = defaultRouteIP
		routeDev = defaultRouteDev
	}

	logging.GetLogger().Debugf("buscando siguiente nodo, iface=%+v, ip=%v, routeIP=%v, routeDev=%v", node, ip, routeIP, routeDev)
	/*
		ipNodeFilter := graph.NewElementFilter(
			filters.NewAndFilter(
				filters.NewTermStringFilter(MetadataTypeKey, TypeIP),
				filters.NewTermStringFilter(MetadataIPKey, ip.String()),
		))
		hops[len(hops)] = nextHost
		err := GetL3Path(nextHost, ip, hops)
	*/
	return nil
}
