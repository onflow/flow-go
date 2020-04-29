package generator

import (
	"fmt"

	"github.com/onflow/cadence"
	encoding "github.com/onflow/cadence/encoding/json"

	"github.com/dapperlabs/flow-go/model/flow"
)

type Events struct {
	count uint32
	ids   *Identifiers
}

func EventGenerator() *Events {
	return &Events{
		count: 1,
		ids:   IdentifierGenerator(),
	}
}

func (g *Events) New() flow.Event {
	identifier := fmt.Sprintf("FooEvent%d", g.count)
	typeID := "test." + identifier

	testEventType := cadence.EventType{
		TypeID:     typeID,
		Identifier: identifier,
		Fields: []cadence.Field{
			{
				Identifier: "a",
				Type:       cadence.IntType{},
			},
			{
				Identifier: "b",
				Type:       cadence.StringType{},
			},
		},
	}

	testEvent := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewInt(int(g.count)),
			cadence.NewString("foo"),
		}).WithType(testEventType)

	payload, err := encoding.Encode(testEvent)
	if err != nil {
		panic(fmt.Sprintf("unexpected error while encoding events "))
	}
	event := flow.Event{
		Type:             flow.EventType(typeID),
		TransactionID:    g.ids.New(),
		TransactionIndex: g.count,
		EventIndex:       g.count,
		Payload:          payload,
	}

	g.count++

	return event
}

type Addresses struct {
	count int
}

func AddressGenerator() *Addresses {
	return &Addresses{1}
}

func (g *Addresses) New() flow.Address {
	addr := flow.BytesToAddress([]byte{uint8(g.count)})
	g.count++
	return addr
}

//type AccountKeys struct {
//	count int
//	ids   *Identifiers
//}
//
//func AccountKeyGenerator() *AccountKeys {
//	return &AccountKeys{
//		count: 1,
//		ids:   IdentifierGenerator(),
//	}
//}
//
//func (g *AccountKeys) New() *flow.AccountKey {
//	accountKey, _ := g.NewWithSigner()
//	return accountKey
//}
//
//func (g *AccountKeys) NewWithSigner() (*flow.AccountPublicKey, crypto.ThresholdSigner) {
//	seed := make([]byte, crypto.MinSeedLengthECDSA_P256)
//	for i := range seed {
//		seed[i] = uint8(g.count)
//	}
//
//	privateKey, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, seed)
//
//	if err != nil {
//		panic(err)
//	}
//
//	accountKey := flow.AccountKey{
//		ID:             g.count,
//		PublicKey:      privateKey.PublicKey(),
//		SigAlgo:        crypto.ECDSA_P256,
//		HashAlgo:       crypto.SHA3_256,
//		Weight:         flow.AccountKeyWeightThreshold,
//		SequenceNumber: 42,
//	}
//
//	g.count++
//
//	return &accountKey, crypto.NewInMemorySigner(privateKey, accountKey.HashAlgo)
//}

//type Accounts struct {
//	addresses   *Addresses
//	accountKeys *AccountKeys
//}
//
//func AccountGenerator() *Accounts {
//	return &Accounts{
//		addresses:   AddressGenerator(),
//		accountKeys: AccountKeyGenerator(),
//	}
//}

//func (g *Accounts) New() *flow.Account {
//	return &flow.Account{
//		Address: g.addresses.New(),
//		Balance: 10,
//		Keys: []*flow.AccountKey{
//			g.accountKeys.New(),
//			g.accountKeys.New(),
//		},
//		Code: nil,
//	}
//}

type Transactions struct {
	count int
}

func TransactionGenerator() *Transactions {
	return &Transactions{1}
}

//func (g *Transactions) NewUnsigned() *flow.Transaction {
//	blockID := newIdentifier(g.count + 1)
//
//	accounts := AccountGenerator()
//	accountA := accounts.New()
//	accountB := accounts.New()
//
//	return flow.NewTransaction().
//		SetScript(ScriptHelloWorld).
//		SetReferenceBlockID(blockID).
//		SetGasLimit(42).
//		SetProposalKey(accountA.Address, accountA.Keys[0].ID, accountA.Keys[0].SequenceNumber).
//		AddAuthorizer(accountA.Address).
//		SetPayer(accountB.Address)
//}

//func (g *Transactions) New() *flow.TransactionBody {
//	tx := g.NewUnsigned()
//
//	// sign payload with proposal key
//	err := tx.SignPayload(
//		tx.ProposalKey.Address,
//		tx.ProposalKey.KeyID,
//		MockSigner([]byte{uint8(tx.ProposalKey.KeyID)}),
//	)
//	if err != nil {
//		panic(err)
//	}
//
//	// sign payload as each authorizer
//	for _, addr := range tx.Authorizers {
//		err = tx.SignPayload(addr, 0, MockSigner(addr.Bytes()))
//		if err != nil {
//			panic(err)
//		}
//	}
//
//	// sign envelope as payer
//	err = tx.SignEnvelope(tx.Payer, 0, MockSigner(tx.Payer.Bytes()))
//	if err != nil {
//		panic(err)
//	}
//
//	return tx
//}
