package emulator

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func configureLedger(
	conf config,
	store EmulatorStorage,
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
) (
	*flowgo.Block,
	snapshot.StorageSnapshot,
	error,
) {

	latestBlock, err := store.LatestBlock(context.Background())
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, nil, err
	}

	if errors.Is(err, ErrNotFound) {
		// bootstrap the ledger with the genesis block
		ledger, err := store.LedgerByHeight(context.Background(), 0)
		if err != nil {
			return nil, nil, err
		}

		genesisExecutionSnapshot, err := bootstrapLedger(vm, ctx, ledger, conf)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to bootstrap execution state: %w", err)
		}

		// commit the genesis block to storage
		genesis := flowgo.Genesis(conf.GetChainID())
		latestBlock = *genesis

		err = store.CommitBlock(
			context.Background(),
			*genesis,
			nil,
			nil,
			nil,
			genesisExecutionSnapshot,
			nil,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	latestLedger, err := store.LedgerByHeight(
		context.Background(),
		latestBlock.Header.Height,
	)

	if err != nil {
		return nil, nil, err
	}

	return &latestBlock, latestLedger, nil
}

func bootstrapLedger(
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
	ledger snapshot.StorageSnapshot,
	conf config,
) (
	*snapshot.ExecutionSnapshot,
	error,
) {
	serviceKey := conf.GetServiceKey()

	ctx = fvm.NewContextFromParent(
		ctx,
		fvm.WithAccountStorageLimit(false),
	)

	flowAccountKey := flowgo.AccountPublicKey{
		PublicKey: serviceKey.PublicKey,
		SignAlgo:  serviceKey.SigAlgo,
		HashAlgo:  serviceKey.HashAlgo,
		Weight:    fvm.AccountKeyWeightThreshold,
	}

	bootstrap := configureBootstrapProcedure(conf, flowAccountKey, conf.GenesisTokenSupply)

	executionSnapshot, output, err := vm.Run(ctx, bootstrap, ledger)
	if err != nil {
		return nil, err
	}

	if output.Err != nil {
		return nil, output.Err
	}

	return executionSnapshot, nil
}

func configureBootstrapProcedure(conf config, flowAccountKey flowgo.AccountPublicKey, supply cadence.UFix64) *fvm.BootstrapProcedure {
	options := make([]fvm.BootstrapProcedureOption, 0)
	options = append(options,
		fvm.WithInitialTokenSupply(supply),
		fvm.WithRestrictedAccountCreationEnabled(false),
		// This enables variable transaction fees AND execution effort metering
		// as described in Variable Transaction Fees:
		// Execution Effort FLIP: https://github.com/onflow/flow/pull/753
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithExecutionMemoryLimit(math.MaxUint32),
		fvm.WithExecutionMemoryWeights(meter.DefaultMemoryWeights),
		fvm.WithExecutionEffortWeights(environment.MainnetExecutionEffortWeights),
	)

	if conf.ExecutionEffortWeights != nil {
		options = append(options,
			fvm.WithExecutionEffortWeights(conf.ExecutionEffortWeights),
		)
	}
	if conf.StorageLimitEnabled {
		options = append(options,
			fvm.WithAccountCreationFee(conf.MinimumStorageReservation),
			fvm.WithMinimumStorageReservation(conf.MinimumStorageReservation),
			fvm.WithStorageMBPerFLOW(conf.StorageMBPerFLOW),
		)
	}
	return fvm.Bootstrap(
		flowAccountKey,
		options...,
	)
}
