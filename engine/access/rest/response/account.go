package response

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/model/flow"
)

type AccountResponse struct {
	Address    string                    `json:"address"`
	Balance    string                    `json:"balance"`
	Keys       AccountPublicKeysResponse `json:"keys,omitempty"`
	Contracts  map[string]string         `json:"contracts,omitempty"`
	Expandable AccountExpandableResponse `json:"_expandable"`
	Links      LinkResponse              `json:"_links,omitempty"`
}

func (a *AccountResponse) Build(flowAccount *flow.Account, link rest.LinkGenerator, expand map[string]bool) error {
	account := AccountResponse{
		Address: flowAccount.Address.String(),
		Balance: fromUint64(flowAccount.Balance),
	}

	a.Expandable = AccountExpandableResponse{
		Keys:      "keys",
		Contracts: "contracts",
	}

	if expand[a.Expandable.Keys] {
		var keys AccountPublicKeysResponse
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

	var self LinkResponse
	err := self.Build(link.AccountLink(account.Address))
	if err != nil {
		return err
	}

	return nil
}

type AccountPublicKeyResponse struct {
	Index            string                    `json:"index"`
	PublicKey        string                    `json:"public_key"`
	SigningAlgorithm *SigningAlgorithmResponse `json:"signing_algorithm"`
	HashingAlgorithm *HashingAlgorithmResponse `json:"hashing_algorithm"`
	SequenceNumber   string                    `json:"sequence_number"`
	Weight           string                    `json:"weight"`
	Revoked          bool                      `json:"revoked"`
}

func (a *AccountPublicKeyResponse) Build(k flow.AccountPublicKey) {
	sigAlgo := SigningAlgorithmResponse(k.SignAlgo.String())
	hashAlgo := HashingAlgorithmResponse(k.HashAlgo.String())

	a.Index = fromUint64(uint64(k.Index))
	a.PublicKey = k.PublicKey.String()
	a.SigningAlgorithm = &sigAlgo
	a.HashingAlgorithm = &hashAlgo
	a.SequenceNumber = fromUint64(k.SeqNumber)
	a.Weight = fromUint64(uint64(k.Weight))
	a.Revoked = k.Revoked
}

type AccountPublicKeysResponse []AccountPublicKeyResponse

func (a *AccountPublicKeysResponse) Build(accountKeys []flow.AccountPublicKey) {
	keys := make([]AccountPublicKeyResponse, len(accountKeys))
	for i, k := range accountKeys {
		var key AccountPublicKeyResponse
		key.Build(k)
		keys[i] = key
	}

	*a = keys
}

type AccountExpandableResponse struct {
	Keys      string `json:"keys,omitempty"`
	Contracts string `json:"contracts,omitempty"`
}
