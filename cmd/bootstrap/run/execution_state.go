package run

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

// NOTE: this is now unused and should become part of another tool.
func GenerateServiceAccountPrivateKey(seed []byte) (flow.AccountPrivateKey, error) {
	priv, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: priv,
		SignAlgo:   crypto.ECDSASecp256k1,
		HashAlgo:   hash.SHA2_256,
	}, nil
}

// NOTE: this is now unused and should become part of another tool.
func GenerateExecutionState(
	dbDir string,
	accountKey flow.AccountPublicKey,
	chain flow.Chain,
	bootstrapOptions ...fvm.BootstrapProcedureOption,
) (flow.StateCommitment, error) {
	metricsCollector := &metrics.NoopCollector{}

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, metricsCollector, dbDir, 100, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return flow.DummyStateCommitment, err
	}
	defer func() {
		<-diskWal.Done()
	}()

	ledgerStorage, err := ledger.NewLedger(diskWal, 100, metricsCollector, zerolog.Nop(), ledger.DefaultPathFinderVersion)
	if err != nil {
		return flow.DummyStateCommitment, err
	}

	return bootstrap.NewBootstrapper(
		zerolog.Nop()).BootstrapLedger(
		ledgerStorage,
		accountKey,
		chain,
		bootstrapOptions...,
	)
}
