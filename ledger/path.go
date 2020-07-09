package ledger

import (
	"bytes"
	"fmt"
)

// Path captures storage path of a payload;
// where we store a payload in the ledger
type Path []byte

func (p Path) String() string {
	str := ""
	for _, i := range p {
		str += fmt.Sprintf("%08b", i)
	}
	if len(str) > 16 {
		str = str[0:8] + "..." + str[len(str)-8:]
	}
	return str
}

// Equals compares this path to another path
func (p Path) Equals(o Path) bool {
	if o == nil {
		return false
	}
	return bytes.Equal([]byte(p), []byte(o))
}
