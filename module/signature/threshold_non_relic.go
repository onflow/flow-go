// +build !relic

package signature

import (
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/module"
)

func NewThresholdProvider(_ string, _ encodable.RandomBeaconPrivKey) module.ThresholdSigner {
	panic("NewThresholdProvider not supported with non-relic build")
}

func EnoughThresholdShares(_ int, _ int) (bool, error) {
	panic("EnoughThresholdShares not supported with non-relic build")
}
