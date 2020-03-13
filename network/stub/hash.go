package stub

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
)

func eventKey(channelID uint8, event interface{}) string {
	tag, _ := encoding.DefaultEncoder.Encode([]byte(fmt.Sprintf("testthenetwork %03d %T", channelID, event)))
	payload, _ := encoding.DefaultEncoder.Encode(event)
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)

	hasher.Write(tag)
	hasher.Write(payload)
	hash := hasher.SumHash()

	key := hex.EncodeToString(hash)
	return key
}
