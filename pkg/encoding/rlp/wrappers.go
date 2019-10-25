package rlp

import "github.com/dapperlabs/flow-go/pkg/types"

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

func wrapTransactionCanonical(tx types.Transaction) transactionCanonicalWrapper {
	scriptAccounts := make([][]byte, len(tx.ScriptAccounts))
	for i, scriptAccount := range tx.ScriptAccounts {
		scriptAccounts[i] = scriptAccount.Bytes()
	}

	w := transactionCanonicalWrapper{
		Script:             tx.Script,
		ReferenceBlockHash: tx.ReferenceBlockHash,
		Nonce:              tx.Nonce,
		ComputeLimit:       tx.ComputeLimit,
		PayerAccount:       tx.PayerAccount.Bytes(),
		ScriptAccounts:     scriptAccounts,
	}

	return w
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

type chunkWrapper struct {
	Transactions          []transactionCanonicalWrapper
	TotalComputationLimit uint64
}

type collectionWrapper struct {
	Transactions []transactionCanonicalWrapper
}
