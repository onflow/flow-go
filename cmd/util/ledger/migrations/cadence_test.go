package migrations

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

func TestMigrateCadence1EmptyContract(t *testing.T) {

	t.Parallel()

	const chainID = flow.Testnet

	address, err := common.HexToAddress("0x4184b8bdf78db9eb")
	require.NoError(t, err)

	const contractName = "FungibleToken"

	registersByAccount := registers.NewByAccount()

	err = registersByAccount.Set(
		string(address[:]),
		flow.ContractKey(contractName),
		// some whitespace for testing purposes
		[]byte(" \t  \n  "),
	)
	require.NoError(t, err)

	encodedContractNames, err := environment.EncodeContractNames([]string{contractName})
	require.NoError(t, err)

	err = registersByAccount.Set(
		string(address[:]),
		flow.ContractNamesKey,
		encodedContractNames,
	)
	require.NoError(t, err)

	programs := map[common.Location]*interpreter.Program{}

	rwf := &testReportWriterFactory{}

	// Run contract checking migration

	log := zerolog.Nop()
	checkingMigration := NewContractCheckingMigration(log, rwf, chainID, false, nil, programs)

	err = checkingMigration(registersByAccount)
	require.NoError(t, err)

	reporter := rwf.reportWriters[contractCheckingReporterName]
	assert.Empty(t, reporter.entries)

	// Initialize metrics collecting migration (used to run into unexpected error)

	metricsCollectingMigration := NewMetricsCollectingMigration(log, chainID, rwf, programs)

	err = metricsCollectingMigration.InitMigration(log, registersByAccount, 1)
	require.NoError(t, err)
}
