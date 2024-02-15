package convert

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// SnapshotToBytes converts a `protocol.Snapshot` to bytes, encoded as JSON
func SnapshotToBytes(snapshot protocol.Snapshot) ([]byte, error) {
	serializable, err := inmem.FromSnapshot(snapshot)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(serializable.Encodable())
	if err != nil {
		return nil, err
	}

	return data, nil
}

// BytesToInmemSnapshot converts an array of bytes to `inmem.Snapshot`
func BytesToInmemSnapshot(bytes []byte) (*inmem.Snapshot, error) {
	var encodable inmem.EncodableSnapshot
	err := json.Unmarshal(bytes, &encodable)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal decoded snapshot: %w", err)
	}

	return inmem.SnapshotFromEncodable(encodable), nil
}
