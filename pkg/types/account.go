package types

// Account represents an account on the Flow network.
//
// An account can be an externally owned account or a contract account with code.
type Account struct {
	Address Address
	Balance uint64
	Code    []byte
	Keys    []AccountKey
}

// AccountKey is a public key associated with an account.
//
// An account key contains the public key encoded as bytes and a key weight.
type AccountKey struct {
	PublicKey []byte
	Weight    uint
}
