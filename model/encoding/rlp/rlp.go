package rlp

import (
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) EncodeTransaction(tx flow.Transaction) ([]byte, error) {
	w := wrapTransaction(tx)
	return rlp.EncodeToBytes(&w)
}

func (e *Encoder) EncodeAccountPublicKey(a flow.AccountPublicKey) ([]byte, error) {
	publicKey, err := a.PublicKey.Encode()
	if err != nil {
		return nil, err
	}

	w := accountPublicKeyWrapper{
		PublicKey: publicKey,
		SignAlgo:  uint(a.SignAlgo),
		HashAlgo:  uint(a.HashAlgo),
		Weight:    uint(a.Weight),
	}

	return rlp.EncodeToBytes(&w)
}

func (e *Encoder) DecodeAccountPublicKey(b []byte) (a flow.AccountPublicKey, err error) {
	var w accountPublicKeyWrapper

	err = rlp.DecodeBytes(b, &w)
	if err != nil {
		return a, err
	}

	signAlgo := crypto.SigningAlgorithm(w.SignAlgo)
	hashAlgo := crypto.HashingAlgorithm(w.HashAlgo)

	publicKey, err := crypto.DecodePublicKey(signAlgo, w.PublicKey)
	if err != nil {
		return a, err
	}

	return flow.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  signAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(w.Weight),
	}, nil
}

func (e *Encoder) EncodeAccountPrivateKey(a flow.AccountPrivateKey) ([]byte, error) {
	privateKey, err := a.PrivateKey.Encode()
	if err != nil {
		return nil, err
	}

	w := accountPrivateKeyWrapper{
		PrivateKey: privateKey,
		SignAlgo:   uint(a.SignAlgo),
		HashAlgo:   uint(a.HashAlgo),
	}

	return rlp.EncodeToBytes(&w)
}

func (e *Encoder) DecodeAccountPrivateKey(b []byte) (a flow.AccountPrivateKey, err error) {
	var w accountPrivateKeyWrapper

	err = rlp.DecodeBytes(b, &w)
	if err != nil {
		return a, err
	}

	signAlgo := crypto.SigningAlgorithm(w.SignAlgo)
	hashAlgo := crypto.HashingAlgorithm(w.HashAlgo)

	privateKey, err := crypto.DecodePrivateKey(signAlgo, w.PrivateKey)
	if err != nil {
		return a, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   signAlgo,
		HashAlgo:   hashAlgo,
	}, nil
}

func (e *Encoder) EncodeChunk(c flow.Chunk) ([]byte, error) {
	transactions := make([]transactionWrapper, 0, len(c.Transactions))

	for i, tx := range c.Transactions {
		transactions[i] = wrapTransaction(*tx)
	}

	w := chunkWrapper{
		Transactions:          transactions,
		TotalComputationLimit: c.TotalComputationLimit,
	}

	return rlp.EncodeToBytes(&w)
}

func (e *Encoder) EncodeCollection(c flow.Collection) ([]byte, error) {
	transactions := make([]transactionWrapper, 0, len(c.Transactions))

	for i, tx := range c.Transactions {
		transactions[i] = wrapTransaction(*tx)
	}

	w := collectionWrapper{
		Transactions: transactions,
	}

	return rlp.EncodeToBytes(&w)
}
