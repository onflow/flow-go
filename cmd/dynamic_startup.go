package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

const sleepInterval = time.Minute

func DynamicStartup(logger zerolog.Logger, initANAddress, initANPubkey string, startupEpoch uint64, startupEpochPhase flow.EpochPhase, rootSnapshotOutDir string) error {

	// get flow client config that connects to our trusted access node
	config, err := common.NewFlowClientConfig(initANAddress, initANPubkey, false)
	if err != nil {
		return fmt.Errorf("failed to create flow client config for node dynamic startup pre-init: %w", err)
	}

	flowClient, err := common.FlowClient(config)
	if err != nil {
		return fmt.Errorf("failed to create flow client for node dynamic startup pre-init: %w", err)
	}

	ctx := context.Background()
	start := time.Now()
	var snapshot *inmem.Snapshot

	// wait until we reach the desired Epoch and EpochPhase
	for {
		b, err := flowClient.GetLatestProtocolStateSnapshot(ctx)
		if err != nil {
			return fmt.Errorf("failed to get latest finalized protocol state snapshot during pre-initialization: %w", err)
		}

		var snapshotEnc inmem.EncodableSnapshot
		err = json.Unmarshal(b, &snapshotEnc)
		if err != nil {
			return fmt.Errorf("failed to unmarshal protocol state snapshot: %w", err)
		}

		snapshot = inmem.SnapshotFromEncodable(snapshotEnc)

		currEpochCounter, err := snapshot.Epochs().Current().Counter()
		if err != nil {
			return fmt.Errorf("failed to get the current epoch counter: %w", err)
		}

		currEpochPhase, err := snapshot.Phase()
		if err != nil {
			return fmt.Errorf("failed to get the current epoch phase: %w", err)
		}

		if currEpochCounter == startupEpoch && currEpochPhase == startupEpochPhase {
			logger.Info().
				Str("time-waiting", time.Since(start).String()).
				Str("current-epoch", fmt.Sprintf("%d", currEpochCounter)).
				Str("current-epoch-phase", currEpochPhase.String()).
				Msg("reached desired epoch and phase in dynamic startup pre-init")

			break
		}

		logger.Info().
			Str("time-waiting", time.Since(start).String()).
			Str("current-epoch", fmt.Sprintf("%d", currEpochCounter)).
			Str("current-epoch-phase", currEpochPhase.String()).
			Msg(fmt.Sprintf("waiting for epoch %d and phase %s", startupEpoch, startupEpochPhase.String()))

		time.Sleep(sleepInterval)
	}

	logger.Info().Str("outdir", rootSnapshotOutDir).Msg("writing root snapshot file")
	return writeRootSnapshotFile(snapshot, rootSnapshotOutDir)
}

func writeRootSnapshotFile(snap *inmem.Snapshot, outDir string) error {
	err := writeJSON(outDir, snap.Encodable())
	if err != nil {
		return fmt.Errorf("failed to write root snapshot file: %w", err)
	}

	return nil
}

func writeJSON(path string, data interface{}) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	marshaled, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, marshaled, 0644)
}
