package rlp

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type transactionWrapper struct {
	Script             []byte
	ReferenceBlockHash []byte
	Nonce              uint64
	ComputeLimit       uint64
	PayerAccount       []byte
	ScriptAccounts     [][]byte
}

func wrapTransaction(tx flow.Transaction) transactionWrapper {
	scriptAccounts := make([][]byte, len(tx.ScriptAccounts))
	for i, scriptAccount := range tx.ScriptAccounts {
		scriptAccounts[i] = scriptAccount.Bytes()
	}

	w := transactionWrapper{
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

type chunkWrapper struct {
	Transactions          []transactionWrapper
	TotalComputationLimit uint64
}

type collectionWrapper struct {
	Transactions []transactionWrapper
}
