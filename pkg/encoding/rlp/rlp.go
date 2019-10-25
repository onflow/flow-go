package rlp

import (
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) EncodeTransaction(tx *types.Transaction) ([]byte, error) {
	scriptAccounts := make([][]byte, len(tx.ScriptAccounts))
	for i, scriptAccount := range tx.ScriptAccounts {
		scriptAccounts[i] = scriptAccount.Bytes()
	}

	signatures := make([]accountSignatureWrapper, len(tx.Signatures))
	for i, signature := range tx.Signatures {
		signatures[i] = accountSignatureWrapper{
			Account:   signature.Account.Bytes(),
			Signature: signature.Signature,
		}
	}

	w := transactionWrapper{
		Script:             tx.Script,
		ReferenceBlockHash: tx.ReferenceBlockHash,
		Nonce:              tx.Nonce,
		ComputeLimit:       tx.ComputeLimit,
		PayerAccount:       tx.PayerAccount.Bytes(),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
		Status:             uint8(tx.Status),
	}

	return rlp.EncodeToBytes(&w)
}

func (e *Encoder) EncodeCanonicalTransaction(tx *types.Transaction) ([]byte, error) {
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

	return rlp.EncodeToBytes(&w)
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

func (e *Encoder) EncodeAccountPrivateKey(a *types.AccountPrivateKey) ([]byte, error) {
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

func (e *Encoder) DecodeAccountPrivateKey(b []byte) (*types.AccountPrivateKey, error) {
	var w accountPrivateKeyWrapper

	err := rlp.DecodeBytes(b, &w)
	if err != nil {
		return nil, err
	}

	signAlgo := crypto.SigningAlgorithm(w.SignAlgo)
	hashAlgo := crypto.HashingAlgorithm(w.HashAlgo)

	privateKey, err := crypto.DecodePrivateKey(signAlgo, w.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &types.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   signAlgo,
		HashAlgo:   hashAlgo,
	}, nil
}
