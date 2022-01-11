package util

import (
	"github.com/gorilla/mux"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rest/models"

	"github.com/onflow/flow-go/model/flow"
)

type LinkFunc func(id flow.Identifier) (string, error)

// LinkGenerator generates the expandable value for the known endpoints
// e.g. "/v1/blocks/c5e935bc75163db82e4a6cf9dc3b54656709d3e21c87385138300abd479c33b7"
type LinkGenerator interface {
	BlockLink(id flow.Identifier) (string, error)
	TransactionLink(id flow.Identifier) (string, error)
	TransactionResultLink(id flow.Identifier) (string, error)
	PayloadLink(id flow.Identifier) (string, error)
	ExecutionResultLink(id flow.Identifier) (string, error)
	AccountLink(address string) (string, error)
	CollectionLink(id flow.Identifier) (string, error)
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
	return generator.linkForID(rest.GetBlocksByIDRoute, id)
}
func (generator *LinkGeneratorImpl) PayloadLink(id flow.Identifier) (string, error) {
	return generator.linkForID(rest.GetBlockPayloadByIDRoute, id)
}
func (generator *LinkGeneratorImpl) ExecutionResultLink(id flow.Identifier) (string, error) {
	return generator.linkForID(rest.GetExecutionResultByIDRoute, id)
}

func (generator *LinkGeneratorImpl) TransactionLink(id flow.Identifier) (string, error) {
	return generator.linkForID(rest.GetTransactionByIDRoute, id)
}

func (generator *LinkGeneratorImpl) TransactionResultLink(id flow.Identifier) (string, error) {
	return generator.linkForID(rest.GetTransactionResultByIDRoute, id)
}

func (generator *LinkGeneratorImpl) CollectionLink(id flow.Identifier) (string, error) {
	return generator.linkForID(rest.GetCollectionByIDRoute, id)
}

func (generator *LinkGeneratorImpl) AccountLink(address string) (string, error) {
	return generator.link(rest.GetAccountRoute, "address", address)
}

// SelfLink generates the _link key value pair for the response
// e.g.
// "_links": { "_self": "/v1/blocks/c5e935bc75163db82e4a6cf9dc3b54656709d3e21c87385138300abd479c33b7" sx}
func SelfLink(id flow.Identifier, linkFun LinkFunc) (*models.Links, error) {
	url, err := linkFun(id)
	if err != nil {
		return nil, err
	}
	return &models.Links{
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
