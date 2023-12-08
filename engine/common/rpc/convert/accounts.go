package convert

import (
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// AccountToMessage converts a flow.Account to a protobuf message
func AccountToMessage(a *flow.Account) (*entities.Account, error) {
	keys := make([]*entities.AccountKey, len(a.Keys))
	for i, k := range a.Keys {
		messageKey, err := AccountKeyToMessage(k)
		if err != nil {
			return nil, err
		}
		keys[i] = messageKey
	}

	return &entities.Account{
		Address:   a.Address.Bytes(),
		Balance:   a.Balance,
		Code:      nil,
		Keys:      keys,
		Contracts: a.Contracts,
	}, nil
}

// MessageToAccount converts a protobuf message to a flow.Account
func MessageToAccount(m *entities.Account) (*flow.Account, error) {
	if m == nil {
		return nil, ErrEmptyMessage
	}

	accountKeys := make([]flow.AccountPublicKey, len(m.GetKeys()))
	for i, key := range m.GetKeys() {
		accountKey, err := MessageToAccountKey(key)
		if err != nil {
			return nil, err
		}

		accountKeys[i] = *accountKey
	}

	return &flow.Account{
		Address:   flow.BytesToAddress(m.GetAddress()),
		Balance:   m.GetBalance(),
		Keys:      accountKeys,
		Contracts: m.Contracts,
	}, nil
}

// AccountKeyToMessage converts a flow.AccountPublicKey to a protobuf message
func AccountKeyToMessage(a flow.AccountPublicKey) (*entities.AccountKey, error) {
	publicKey := a.PublicKey.Encode()
	return &entities.AccountKey{
		Index:          uint32(a.Index),
		PublicKey:      publicKey,
		SignAlgo:       uint32(a.SignAlgo),
		HashAlgo:       uint32(a.HashAlgo),
		Weight:         uint32(a.Weight),
		SequenceNumber: uint32(a.SeqNumber),
		Revoked:        a.Revoked,
	}, nil
}

// MessageToAccountKey converts a protobuf message to a flow.AccountPublicKey
func MessageToAccountKey(m *entities.AccountKey) (*flow.AccountPublicKey, error) {
	if m == nil {
		return nil, ErrEmptyMessage
	}

	sigAlgo := crypto.SigningAlgorithm(m.GetSignAlgo())
	hashAlgo := hash.HashingAlgorithm(m.GetHashAlgo())

	publicKey, err := crypto.DecodePublicKey(sigAlgo, m.GetPublicKey())
	if err != nil {
		return nil, err
	}

	return &flow.AccountPublicKey{
		Index:     int(m.GetIndex()),
		PublicKey: publicKey,
		SignAlgo:  sigAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(m.GetWeight()),
		SeqNumber: uint64(m.GetSequenceNumber()),
		Revoked:   m.GetRevoked(),
	}, nil
}
