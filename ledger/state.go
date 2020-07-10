package ledger

import (
	"bytes"
	"encoding/hex"
)

// StateCommitment captures a commitment to an specific state of the ledger
type StateCommitment []byte

func (sc StateCommitment) String() string {
	return hex.EncodeToString(sc)
}

// Equals compares the state commitment to another one
func (sc StateCommitment) Equals(o StateCommitment) bool {
	if o == nil {
		return false
	}
	return bytes.Equal(sc, o)
}
