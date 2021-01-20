package topology

import (
	"bytes"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/logging"
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
	log          zerolog.Logger
	top          network.Topology  // instance of underlying topology.
	cachedFanout flow.IdentityList // most recently generated fanout.
	cachedError  error             // most recently generated error by invoking underlying topology.
	fingerprint  flow.Identifier   // input IdentityList unique fingerprint for cached fanout.
}

//NewCache creates and returns a topology Cache given an instance of topology implementation.
func NewCache(log zerolog.Logger, top network.Topology) *Cache {
	return &Cache{
		log:          log.With().Str("component", "topology_cache").Logger(),
		top:          top,
		cachedFanout: nil,
		cachedError:  nil,
		fingerprint:  flow.Identifier{},
	}
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
	inputFingerprint := ids.Fingerprint()
	if !bytes.Equal(inputFingerprint[:], c.fingerprint[:]) {
		// updates current cache with a new topology
		c.cachedFanout, c.cachedError = c.top.GenerateFanout(ids)
		c.fingerprint = inputFingerprint

		c.log.Debug().
			Hex("cached_fingerprint", logging.ID(c.fingerprint)).
			Hex("new_fingerprint", logging.ID(inputFingerprint)).
			Int("input_size", len(ids)).
			Msg("topology cache updated")
	}

	return c.cachedFanout, c.cachedError
}
