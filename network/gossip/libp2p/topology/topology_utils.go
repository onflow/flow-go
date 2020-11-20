package topology

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
)

// intSeedFromID generates a int64 seed from a flow.Identifier
func intSeedFromID(id flow.Identifier) (int64, error) {
	var seed int64
	buf := bytes.NewBuffer(id[:])
	if err := binary.Read(buf, binary.LittleEndian, &seed); err != nil {
		return -1, fmt.Errorf("could not read random bytes: %w", err)
	}
	return seed, nil
}

// intSeedFromID generates a int64 seed from a flow.Identifier
func byteSeedFromID(id flow.Identifier) ([]byte, error) {
	h, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return nil, fmt.Errorf("could not generate hasher: %w", err)
	}

	encodedId, err := encoding.DefaultEncoder.Encode(id)
	if err != nil {
		return nil, fmt.Errorf("could not encode id: %w", err)
	}

	return h.ComputeHash(encodedId), nil
}
