package types

// IndexedPubKey is a struct contains the public key and its signer index
// within its consensus cluster.
type IndexedPubKey struct {
	PubKey      PubKey
	SignerIndex SignerIndex
}
