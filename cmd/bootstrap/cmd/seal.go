package cmd

import (
	"encoding/hex"
	"time"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/flow"
)

func constructRootResultAndSeal(
	rootCommit string,
	block *flow.Block,
	epochSetup *flow.EpochSetup,
	epochCommit *flow.EpochCommit,
) (*flow.ExecutionResult, *flow.Seal) {

	stateCommitBytes, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}
	stateCommit, err := flow.ToStateCommitment(stateCommitBytes)
	if err != nil {
		log.Fatal().
			Int("expected_state_commitment_length", len(stateCommit)).
			Int("received_state_commitment_length", len(stateCommitBytes)).
			Msg("root state commitment has incompatible length")
	}

	result := run.GenerateRootResult(block, stateCommit, epochSetup, epochCommit)
	seal, err := run.GenerateRootSeal(result)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate root seal")
	}

	if seal.ResultID != result.ID() {
		log.Fatal().Msgf("root block seal (%v) mismatch with result id: (%v)", seal.ResultID, result.ID())
	}

	return result, seal
}

// rootEpochTargetEndTime computes the target end time for the given epoch, using the given config.
// CAUTION: the variables `flagEpochTimingRefCounter`, `flagEpochTimingDuration`, and
// `flagEpochTimingRefTimestamp` must contain proper values. You can either specify a value for
// each config parameter or use the function `validateOrPopulateEpochTimingConfig()` to populate the variables
// from defaults.
func rootEpochTargetEndTime() uint64 {
	if flagEpochTimingRefTimestamp == 0 || flagEpochTimingDuration == 0 {
		panic("invalid epoch timing config: must specify ALL of --epoch-target-end-time-ref-counter, --epoch-target-end-time-ref-timestamp, and --epoch-target-end-time-duration")
	}
	if flagEpochCounter < flagEpochTimingRefCounter {
		panic("invalid epoch timing config: reference epoch counter must be before root epoch counter")
	}
	targetEndTime := flagEpochTimingRefTimestamp + (flagEpochCounter-flagEpochTimingRefCounter)*flagEpochTimingDuration
	if targetEndTime <= uint64(time.Now().Unix()) {
		panic("sanity check failed: root epoch target end time is before current time")
	}
	return targetEndTime
}
