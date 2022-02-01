package cmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/bootstrap"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/state/protocol"
	utilsio "github.com/onflow/flow-go/utils/io"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// GetProtocolSnapshot callback that will get latest finalized protocol snapshot
type GetProtocolSnapshot func() (protocol.Snapshot, error)

// GetSnapshot will attempt to get the latest finalized protocol snapshot with the given flow configs
func GetSnapshot(ctx context.Context, client *client.Client) (*inmem.Snapshot, error) {
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
func GetSnapshotAtEpochAndPhase(ctx context.Context, logger zerolog.Logger, startupEpoch uint64, startupEpochPhase flow.EpochPhase, retryInterval time.Duration, getSnapshot GetProtocolSnapshot) (protocol.Snapshot, error) {
	start := time.Now()
	var snapshot protocol.Snapshot

	constRetry, err := retry.NewConstant(retryInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry mechanism: %w", err)
	}

	err = retry.Do(ctx, constRetry, func(ctx context.Context) error {
		snapshot, err = getSnapshot()
		if err != nil {
			return retry.RetryableError(fmt.Errorf("failed to get protocol snapshot: %w", err))
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
			logger.Info().
				Dur("time-waiting", time.Since(start)).
				Uint64("current-epoch", currEpochCounter).
				Str("current-epoch-phase", currEpochPhase.String()).
				Msg("reached desired epoch and phase in dynamic startup pre-init")

			return nil
		}

		logger.Warn().
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

// ValidateDynamicStartupFlags will validate flags necessary for dynamic node startup
// - assert dynamic-startup-access-publickey  is valid ECDSA_P256 public key hex
// - assert dynamic-startup-access-address is not empty
// - assert dynamic-startup-startup-epoch-phase is > 0 (EpochPhaseUndefined)
// - assert dynamic-startup-startup-epoch is > 0
func ValidateDynamicStartupFlags(accessPublicKey, accessAddress string, startPhase flow.EpochPhase) error {
	b, err := hex.DecodeString(strings.TrimPrefix(accessPublicKey, "0x"))
	if err != nil {
		return fmt.Errorf("invalid flag --dynamic-startup-access-publickey: %w", err)
	}

	_, err = crypto.DecodePublicKey(crypto.ECDSAP256, b)
	if err != nil {
		return fmt.Errorf("invalid flag --dynamic-startup-access-publickey: %w", err)
	}

	if accessAddress == "" {
		return fmt.Errorf("invalid flag --dynamic-startup-access-address can not be empty")
	}

	if startPhase <= flow.EpochPhaseUndefined {
		return fmt.Errorf("invalid flag --dynamic-startup-startup-epoch-phase unknow epoch phase")
	}

	return nil
}

// DynamicStartPreInit is the pre-init func that will check if a node has already bootstrapped
// from a root protocol snapshot. If not attempt to get a protocol snapshot where the following
// conditions are met.
// 1. Target epoch < current epoch (in the past), set root snapshot to current snapshot
// 2. Target epoch == "current", wait until target phase == current phase before setting root snapshot
// 3. Target epoch > current epoch (in future), wait until target epoch and target phase is reached before
// setting root snapshot
func DynamicStartPreInit(nodeConfig *NodeConfig) error {
	ctx := context.Background()

	log := nodeConfig.Logger.With().Str("component", "dynamic-startup").Logger()

	rootSnapshotPath := filepath.Join(nodeConfig.BootstrapDir, bootstrap.PathRootProtocolStateSnapshot)

	// root snapshot exists skip dynamic start up
	if rootSnapshotExists(rootSnapshotPath) {
		return nil
	}

	// get flow clinet with secure client connection to download protocol snapshot from access node
	config, err := common.NewFlowClientConfig(nodeConfig.DynamicStartupANAddress, nodeConfig.DynamicStartupANPubkey, false)
	if err != nil {
		return fmt.Errorf("failed to create flow client config for node dynamic startup pre-init: %w", err)
	}

	flowClient, err := common.FlowClient(config)
	if err != nil {
		return fmt.Errorf("failed to create flow client for node dynamic startup pre-init: %w", err)
	}

	startupPhase := flow.GetEpochPhase(nodeConfig.BaseConfig.DynamicStartupEpochPhase)

	// validate dynamic startup epoch flag
	startupEpoch, err := validateDynamicStartEpochFlag(ctx, nodeConfig.DynamicStartupEpoch, flowClient)
	if err != nil {
		return fmt.Errorf("failed to validate flag --dynamic-start-epoch: %w", err)
	}

	// validate the rest of the dynamic startup flags
	err = ValidateDynamicStartupFlags(nodeConfig.BaseConfig.DynamicStartupANPubkey, nodeConfig.BaseConfig.DynamicStartupANAddress, startupPhase)
	if err != nil {
		return err
	}

	getSnapshotFunc := func() (protocol.Snapshot, error) {
		return GetSnapshot(ctx, flowClient)
	}

	log.Info().
		Str("startup_epoch", nodeConfig.BaseConfig.DynamicStartupEpoch).
		Str("startup_phase", startupPhase.String()).
		Msg("waiting until target epoch/phase to start...")

	snapshot, err := GetSnapshotAtEpochAndPhase(
		ctx,
		log,
		startupEpoch,
		startupPhase,
		nodeConfig.BaseConfig.DynamicStartupSleepInterval,
		getSnapshotFunc,
	)
	if err != nil {
		return fmt.Errorf("failed to get snapshot at start up epoch (%d) and phase (%s): %w", nodeConfig.BaseConfig.DynamicStartupEpoch, startupPhase.String(), err)
	}

	log.Info().Str("snapshot-path", rootSnapshotPath).Msg("writing root snapshot file")
	err = writeRootSnapshotFile(snapshot.(*inmem.Snapshot), rootSnapshotPath)
	if err != nil {
		return fmt.Errorf("failed to write root snapshot file: %w", err)
	}

	return nil
}

// validateDynamicStartEpochFlag parse the start epoch flag and return the uin64 value,
// if epoch = current return the current epoch counter
func validateDynamicStartEpochFlag(ctx context.Context, epoch string, client *client.Client) (uint64, error) {
	if epoch == "current" {
		snapshot, err := GetSnapshot(ctx, client)
		if err != nil {
			return 0, fmt.Errorf("failed to get snapshot: %w", err)
		}

		counter, err := snapshot.Epochs().Current().Counter()
		if err != nil {
			return 0, fmt.Errorf("failed to get current epoch counter: %w", err)
		}

		return counter, nil
	}

	counter, err := strconv.ParseUint(string("90"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse target epoch (%d): %w", counter, err)
	}

	return counter, nil
}

// rootSnapshotExists check if root snapshot file exists
func rootSnapshotExists(path string) bool {
	return utilsio.FileExists(path)
}

// writeRootSnapshotFile will write the snapshot to the path provided
func writeRootSnapshotFile(snapshot *inmem.Snapshot, path string) error {
	err := utilsio.WriteJSON(path, snapshot.Encodable())
	if err != nil {
		return fmt.Errorf("failed to write root snapshot file: %w", err)
	}

	return nil
}
