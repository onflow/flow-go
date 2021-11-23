package rest

import (
	"encoding/json"
	"github.com/onflow/flow-go/engine/access/rest/generated"
)

type responseDecorator struct {
	response interface{}
	request  *requestDecorator
	link     LinkGenerator
}

func newResponseDecorator(req *requestDecorator, response interface{}, link LinkGenerator) (*responseDecorator, error) {
	res := &responseDecorator{
		response: response,
		request:  req,
		link:     link,
	}
	err := res.process()

	return res, err
}

// process finds out what is the response type and applies correct decorators
func (r *responseDecorator) process() error {
	switch res := r.response.(type) {
	case generated.Account:
		return r.account(res)
	}

	return nil
}

// JSON serializer
func (r *responseDecorator) JSON() ([]byte, error) {
	err := r.process()

	serialised, err := json.Marshal(r.response)
	if err != nil {
		return nil, err
	}

	return serialised, nil
}

func (r *responseDecorator) account(account generated.Account) error {
	link, err := r.link.AccountLink(account.Address)
	if err != nil {
		return err
	}
	account.Links = LinkResponse(link)

	account.Expandable = &generated.AccountExpandable{}

	if !r.request.expands(ExpandableFieldContracts) {
		contractLink, err := r.link.AccountContractLink(account.Address)
		if err != nil {
			return err
		}
		account.Contracts = nil
		account.Expandable.Contracts = contractLink
	}

	if !r.request.expands(ExpandableFieldKeys) {
		keysLink, err := r.link.AccountKeysLink(account.Address)
		if err != nil {
			return err
		}
		account.Keys = nil
		account.Expandable.Keys = keysLink
	}

	// todo here we can handle selection

	return nil
}
