package mock

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/dchest/siphash"
)

func eventKey(engineID uint8, event interface{}) string {
	sip := siphash.New([]byte("testthenetwork" + fmt.Sprintf("%03d", engineID)))
	payload, _ := json.Marshal(event)
	hash := sip.Sum(payload)
	key := hex.EncodeToString(hash)
	return key
}
