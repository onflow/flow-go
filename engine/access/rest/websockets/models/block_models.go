package models

import (
	"github.com/onflow/flow-go/model/flow"
)

type BlockMessageResponse struct {
	Block *flow.Block `json:"block"`
}
