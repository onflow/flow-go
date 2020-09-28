package stub

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
)

// eventKey generates a unique fingerprint for the tuple of (sender, event, type of event, channelID)
func eventKey(from flow.Identifier, channelID string, event interface{}) (string, error) {
	tag, err := encoding.DefaultEncoder.Encode([]byte(fmt.Sprintf("testthenetwork %s %T", channelID, event)))
	if err != nil {
		return "", fmt.Errorf("could not encode the tag: %w", err)
	}

	payload, err := encoding.DefaultEncoder.Encode(event)
	if err != nil {
		return "", fmt.Errorf("could not encode event: %w", err)
	}

	sender, err := encoding.DefaultEncoder.Encode(from)
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
