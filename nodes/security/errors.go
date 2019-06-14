package security

import (
	"fmt"

	"github.com/dapperlabs/bamboo-emulator/crypto"
)

type DuplicateBlockError struct {
	blockHash crypto.Hash
}

func (e *DuplicateBlockError) Error() string {
	return fmt.Sprintf("Block with hash %s already exists", e.blockHash)
}
