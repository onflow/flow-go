package types

// AccountSignature is a compound type that includes a signature, a public key
// and an account address.
type AccountSignature struct {
	Account   Address
	PublicKey []byte
	Signature []byte
}

// Bytes returns the bytes representation of the account signature.
func (a AccountSignature) Bytes() []byte {
	b := append(a.Account.Bytes(), a.PublicKey...)
	return append(b, a.Signature...)
}
