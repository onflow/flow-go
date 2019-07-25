package types

// AccountSignature includes a signature and the account address that generated it.
type AccountSignature struct {
	Account   Address
	Signature []byte
}

// Bytes returns the bytes representation of the account signature.
func (a AccountSignature) Bytes() []byte {
	return append(a.Account.Bytes(), a.Signature...)
}
