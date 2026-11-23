/*
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

package graph

import (
	"errors"
	"fmt"
	"sync"
)

// ErrBackendQuery is wrapped by the error returned when a request to the
// persistent backend failed while running a query
var ErrBackendQuery = errors.New("persistent backend query failed")

// QueryErrors collects the errors of the persistent backend requests made
// while running a query. The backend methods do not return errors, so a
// failed request used to look like an empty result. It is safe for concurrent
// use, and a nil *QueryErrors ignores the errors.
type QueryErrors struct {
	lock  sync.Mutex
	count int
	first error
}

// Add records a failed request
func (q *QueryErrors) Add(err error) {
	if q == nil || err == nil {
		return
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	q.count++
	if q.first == nil {
		q.first = err
	}
}

// Err returns nil if no request failed, otherwise an error wrapping
// ErrBackendQuery with the number of failed requests and the first error
func (q *QueryErrors) Err() error {
	if q == nil {
		return nil
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	if q.count == 0 {
		return nil
	}
	return fmt.Errorf("%w: %d request(s) failed, first error: %v", ErrBackendQuery, q.count, q.first)
}
