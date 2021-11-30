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
	return generator.linkForID(getBlocksByIDRoute, id)
}
func (generator *LinkGeneratorImpl) PayloadLink(id flow.Identifier) (string, error) {
	return generator.linkForID(getBlocksByIDRoute, id)
}
func (generator *LinkGeneratorImpl) ExecutionResultLink(id flow.Identifier) (string, error) {
	return generator.linkForID(getExecutionResultByIDRoute, id)
}

func (generator *LinkGeneratorImpl) TransactionLink(id flow.Identifier) (string, error) {
	return generator.linkForID(getTransactionByIDRoute, id) // todo(sideninja) handler now has route attribute, we could get the route from there and just use this as route builder to return the Link, also discuss having this return generated.Links, or even be moved to converters
}

func (generator *LinkGeneratorImpl) TransactionResultLink(id flow.Identifier) (string, error) {
	return generator.linkForID(getTransactionResultByIDRoute, id)
}

func (generator *LinkGeneratorImpl) AccountLink(address string) (string, error) {
	return generator.link(getAccountRoute, "address", address)
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

func (generator *LinkGeneratorImpl) linkForID(route string, id flow.Identifier) (string, error) {
	return generator.link(route, "id", id.String())
}

func (generator *LinkGeneratorImpl) link(route string, key string, value string) (string, error) {
	url, err := generator.router.Get(route).URLPath(key, value)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}
