package types

import "github.com/dapperlabs/bamboo-node/pkg/crypto"

// Account represents an account on the Bamboo network.
//
// An account can be an externally owned account or a contract account with code.
type Account struct {
	Address    Address
	Balance    uint64
	Code       []byte
	PublicKeys [][]byte
}

type AccountKey struct {
	Account Address
	Key     crypto.PrKey
}
