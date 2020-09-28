package run

import (
	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/signature"
)

// RunFastKG is an alternative to RunDKG that runs much faster by using centralized threshold signature key generation.
func RunFastKG(n int, seed []byte) (model.DKGData, error) {

	skShares, pkShares, pkGroup, err := crypto.ThresholdSignKeyGen(int(n),
		signature.RandomBeaconThreshold(int(n)), seed)
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
