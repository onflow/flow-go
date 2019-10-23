package proto

import (
	"errors"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/services/observe"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/types"
)

var ErrEmptyMessage = errors.New("protobuf message is empty")

func MessageToBlockHeader(m *shared.BlockHeader) types.BlockHeader {
	return types.BlockHeader{
		Hash:              crypto.BytesToHash(m.GetHash()),
		PreviousBlockHash: crypto.BytesToHash(m.GetPreviousBlockHash()),
		Number:            m.GetNumber(),
		TransactionCount:  m.GetTransactionCount(),
	}
}

func BlockHeaderToMessage(b types.BlockHeader) *shared.BlockHeader {
	return &shared.BlockHeader{
		Hash:              b.Hash,
		PreviousBlockHash: b.PreviousBlockHash,
		Number:            b.Number,
		TransactionCount:  b.TransactionCount,
	}
}

func MessageToAccountSignature(m *shared.AccountSignature) types.AccountSignature {
	return types.AccountSignature{
		Account:   types.BytesToAddress(m.GetAccount()),
		Signature: m.GetSignature(),
	}
}

func AccountSignatureToMessage(a types.AccountSignature) *shared.AccountSignature {
	return &shared.AccountSignature{
		Account:   a.Account.Bytes(),
		Signature: a.Signature,
	}
}

func MessageToTransaction(m *shared.Transaction) (types.Transaction, error) {
	if m == nil {
		return types.Transaction{}, ErrEmptyMessage
	}

	scriptAccounts := make([]types.Address, len(m.ScriptAccounts))
	for i, account := range m.ScriptAccounts {
		scriptAccounts[i] = types.BytesToAddress(account)
	}

	signatures := make([]types.AccountSignature, len(m.Signatures))
	for i, accountSig := range m.Signatures {
		signatures[i] = MessageToAccountSignature(accountSig)
	}

	return types.Transaction{
		Script:             m.GetScript(),
		ReferenceBlockHash: m.ReferenceBlockHash,
		Nonce:              m.GetNonce(),
		ComputeLimit:       m.GetComputeLimit(),
		PayerAccount:       types.BytesToAddress(m.PayerAccount),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
	}, nil
}

func TransactionToMessage(t types.Transaction) *shared.Transaction {
	scriptAccounts := make([][]byte, len(t.ScriptAccounts))
	for i, account := range t.ScriptAccounts {
		scriptAccounts[i] = account.Bytes()
	}

	signatures := make([]*shared.AccountSignature, len(t.Signatures))
	for i, accountSig := range t.Signatures {
		signatures[i] = AccountSignatureToMessage(accountSig)
	}

	return &shared.Transaction{
		Script:             t.Script,
		ReferenceBlockHash: t.ReferenceBlockHash,
		Nonce:              t.Nonce,
		ComputeLimit:       t.ComputeLimit,
		PayerAccount:       t.PayerAccount.Bytes(),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
	}
}

func MessageToAccount(m *shared.Account) (types.Account, error) {
	if m == nil {
		return types.Account{}, ErrEmptyMessage
	}

	accountKeys := make([]types.AccountPublicKey, len(m.Keys))
	for i, key := range m.Keys {
		accountKey, err := MessageToAccountKey(key)
		if err != nil {
			return types.Account{}, err
		}

		accountKeys[i] = accountKey
	}

	return types.Account{
		Address: types.BytesToAddress(m.Address),
		Balance: m.Balance,
		Code:    m.Code,
		Keys:    accountKeys,
	}, nil
}

func AccountToMessage(a types.Account) (*shared.Account, error) {
	accountKeys := make([]*shared.AccountKey, len(a.Keys))
	for i, key := range a.Keys {
		accountKeyMsg, err := AccountKeyToMessage(key)
		if err != nil {
			return nil, err
		}
		accountKeys[i] = accountKeyMsg
	}

	return &shared.Account{
		Address: a.Address.Bytes(),
		Balance: a.Balance,
		Code:    a.Code,
		Keys:    accountKeys,
	}, nil
}

func MessageToAccountKey(m *shared.AccountKey) (types.AccountPublicKey, error) {
	if m == nil {
		return types.AccountPublicKey{}, ErrEmptyMessage
	}

	signAlgo := crypto.SigningAlgorithm(m.GetSignAlgo())
	hashAlgo := crypto.HashingAlgorithm(m.GetHashAlgo())

	publicKey, err := crypto.DecodePublicKey(signAlgo, m.GetPublicKey())
	if err != nil {
		return types.AccountPublicKey{}, err
	}

	return types.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  signAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(m.GetWeight()),
	}, nil
}

func AccountKeyToMessage(a types.AccountPublicKey) (*shared.AccountKey, error) {
	publicKey, err := a.PublicKey.Encode()
	if err != nil {
		return nil, err
	}

	return &shared.AccountKey{
		PublicKey: publicKey,
		SignAlgo:  uint32(a.SignAlgo),
		HashAlgo:  uint32(a.HashAlgo),
		Weight:    uint32(a.Weight),
	}, nil
}

func MessageToEventQuery(m *observe.GetEventsRequest) types.EventQuery {
	return types.EventQuery{
		ID:         m.GetEventId(),
		StartBlock: m.GetStartBlock(),
		EndBlock:   m.GetEndBlock(),
	}
}

func EventQueryToMessage(q *types.EventQuery) *observe.GetEventsRequest {
	return &observe.GetEventsRequest{
		EventId:    q.ID,
		StartBlock: q.StartBlock,
		EndBlock:   q.EndBlock,
	}
}
