package execution

import "github.com/dapperlabs/flow-go/crypto"

// ScriptHash computes the content hash of a Cadence script.
func ScriptHash(script []byte) crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	return hasher.ComputeHash(script)
}
