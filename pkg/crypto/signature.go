package crypto

// Signature is a compound type combining a signature with an account address.
type Signature struct {
	Account Address
	Sig     []byte
}

func (s Signature) Bytes() []byte {
	out := make([]byte, 0)
	out = append(out, s.Account.Bytes()...)
	out = append(out, s.Sig[:]...)
	return out
}

func BytesToSig(sig []byte) Signature {
	return Signature{
		Account: BytesToAddress(sig[0:AddressLength]),
		Sig:     sig[AddressLength:len(sig)],
	}
}

// Sign signs a digest with the provided key pair.
func Sign(digest Hash, account Address, keyPair *KeyPair) *Signature {
	// TODO: implement real signatures
	return &Signature{
		Account: account,
		Sig:     nil,
	}
}
