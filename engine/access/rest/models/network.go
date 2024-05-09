package models

import (
	"github.com/onflow/flow-go/access"
)

func (t *NetworkParameters) Build(params *access.NetworkParameters) {
	t.ChainId = params.ChainID.String()
}
