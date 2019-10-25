package rlp

type transactionWrapper struct {
	Script             []byte
	ReferenceBlockHash []byte
	Nonce              uint64
	ComputeLimit       uint64
	PayerAccount       []byte
	ScriptAccounts     [][]byte
	Signatures         []accountSignatureWrapper
	Status             uint8
}

type transactionCanonicalWrapper struct {
	Script             []byte
	ReferenceBlockHash []byte
	Nonce              uint64
	ComputeLimit       uint64
	PayerAccount       []byte
	ScriptAccounts     [][]byte
}

type accountPublicKeyWrapper struct {
	PublicKey []byte
	SignAlgo  uint
	HashAlgo  uint
	Weight    uint
}

type accountPrivateKeyWrapper struct {
	PrivateKey []byte
	SignAlgo   uint
	HashAlgo   uint
}

type accountSignatureWrapper struct {
	Account   []byte
	Signature []byte
}
