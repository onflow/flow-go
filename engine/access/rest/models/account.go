package models

import "github.com/onflow/flow-go/model/flow"

type Account struct {
	Address string `json:"address"`

	Balance string `json:"balance"`

	//Keys []AccountPublicKey `json:"keys,omitempty"`

	Contracts map[string]string `json:"contracts,omitempty"`

	//Expandable *AccountExpandable `json:"_expandable"`

	//Links *Links `json:"_links,omitempty"`
}

func (a *Account) Flow() (*flow.Account, error) {
	// convert and validate
	return nil, nil
}

func (a *Account) Build(account *flow.Account) {
	// build account
}
