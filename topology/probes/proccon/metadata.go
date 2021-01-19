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

package proccon

import (
	"encoding/json"
	"fmt"

	"github.com/skydive-project/skydive/graffiti/getter"
)

// easyjson:json
// gendecoder
type NetworkInfo map[string]ProcInfo

// ProcInfo store info associated to each TCP connection or listen endpoint.
// It is used to delete old conections and be able to tell which connections
// are seen several times
// easyjson:json
// gendecoder
type ProcInfo struct {
	CreatedAt int64
	UpdatedAt int64
	Revision  int64
}

// MetadataDecoder implements a json message raw decoder
func MetadataDecoder(raw json.RawMessage) (getter.Getter, error) {
	var m NetworkInfo
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unable to unmarshal proccon metadata %s: %s", string(raw), err)
	}

	return &m, nil
}
