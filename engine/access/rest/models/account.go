package models

import (
	"github.com/onflow/flow-go/model/flow"
)

const expandableKeys = "keys"
const expandableContracts = "contracts"

func (a *Account) Build(flowAccount *flow.Account, link LinkGenerator, expand map[string]bool) error {
	a.Address = flowAccount.Address.String()
	a.Balance = fromUint64(flowAccount.Balance)
	a.Expandable = &AccountExpandable{}

	if expand[expandableKeys] {
		var keys AccountPublicKeys
		keys.Build(flowAccount.Keys)
		a.Keys = keys
	} else {
		a.Expandable.Keys = expandableKeys
	}

	if expand[expandableContracts] {
		contracts := make(map[string]string, len(flowAccount.Contracts))
		for name, code := range flowAccount.Contracts {
			contracts[name] = ToBase64(code)
		}
		a.Contracts = contracts
	} else {
		a.Expandable.Contracts = expandableContracts
	}

	var self Links
	err := self.Build(link.AccountLink(a.Address))
	if err != nil {
		return err
	}

	a.Links = &self

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
