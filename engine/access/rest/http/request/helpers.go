package request

import (
	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/model/flow"
)

type GetByIDRequest struct {
	ID flow.Identifier
}

func (g *GetByIDRequest) Build(r *common.Request) error {
	return g.Parse(
		r.GetVar(idQuery),
	)
}

func (g *GetByIDRequest) Parse(rawID string) error {
	var id parser.ID
	err := id.Parse(rawID)
	if err != nil {
		return err
	}
	g.ID = id.Flow()

	return nil
}
