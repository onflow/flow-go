package models

import (
	"github.com/gorilla/mux"

	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
)

// LinkGenerator generates the expandable value for the known endpoints
// e.g. "/v1/blocks/c5e935bc75163db82e4a6cf9dc3b54656709d3e21c87385138300abd479c33b7"
type LinkGenerator interface {
	commonmodels.LinkGenerator

	ContractLink(identifier string) (string, error)
}

type LinkGeneratorImpl struct {
	commonmodels.LinkGenerator
	router *mux.Router
}

func NewLinkGeneratorImpl(router *mux.Router, baseGenerator commonmodels.LinkGenerator) *LinkGeneratorImpl {
	return &LinkGeneratorImpl{
		LinkGenerator: baseGenerator,
		router:        router,
	}
}

func (generator *LinkGeneratorImpl) ContractLink(identifier string) (string, error) {
	return generator.link("getContract", "identifier", identifier)
}

func (generator *LinkGeneratorImpl) link(route string, key string, value string) (string, error) {
	url, err := generator.router.Get(route).URLPath(key, value)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}
