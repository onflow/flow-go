// +build !relic

package signature

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module"
)

// NewRandomBeaconSigner is needed to allow the code to compile without relic build,
// See module/signature/random_beacon.go for implementation for relic build
func NewRandomBeaconSigner(_ string, _ crypto.PrivateKey) module.Signer {
	panic("NewRandomBeaconSigner not supported with non-relic build")
}

func NewStakingSigner(_ string, _ module.Local) module.Signer {
	panic("NewStakingSigner not supported with non-relic build")
}
