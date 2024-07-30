package common

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/utils/logging"

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
// Args:
// - ctx: context used when getting the snapshot from the network.
// - log: the logger
// - startupEpoch: the desired epoch in which to take a snapshot for startup.
// - startupEpochPhase: the desired epoch phase in which to take a snapshot for startup.
// - retryInterval: sleep interval used to retry getting the snapshot from the network in our desired epoch and epoch phase.
// - getSnapshot: func used to get the snapshot.
// Returns:
// - protocol.Snapshot: the protocol snapshot.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
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

		currEpochPhase, err := snapshot.EpochPhase()
		if err != nil {
			return fmt.Errorf("failed to get the current epoch phase: %w", err)
		}

		if shouldStartAtEpochPhase(currEpochCounter, startupEpoch, currEpochPhase, startupEpochPhase) {
			head, err := snapshot.Head()
			if err != nil {
				return fmt.Errorf("could not get Dynamic Startup snapshot header: %w", err)
			}
			log.Info().
				Dur("time_waiting", time.Since(start)).
				Uint64("current_epoch", currEpochCounter).
				Str("current_epoch_phase", currEpochPhase.String()).
				Hex("finalized_root_block_id", logging.ID(head.ID())).
				Uint64("finalized_block_height", head.Height).
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

// shouldStartAtEpochPhase determines whether Dynamic Startup should start up the node, based on a
// target epoch/phase and a current epoch/phase.
func shouldStartAtEpochPhase(currentEpoch, targetEpoch uint64, currentPhase, targetPhase flow.EpochPhase) bool {
	// if the current epoch is after the target epoch, start up regardless of phase
	if currentEpoch > targetEpoch {
		return true
	}
	// if the current epoch is before the target epoch, do not start up regardless of phase
	if currentEpoch < targetEpoch {
		return false
	}
	// if the target phase is EpochPhaseFallback, only start up if the current phase exactly matches
	if targetPhase == flow.EpochPhaseFallback {
		return currentPhase == flow.EpochPhaseFallback
	}
	// for any other target phase, start up if current phase is >= target
	return currentPhase >= targetPhase
}
