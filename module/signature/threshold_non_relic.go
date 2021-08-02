// +build !relic

package signature

import (
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/module"
)

func NewThresholdProvider(_ string, _ encodable.RandomBeaconPrivKey) module.ThresholdSigner {
	panic("")
}

func EnoughThresholdShares(size int, shares int) (bool, error) {
	panic("")
}
