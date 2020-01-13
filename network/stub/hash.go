package stub

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/dchest/siphash"
)

func eventKey(channelID uint8, event interface{}) string {
	sip := siphash.New([]byte("testthenetwork" + fmt.Sprintf("%03d", channelID)))
	payload, _ := json.Marshal(event)
	hash := sip.Sum(payload)
	key := hex.EncodeToString(hash)
	return key
}
