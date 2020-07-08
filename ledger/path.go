package ledger

import (
	"bytes"
	"encoding/hex"
)

// Path captures storage path of a payload;
// where we store this payload in the ledger
type Path []byte

func (p Path) String() string {
	return hex.EncodeToString(p)
}

// Equals compares this path to another path
func (p Path) Equals(o Path) bool {
	if o == nil {
		return false
	}
	return bytes.Equal([]byte(p), []byte(o))
}
