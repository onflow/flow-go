package topology

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/onflow/flow-go/model/flow"
)

// FanoutFunc represents a function type that receiving total number of nodes
// in flow system, returns fanout of individual nodes.
type FanoutFunc func(size int) int

// LinearFanoutFunc guarantees full network connectivity in a deterministic way.
// Given system of `size` nodes, it returns `size+1/2`.
func LinearFanout(size int) int {
	fanout := math.Ceil(float64(size+1) / 2)
	return int(fanout)
}

// seedFromID generates a int64 seed from a flow.Identifier
func seedFromID(id flow.Identifier) (int64, error) {
	var seed int64
	buf := bytes.NewBuffer(id[:])
	if err := binary.Read(buf, binary.LittleEndian, &seed); err != nil {
		return -1, fmt.Errorf("could not read random bytes: %w", err)
	}
	return seed, nil
}
