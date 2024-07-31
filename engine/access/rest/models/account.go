package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const expandableKeys = "keys"
const expandableContracts = "contracts"

func (a *Account) Build(flowAccount *flow.Account, link LinkGenerator, expand map[string]bool) error {
	a.Address = flowAccount.Address.String()
	a.Balance = util.FromUint(flowAccount.Balance)
	a.Expandable = &AccountExpandable{}

	if expand[expandableKeys] {
		var keys AccountKeys
		keys.Build(flowAccount.Keys)
		a.Keys = keys
	} else {
		a.Expandable.Keys = expandableKeys
	}

	if expand[expandableContracts] {
		contracts := make(map[string]string, len(flowAccount.Contracts))
		for name, code := range flowAccount.Contracts {
			contracts[name] = util.ToBase64(code)
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

	a.Index = util.FromUint(k.Index)
	a.PublicKey = k.PublicKey.String()
	a.SigningAlgorithm = &sigAlgo
	a.HashingAlgorithm = &hashAlgo
	a.SequenceNumber = util.FromUint(k.SeqNumber)
	a.Weight = util.FromUint(uint64(k.Weight))
	a.Revoked = k.Revoked
}

type AccountKeys []AccountPublicKey

func (a *AccountKeys) Build(accountKeys []flow.AccountPublicKey) {
	keys := make([]AccountPublicKey, len(accountKeys))
	for i, k := range accountKeys {
		var key AccountPublicKey
		key.Build(k)
		keys[i] = key
	}

	*a = keys
}

// Build function use model AccountPublicKeys type for GetAccountKeys call
// AccountPublicKeys is an auto-generated type from the openapi spec
func (a *AccountPublicKeys) Build(accountKeys []flow.AccountPublicKey) {
	keys := make([]AccountPublicKey, len(accountKeys))
	for i, k := range accountKeys {
		var key AccountPublicKey
		key.Build(k)
		keys[i] = key
	}

	a.Keys = keys
}
