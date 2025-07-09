package models

import (
	accessmodel "github.com/onflow/flow-go/model/access"
)

func (t *NetworkParameters) Build(params *accessmodel.NetworkParameters) {
	t.ChainId = params.ChainID.String()
}
