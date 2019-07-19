package oldcrypto

const (
	SignatureLength = 72
)

// Sig represents a 72 byte signature.
type Sig [SignatureLength]byte

// Signature is a compound type combining a signature with an account address.
type Signature struct {
	Account Address
	Sig     Sig
}

// Bytes gets the byte representation of the underlying signature.
func (s Signature) Bytes() []byte {
	out := make([]byte, 0)
	out = append(out, s.Account.Bytes()...)
	out = append(out, s.Sig[:]...)
	return out
}

// BytesToSig returns Signature with value sig.
//
// If sig is larger than SignatureLength, sig will be cropped from the left.
func BytesToSig(sig []byte) Signature {
	var s Sig
	s.SetBytes(sig[AddressLength:len(sig)])
	return Signature{
		Account: BytesToAddress(sig[0:AddressLength]),
		Sig:     s,
	}
}

// SetBytes sets the signature to the value of b.
// If b is larger than len(a) it will panic.
func (a *Sig) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-SignatureLength:]
	}
	copy(a[SignatureLength-len(b):], b)
}

// Sign signs a digest with the provided key pair.
func Sign(digest Hash, account Address, keyPair *KeyPair) *Signature {
	// TODO: implement real signatures
	return &Signature{
		Account: account,
		Sig:     Sig{},
	}
}
