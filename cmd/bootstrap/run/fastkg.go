package run

import (
	"github.com/dapperlabs/flow-go/crypto"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
)

// RunFastKG is an alternative to RunDKG that runs much faster by using centralized threshold signature key generation.
func RunFastKG(n int, seed []byte) (model.DKGData, error) {

	beaconKeys, _, groupKey, err := crypto.ThresholdSignKeyGen(int(n), seed)
	if err != nil {
		return model.DKGData{}, err
	}

	dkgData := model.DKGData{
		Participants: make([]model.DKGParticipant, 0, n),
		PubGroupKey:  groupKey,
	}
	for i, key := range beaconKeys {
		dkgData.Participants = append(dkgData.Participants, model.DKGParticipant{
			KeyShare:   key,
			GroupIndex: i,
		})
	}

	return dkgData, nil
}
