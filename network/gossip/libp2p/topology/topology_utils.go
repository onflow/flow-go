package topology

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// seedFromID generates a int64 seed from a flow.Identifier
func seedFromID(id flow.Identifier) (int64, error) {
	var seed int64
	buf := bytes.NewBuffer(id[:])
	if err := binary.Read(buf, binary.LittleEndian, &seed); err != nil {
		return -1, fmt.Errorf("could not read random bytes: %w", err)
	}
	return seed, nil
}
