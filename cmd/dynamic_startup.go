package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	utilsio "github.com/onflow/flow-go/utils/io"
)

// ValidateDynamicStartupFlags will validate flags necessary for dynamic node startup
// - assert dynamic-startup-access-publickey  is valid ECDSA_P256 public key hex
// - assert dynamic-startup-access-address is not empty
// - assert dynamic-startup-startup-epoch-phase is > 0 (EpochPhaseUndefined)
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
		return fmt.Errorf("invalid flag --dynamic-startup-startup-epoch-phase unknown epoch phase")
	}

	return nil
}

// DynamicStartPreInit is the pre-init func that will check if a node has already bootstrapped
// from a root protocol snapshot. If not attempt to get a protocol snapshot where the following
// conditions are met.
//  1. Target epoch < current epoch (in the past), set root snapshot to current snapshot
//  2. Target epoch == "current", wait until target phase == current phase before setting root snapshot
//  3. Target epoch > current epoch (in future), wait until target epoch and target phase is reached before
//
// setting root snapshot
func DynamicStartPreInit(nodeConfig *NodeConfig) error {
	ctx := context.Background()

	log := nodeConfig.Logger.With().Str("component", "dynamic-startup").Logger()

	// CASE 1: The state is already bootstrapped - nothing to do
	isBootstrapped, err := badgerstate.IsBootstrapped(nodeConfig.DB)
	if err != nil {
		return fmt.Errorf("could not check if state is boostrapped: %w", err)
	}
	if isBootstrapped {
		log.Debug().Msg("protocol state already bootstrapped, skipping dynamic startup")
		return nil
	}

	// CASE 2: The state is not already bootstrapped.
	// We will either bootstrap from a file or using Dynamic Startup.
	rootSnapshotPath := filepath.Join(nodeConfig.BootstrapDir, bootstrap.PathRootProtocolStateSnapshot)
	rootSnapshotFileExists := utilsio.FileExists(rootSnapshotPath)
	dynamicStartupFlagsSet := anyDynamicStartupFlagsAreSet(nodeConfig)

	// If the user has provided both a root snapshot file AND dynamic startup specification, return an error.
	// Previously, the snapshot file would take precedence over the Dynamic Startup flags.
	// This caused operators to inadvertently bootstrap from an old snapshot file when attempting to use Dynamic Startup.
	// Therefore, we instead require the operator to explicitly choose one option or the other.
	if rootSnapshotFileExists && dynamicStartupFlagsSet {
		return fmt.Errorf("must specify either a root snapshot file (%s) or Dynamic Startup flags (--dynamic-startup-*) but not both", rootSnapshotPath)
	}

	// CASE 2.1: Use the root snapshot file to bootstrap.
	if rootSnapshotFileExists {
		log.Debug().
			Str("root_snapshot_path", rootSnapshotPath).
			Msg("protocol state is not bootstrapped, will bootstrap using configured root snapshot file, skipping dynamic startup")
		return nil
	}

	// CASE 2.2: Use Dynamic Startup to bootstrap.

	// get flow client with secure client connection to download protocol snapshot from access node
	config, err := common.NewFlowClientConfig(nodeConfig.DynamicStartupANAddress, nodeConfig.DynamicStartupANPubkey, flow.ZeroID, false)
	if err != nil {
		return fmt.Errorf("failed to create flow client config for node dynamic startup pre-init: %w", err)
	}
	flowClient, err := common.FlowClient(config)
	if err != nil {
		return fmt.Errorf("failed to create flow client for node dynamic startup pre-init: %w", err)
	}
	getSnapshotFunc := func(ctx context.Context) (protocol.Snapshot, error) {
		return common.GetSnapshot(ctx, flowClient)
	}

	// validate dynamic startup epoch flag
	startupEpoch, err := validateDynamicStartEpochFlags(ctx, getSnapshotFunc, nodeConfig.DynamicStartupEpoch)
	if err != nil {
		return fmt.Errorf("failed to validate flag --dynamic-start-epoch: %w", err)
	}
	startupPhase := flow.GetEpochPhase(nodeConfig.DynamicStartupEpochPhase)

	// validate the rest of the dynamic startup flags
	err = ValidateDynamicStartupFlags(nodeConfig.DynamicStartupANPubkey, nodeConfig.DynamicStartupANAddress, startupPhase)
	if err != nil {
		return err
	}

	snapshot, err := common.GetSnapshotAtEpochAndPhase(
		ctx,
		log,
		startupEpoch,
		startupPhase,
		nodeConfig.BaseConfig.DynamicStartupSleepInterval,
		getSnapshotFunc,
	)
	if err != nil {
		return fmt.Errorf("failed to get snapshot at start up epoch (%d) and phase (%s): %w", startupEpoch, startupPhase.String(), err)
	}

	// set the root snapshot in the config - we will use this later to bootstrap
	nodeConfig.RootSnapshot = snapshot
	return nil
}

// anyDynamicStartupFlagsAreSet returns true if either the AN address or AN public key for Dynamic Startup are set.
// All other Dynamic Startup flags have default values (and aren't required) hence they aren't checked here.
// Both these flags must be set for Dynamic Startup to occur.
func anyDynamicStartupFlagsAreSet(config *NodeConfig) bool {
	if len(config.DynamicStartupANAddress) > 0 || len(config.DynamicStartupANPubkey) > 0 {
		return true
	}
	return false
}

// validateDynamicStartEpochFlags parse the start epoch flag and return the uin64 value,
// if epoch = current return the current epoch counter
func validateDynamicStartEpochFlags(ctx context.Context, getSnapshot common.GetProtocolSnapshot, flagEpoch string) (uint64, error) {

	// if flag is not `current` sentinel, it must be a specific epoch counter (uint64)
	if flagEpoch != "current" {
		epochCounter, err := strconv.ParseUint(flagEpoch, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid epoch counter flag (%s): %w", flagEpoch, err)
		}
		return epochCounter, nil
	}

	// we are using the current epoch, retrieve latest snapshot to determine this value
	snapshot, err := getSnapshot(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get snapshot: %w", err)
	}

	epochCounter, err := snapshot.Epochs().Current().Counter()
	if err != nil {
		return 0, fmt.Errorf("failed to get current epoch counter: %w", err)
	}

	return epochCounter, nil
}
