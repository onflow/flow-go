package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go-sdk/access/grpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

const getSnapshotTimeout = 30 * time.Second

// GetProtocolSnapshot callback that will get latest finalized protocol snapshot
type GetProtocolSnapshot func(ctx context.Context) (protocol.Snapshot, error)

// GetSnapshot will attempt to get the latest finalized protocol snapshot with the given flow configs
func GetSnapshot(ctx context.Context, client *grpc.Client) (*inmem.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, getSnapshotTimeout)
	defer cancel()

	b, err := client.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest finalized protocol state snapshot during pre-initialization: %w", err)
	}

	var snapshotEnc inmem.EncodableSnapshot
	err = json.Unmarshal(b, &snapshotEnc)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal protocol state snapshot: %w", err)
	}

	snapshot := inmem.SnapshotFromEncodable(snapshotEnc)
	return snapshot, nil
}

// GetSnapshotAtEpochAndPhase will get the latest finalized protocol snapshot and check the current epoch and epoch phase.
// If we are past the target epoch and epoch phase we exit the retry mechanism immediately.
// If not check the snapshot at the specified interval until we reach the target epoch and phase.
func GetSnapshotAtEpochAndPhase(ctx context.Context, log zerolog.Logger, startupEpoch uint64, startupEpochPhase flow.EpochPhase, retryInterval time.Duration, getSnapshot GetProtocolSnapshot) (protocol.Snapshot, error) {
	start := time.Now()

	log = log.With().
		Uint64("target_epoch_counter", startupEpoch).
		Str("target_epoch_phase", startupEpochPhase.String()).
		Logger()

	log.Info().Msg("starting dynamic startup - waiting until target epoch/phase to start...")

	var snapshot protocol.Snapshot
	var err error

	backoff := retry.NewConstant(retryInterval)
	err = retry.Do(ctx, backoff, func(ctx context.Context) error {
		snapshot, err = getSnapshot(ctx)
		if err != nil {
			err = fmt.Errorf("failed to get protocol snapshot: %w", err)
			log.Error().Err(err).Msg("could not get protocol snapshot")
			return retry.RetryableError(err)
		}

		// if we encounter any errors interpreting the snapshot something went wrong stop retrying
		currEpochCounter, err := snapshot.Epochs().Current().Counter()
		if err != nil {
			return fmt.Errorf("failed to get the current epoch counter: %w", err)
		}

		currEpochPhase, err := snapshot.Phase()
		if err != nil {
			return fmt.Errorf("failed to get the current epoch phase: %w", err)
		}

		// check if we are in or past the target epoch and phase
		if currEpochCounter > startupEpoch || (currEpochCounter == startupEpoch && currEpochPhase >= startupEpochPhase) {
			log.Info().
				Dur("time-waiting", time.Since(start)).
				Uint64("current-epoch", currEpochCounter).
				Str("current-epoch-phase", currEpochPhase.String()).
				Msg("finished dynamic startup - reached desired epoch and phase")

			return nil
		}

		// wait then poll for latest snapshot again
		log.Info().
			Dur("time-waiting", time.Since(start)).
			Uint64("current-epoch", currEpochCounter).
			Str("current-epoch-phase", currEpochPhase.String()).
			Msgf("waiting for epoch %d and phase %s", startupEpoch, startupEpochPhase.String())

		return retry.RetryableError(fmt.Errorf("dynamic startup epoch and epoch phase not reached"))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for target epoch and phase: %w", err)
	}

	return snapshot, nil
}
