package rlp

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
