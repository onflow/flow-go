package models

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

func (a *Account) Build(flowAccount *flow.Account, link rest.LinkGenerator, expand map[string]bool) error {
	account := Account{
		Address: flowAccount.Address.String(),
		Balance: fromUint64(flowAccount.Balance),
	}

	a.Expandable = &AccountExpandable{
		Keys:      "keys",
		Contracts: "contracts",
	}

	if expand[a.Expandable.Keys] {
		var keys AccountPublicKeys
		keys.Build(flowAccount.Keys)
		a.Keys = keys

		a.Expandable.Keys = ""
	}

	if expand[a.Expandable.Contracts] {
		contracts := make(map[string]string, len(flowAccount.Contracts))
		for name, code := range flowAccount.Contracts {
			contracts[name] = toBase64(code)
		}
		a.Contracts = contracts
		a.Expandable.Contracts = ""
	}

	var self Links
	err := self.Build(link.AccountLink(account.Address))
	if err != nil {
		return err
	}

	a.Links = self

	return nil
}

func (a *AccountPublicKey) Build(k flow.AccountPublicKey) {
	sigAlgo := SigningAlgorithm(k.SignAlgo.String())
	hashAlgo := HashingAlgorithm(k.HashAlgo.String())

	a.Index = fromUint64(uint64(k.Index))
	a.PublicKey = k.PublicKey.String()
	a.SigningAlgorithm = &sigAlgo
	a.HashingAlgorithm = &hashAlgo
	a.SequenceNumber = fromUint64(k.SeqNumber)
	a.Weight = fromUint64(uint64(k.Weight))
	a.Revoked = k.Revoked
}

type AccountPublicKeys []AccountPublicKey

func (a *AccountPublicKeys) Build(accountKeys []flow.AccountPublicKey) {
	keys := make([]AccountPublicKey, len(accountKeys))
	for i, k := range accountKeys {
		var key AccountPublicKey
		key.Build(k)
		keys[i] = key
	}

	*a = keys
}
