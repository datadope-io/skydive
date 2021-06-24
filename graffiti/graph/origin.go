/*
 * Copyright (C) 2020 Sylvain Afchain.
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

package graph

import (
	"context"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/service"
	ws "github.com/skydive-project/skydive/graffiti/websocket"
)

// ClientOrigin return a string identifying a client using its service type and host id
func ClientOrigin(c ws.Speaker) string {
	return string(c.GetServiceType()) + "." + c.GetRemoteHost()
}

// DelSubGraphOfOrigin deletes all the nodes with a specified origin
func DelSubGraphOfOrigin(ctx context.Context, g *Graph, origin string) {
	g.DelNodes(ctx, Metadata{"Origin": origin})
}

func Origin(hostID string, kind service.Type) string {
	return fmt.Sprintf("%s.%s", kind, hostID)
}
