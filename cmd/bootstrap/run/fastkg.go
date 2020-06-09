package run

import (
	"github.com/dapperlabs/flow-go/crypto"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
)

// RunFastKG is an alternative to RunDKG that runs much faster by using centralized threshold signature key generation.
func RunFastKG(n int, seed []byte) (model.DKGData, error) {

	skShares, pkShares, pkGroup, err := crypto.ThresholdSignKeyGen(int(n), seed)
	if err != nil {
		return model.DKGData{}, err
	}

	dkgData := model.DKGData{
		PrivKeyShares: skShares,
		PubGroupKey:   pkGroup,
		PubKeyShares:  pkShares,
	}

	return dkgData, nil
}
