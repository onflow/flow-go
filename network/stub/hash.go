package stub

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding/json"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

// eventKey generates a unique fingerprint for the tuple of (sender, event, type of event, channel)
func eventKey(from flow.Identifier, channel network.Channel, event interface{}) (string, error) {
	marshaler := json.NewMarshaler()

	tag, err := marshaler.Marshal([]byte(fmt.Sprintf("testthenetwork %s %T", channel, event)))
	if err != nil {
		return "", fmt.Errorf("could not encode the tag: %w", err)
	}

	payload, err := marshaler.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("could not encode event: %w", err)
	}

	sender, err := marshaler.Marshal(from)
	if err != nil {
		return "", fmt.Errorf("could not encode sender: %w", err)
	}

	hasher := hash.NewSHA3_256()
	_, err = hasher.Write(tag)
	if err != nil {
		return "", fmt.Errorf("could not write to hasher: %w", err)
	}

	_, err = hasher.Write(payload)
	if err != nil {
		return "", fmt.Errorf("could not write to hasher: %w", err)
	}

	_, err = hasher.Write(sender)
	if err != nil {
		return "", fmt.Errorf("could not write to hasher: %w", err)
	}

	hash := hasher.SumHash()
	key := hex.EncodeToString(hash)
	return key, nil

}
