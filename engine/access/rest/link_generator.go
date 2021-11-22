package rest

import (
	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/model/flow"
)

type LinkGenerator interface {
	BlockLink(id flow.Identifier) (string, error)
	TransactionLink(id flow.Identifier) (string, error)
	TransactionResultLink(id flow.Identifier) (string, error)
}

type LinkGeneratorImpl struct {
	router *mux.Router
}

func NewLinkGeneratorImpl(router *mux.Router) *LinkGeneratorImpl {
	return &LinkGeneratorImpl{
		router: router,
	}
}

func linkFromRoute(route *mux.Route, id flow.Identifier) (string, error) {
	url, err := route.URLPath("id", id.String())
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

func (generator *LinkGeneratorImpl) BlockLink(id flow.Identifier) (string, error) {
	return linkFromRoute(generator.router.Get(getBlocksByIDRoute), id)
}

func (generator *LinkGeneratorImpl) TransactionLink(id flow.Identifier) (string, error) {
	return linkFromRoute(generator.router.Get(getTransactionByIDRoute), id) // todo(sideninja) handler now has route attribute, we could get the route from there and just use this as route builder to return the Link, also discuss having this return generated.Links, or even be moved to converters
}

func (generator *LinkGeneratorImpl) TransactionResultLink(id flow.Identifier) (string, error) {
	return linkFromRoute(generator.router.Get(getTransactionResultByIDRoute), id)
}
