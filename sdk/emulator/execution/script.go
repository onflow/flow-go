package execution

import "github.com/dapperlabs/flow-go/pkg/crypto"

func ScriptHash(script []byte) crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	return hasher.ComputeHash(script)
}
