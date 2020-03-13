package stub

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
)

func eventKey(channelID uint8, event interface{}) string {
	tag := []byte(fmt.Sprintf("testthenetwork %03d %T", channelID, event))
	payload, _ := encoding.DefaultEncoder.Encode(tag)
	hasher, _ := crypto.NewHasher(crypto.SHA2_256)
	hash := hasher.ComputeHash(append(tag, payload...))
	key := hex.EncodeToString(hash)
	return key
}
