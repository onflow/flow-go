package convert

import (
	"errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/proto/sdk/entities"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/model/types"
)

var ErrEmptyMessage = errors.New("protobuf message is empty")

func MessageToBlockHeader(m *entities.BlockHeader) types.BlockHeader {
	return types.BlockHeader{
		Hash:              crypto.BytesToHash(m.GetHash()),
		PreviousBlockHash: crypto.BytesToHash(m.GetPreviousBlockHash()),
		Number:            m.GetNumber(),
		TransactionCount:  m.GetTransactionCount(),
	}
}

func BlockHeaderToMessage(b types.BlockHeader) *entities.BlockHeader {
	return &entities.BlockHeader{
		Hash:              b.Hash,
		PreviousBlockHash: b.PreviousBlockHash,
		Number:            b.Number,
		TransactionCount:  b.TransactionCount,
	}
}

func MessageToAccountSignature(m *entities.AccountSignature) types.AccountSignature {
	return types.AccountSignature{
		Account:   types.BytesToAddress(m.GetAccount()),
		Signature: m.GetSignature(),
	}
}

func AccountSignatureToMessage(a types.AccountSignature) *entities.AccountSignature {
	return &entities.AccountSignature{
		Account:   a.Account.Bytes(),
		Signature: a.Signature,
	}
}

func MessageToTransaction(m *entities.Transaction) (types.Transaction, error) {
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

func TransactionToMessage(t types.Transaction) *entities.Transaction {
	scriptAccounts := make([][]byte, len(t.ScriptAccounts))
	for i, account := range t.ScriptAccounts {
		scriptAccounts[i] = account.Bytes()
	}

	signatures := make([]*entities.AccountSignature, len(t.Signatures))
	for i, accountSig := range t.Signatures {
		signatures[i] = AccountSignatureToMessage(accountSig)
	}

	return &entities.Transaction{
		Script:             t.Script,
		ReferenceBlockHash: t.ReferenceBlockHash,
		Nonce:              t.Nonce,
		ComputeLimit:       t.ComputeLimit,
		PayerAccount:       t.PayerAccount.Bytes(),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
	}
}

func MessageToAccount(m *entities.Account) (types.Account, error) {
	if m == nil {
		return types.Account{}, ErrEmptyMessage
	}

	accountKeys := make([]types.AccountKey, len(m.Keys))
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

func AccountToMessage(a types.Account) *entities.Account {
	accountKeys := make([]*entities.AccountKey, len(a.Keys))
	for i, key := range a.Keys {
		accountKeys[i] = AccountKeyToMessage(key)
	}

	return &entities.Account{
		Address: a.Address.Bytes(),
		Balance: a.Balance,
		Code:    a.Code,
		Keys:    accountKeys,
	}
}

func MessageToAccountKey(m *entities.AccountKey) (types.AccountKey, error) {
	if m == nil {
		return types.AccountKey{}, ErrEmptyMessage
	}

	return types.AccountKey{
		PublicKey: m.PublicKey,
		Weight:    int(m.Weight),
	}, nil
}

func AccountKeyToMessage(a types.AccountKey) *entities.AccountKey {
	return &entities.AccountKey{
		PublicKey: a.PublicKey,
		Weight:    uint32(a.Weight),
	}
}

func MessageToEventQuery(m *observation.GetEventsRequest) types.EventQuery {
	return types.EventQuery{
		ID:         m.GetEventId(),
		StartBlock: m.GetStartBlock(),
		EndBlock:   m.GetEndBlock(),
	}
}

func EventQueryToMessage(q *types.EventQuery) *observation.GetEventsRequest {
	return &observation.GetEventsRequest{
		EventId:    q.ID,
		StartBlock: q.StartBlock,
		EndBlock:   q.EndBlock,
	}
}
