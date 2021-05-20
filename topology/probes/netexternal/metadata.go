//go:generate go run github.com/skydive-project/skydive/graffiti/gendecoder -package github.com/skydive-project/skydive/topology/probes/proccon
//go:generate go run github.com/mailru/easyjson/easyjson $GOFILE

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

package netexternal

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/skydive-project/skydive/graffiti/getter"
)

// RouteTable is a list of routes.
// easyjson:json
// gendecoder
type RouteTable []Route

// Route represent a single ip route. This representation will be stored in the interfaces
// nodes medatada
// easyjson:json
// gendecoder
// TODO esto aquí hace el gencoder. Debería ir en el paquete netexternal/
// Mover todo el paquete graphql a netexternal?
type Route struct {
	// Name optional name for this route
	Name string
	// Network where this route apply
	Network net.IPNet
	// NextHop IP address where the traffic should be sent
	NextHop net.IP
	// DeviceNextHop optinal way to define the next hop
	DeviceNextHop string
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var r RouteTable
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("unable to unmarshal proccon metadata %s: %s", string(raw), err)
	}

	return &r, nil
}
