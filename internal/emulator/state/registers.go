package state

import (
	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

// Registers is a map of register values.
type Registers map[crypto.Hash][]byte

func fullKey(controller, owner, key []byte) crypto.Hash {
	fullKey := append(controller, owner...)
	fullKey = append(fullKey, key...)

	return crypto.NewHash(fullKey)
}

func (r Registers) Set(controller, owner, key, value []byte) {
	r[fullKey(controller, owner, key)] = value
}

func (r Registers) Get(controller, owner, key []byte) (value []byte, exists bool) {
	value, exists = r[fullKey(controller, owner, key)]
	return value, exists
}

func (r Registers) MergeWith(registers Registers) {
	for key, value := range registers {
		r[key] = value
	}
}
