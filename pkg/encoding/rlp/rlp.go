package rlp

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) EncodeTransaction(*types.Transaction) ([]byte, error) {
	return nil, nil
}

func (e *Encoder) EncodeCanonicalTransaction(*types.Transaction) ([]byte, error) {
	return nil, nil
}

func (e *Encoder) EncodeAccountPublicKey(a *types.AccountPublicKey) ([]byte, error) {
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

func (e *Encoder) DecodeAccountPublicKey(b []byte) (*types.AccountPublicKey, error) {
	var w accountPublicKeyWrapper

	err := rlp.DecodeBytes(b, &w)
	if err != nil {
		return nil, err
	}

	signAlgo := crypto.SigningAlgorithm(w.SignAlgo)
	hashAlgo := crypto.HashingAlgorithm(w.HashAlgo)

	publicKey, err := crypto.DecodePublicKey(signAlgo, w.PublicKey)
	if err != nil {
		return nil, err
	}

	return &types.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  signAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(w.Weight),
	}, nil
}
