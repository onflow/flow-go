package rlp

type transactionCanonicalWrapper struct {
	Script             []byte
	ReferenceBlockHash []byte
	Nonce              uint64
	ComputeLimit       uint64
	PayerAccount       []byte
	ScriptAccounts     [][]byte
}

// accountPublicKeyWrapper is used for encoding and decoding.
type accountPublicKeyWrapper struct {
	PublicKey []byte
	SignAlgo  uint
	HashAlgo  uint
	Weight    uint
}
