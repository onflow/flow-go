package types

import (
	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

// Registers is a map of register values.
type Registers map[crypto.Hash][]byte
