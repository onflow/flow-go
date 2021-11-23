package rest

import (
	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

type LinkFun func(id flow.Identifier) (string, error)

type LinkGenerator interface {
	BlockLink(id flow.Identifier) (string, error)
	TransactionLink(id flow.Identifier) (string, error)
	TransactionResultLink(id flow.Identifier) (string, error)
	PayloadLink(id flow.Identifier) (string, error)
	ExecutionResultLink(id flow.Identifier) (string, error)
	AccountLink(address string) (string, error)
	AccountContractLink(address string) (string, error)
	AccountKeysLink(address string) (string, error)
}

type LinkGeneratorImpl struct {
	router *mux.Router
}

func NewLinkGeneratorImpl(router *mux.Router) *LinkGeneratorImpl {
	return &LinkGeneratorImpl{
		router: router,
	}
}

func (generator *LinkGeneratorImpl) BlockLink(id flow.Identifier) (string, error) {
	return generator.link(getBlocksByIDRoute, id)
}
func (generator *LinkGeneratorImpl) PayloadLink(id flow.Identifier) (string, error) {
	return generator.link(getBlocksByIDRoute, id)
}
func (generator *LinkGeneratorImpl) ExecutionResultLink(id flow.Identifier) (string, error) {
	return generator.link(getExecutionResultByIDRoute, id)
}

func (generator *LinkGeneratorImpl) TransactionLink(id flow.Identifier) (string, error) {
	return generator.link(getTransactionByIDRoute, id) // todo(sideninja) handler now has route attribute, we could get the route from there and just use this as route builder to return the Link, also discuss having this return generated.Links, or even be moved to converters
}

func (generator *LinkGeneratorImpl) TransactionResultLink(id flow.Identifier) (string, error) {
	return generator.link(getTransactionResultByIDRoute, id)
}

func (generator *LinkGeneratorImpl) AccountLink(address string) (string, error) {
	return generator.linkAddress(getAccountRoute, address)
}

func (generator *LinkGeneratorImpl) AccountContractLink(address string) (string, error) {
	return generator.linkAddress(getAccountContractsRoute, address)
}

func (generator *LinkGeneratorImpl) AccountKeysLink(address string) (string, error) {
	return generator.linkAddress(getAccountKeysRoute, address)
}

func selfLink(id flow.Identifier, linkFun LinkFun) (*generated.Links, error) {
	url, err := linkFun(id)
	if err != nil {
		return nil, err
	}
	return &generated.Links{
		Self: url,
	}, nil
}

func (generator *LinkGeneratorImpl) link(route string, id flow.Identifier) (string, error) {
	url, err := generator.router.Get(route).URLPath("id", id.String())
	if err != nil {
		return "", err
	}
	// TODO: remove the leading '/v1' from the generated link
	return url.String(), nil
}

func (generator *LinkGeneratorImpl) linkAddress(route string, address string) (string, error) {
	url, err := generator.router.Get(route).URLPath("address", address)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}
