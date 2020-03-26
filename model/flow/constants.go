package flow

import (
	"time"
)

const (
	DefaultChainID = "flow"
)

func GenesisTime() time.Time {
	return time.Date(2018, time.December, 19, 22, 32, 30, 42, time.UTC)
}
