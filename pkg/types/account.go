package types

// Account represents an account on the Bamboo network.
//
// An account can be an externally owned account or a contract account with code.
type Account struct {
	Address    Address
	Balance    uint64
	Code       []byte
	PublicKeys [][]byte
}
