package topology

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// Cache provides caching the most recently generated topology.
// It implements the same GenerateFanout as a normal topology, so can easily replace any topology implementation.
// As long as the input IdentityList to it is the same the cached topology is returned without invoking the
// underlying GenerateFanout.
// This is vital to provide a deterministic topology interface, as by its nature, the Topology interface does not
// guarantee a deterministic behavior.
//
// Note: as a convention with other topology implementations, Cache is not concurrency-safe, and should be invoked
// in a concurrency safe way, i.e., the caller should lock for it.
type Cache struct {
	top         network.Topology
	cachedList  flow.IdentityList
	cachedError error
	fingerprint flow.Identifier
}

// GenerateFanout receives IdentityList of entire network and constructs the fanout IdentityList
// of this instance.
// It caches the most recently generated fanout list, so as long as the input list is the same, it returns
// the same output. It invalidates and updates its internal cache the first time input list changes.
// A node directly communicates with its fanout IdentityList on epidemic dissemination
// of the messages (i.e., publish and multicast).
// Independent invocations of GenerateFanout on different nodes collaboratively must construct a cohesive
// connected graph of nodes that enables them talking to each other.
func (c *Cache) GenerateFanout(ids flow.IdentityList) (flow.IdentityList, error) {
	fp := ids.Fingerprint()
	if !bytes.Equal(fp[:], c.fingerprint[:]) {
		// updates current cache with a new topology
		c.cachedList, c.cachedError = c.top.GenerateFanout(ids)
	}

	return c.cachedList, c.cachedError
}
