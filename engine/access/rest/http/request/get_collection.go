package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
)

const ExpandsTransactions = "transactions"

type GetCollection struct {
	GetByIDRequest
	ExpandsTransactions bool
}

// GetCollectionRequest extracts necessary variables from the provided request,
// builds a GetCollection instance, and validates it.
//
// No errors are expected during normal operation.
func GetCollectionRequest(r *common.Request) (GetCollection, error) {
	var req GetCollection
	err := req.Build(r)
	return req, err
}

func (g *GetCollection) Build(r *common.Request) error {
	err := g.GetByIDRequest.Build(r)
	g.ExpandsTransactions = r.Expands(ExpandsTransactions)

	return err
}
