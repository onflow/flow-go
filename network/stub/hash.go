package stub

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
)

// eventKey generates a unique fingerprint for the tuple of (event, type of event, channelID)
func eventKey(channelID uint8, event interface{}) (string, error) {
	tag, err := encoding.DefaultEncoder.Encode([]byte(fmt.Sprintf("testthenetwork %03d %T", channelID, event)))
	if err != nil {
		return "", fmt.Errorf("could not encode the tag: %w", err)
	}

	payload, err := encoding.DefaultEncoder.Encode(event)
	if err != nil {
		return "", fmt.Errorf("could not encode event: %w", err)
	}

	hasher, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return "", fmt.Errorf("could not create a hasher: %w", err)
	}

	_, err = hasher.Write(tag)
	if err != nil {
		return "", fmt.Errorf("could not write to hasher: %w", err)
	}

	_, err = hasher.Write(payload)
	if err != nil {
		return "", fmt.Errorf("could not write to hasher: %w", err)
	}

	hash := hasher.SumHash()
	key := hex.EncodeToString(hash)
	return key, nil

}
