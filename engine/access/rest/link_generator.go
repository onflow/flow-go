package rest

import (
	"github.com/gorilla/mux"

	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

type LinkFun func(id flow.Identifier) (string, error)

type LinkGenerator interface {
	BlockLink(id flow.Identifier) (string, error)
	PayloadLink(id flow.Identifier) (string, error)
	ExecutionResultLink(id flow.Identifier) (string, error)
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

func (generator *LinkGeneratorImpl) link(route string, id flow.Identifier) (string, error) {
	url, err := generator.router.Get(route).URLPath("id", id.String())
	if err != nil {
		return "", err
	}
	// TODO: remove the leading '/v1' from the generated link
	return url.String(), nil
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
