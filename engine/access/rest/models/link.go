package models

import (
	"github.com/gorilla/mux"
	"github.com/onflow/flow-go/model/flow"
)

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

func (l *Links) Build(link string, err error) error {
	if err != nil {
		return err
	}

	l.Self = link
	return nil
}

type LinkFunc func(id flow.Identifier) (string, error)

type LinkGeneratorImpl struct {
	router *mux.Router
}

func NewLinkGeneratorImpl(router *mux.Router) *LinkGeneratorImpl {
	return &LinkGeneratorImpl{
		router: router,
	}
}

func (generator *LinkGeneratorImpl) BlockLink(id flow.Identifier) (string, error) {
	return generator.linkForID("getBlocksByIDs", id)
}
func (generator *LinkGeneratorImpl) PayloadLink(id flow.Identifier) (string, error) {
	return generator.linkForID("getBlockPayloadByID", id)
}
func (generator *LinkGeneratorImpl) ExecutionResultLink(id flow.Identifier) (string, error) {
	return generator.linkForID("getExecutionResultByID", id)
}

func (generator *LinkGeneratorImpl) TransactionLink(id flow.Identifier) (string, error) {
	return generator.linkForID("getTransactionByID", id)
}

func (generator *LinkGeneratorImpl) TransactionResultLink(id flow.Identifier) (string, error) {
	return generator.linkForID("getTransactionResultByID", id)
}

func (generator *LinkGeneratorImpl) CollectionLink(id flow.Identifier) (string, error) {
	return generator.linkForID("getCollectionByID", id)
}

func (generator *LinkGeneratorImpl) AccountLink(address string) (string, error) {
	return generator.link("getAccount", "address", address)
}

// SelfLink generates the _link key value pair for the response
// e.g.
// "_links": { "_self": "/v1/blocks/c5e935bc75163db82e4a6cf9dc3b54656709d3e21c87385138300abd479c33b7" sx}
func SelfLink(id flow.Identifier, linkFun LinkFunc) (*Links, error) {
	url, err := linkFun(id)
	if err != nil {
		return nil, err
	}
	var link Links
	link.Build(url, nil)
	return &link, nil
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
