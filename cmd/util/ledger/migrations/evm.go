package migrations

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func NewEVMDeploymentMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
	full bool,
) RegistersMigration {

	systemContracts := systemcontracts.SystemContractsForChain(chainID)
	address := systemContracts.EVMContract.Address

	var code []byte
	if full {
		code = stdlib.ContractCode(
			systemContracts.NonFungibleToken.Address,
			systemContracts.FungibleToken.Address,
			systemContracts.FlowToken.Address,
		)
	} else {
		code = []byte(stdlib.ContractMinimalCode)
	}

	return NewDeploymentMigration(
		chainID,
		Contract{
			Name: systemContracts.EVMContract.Name,
			Code: code,
		},
		address,
		map[flow.Address]struct{}{
			address: {},
		},
		logger,
	)
}

func NewEVMSetupMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
) RegistersMigration {

	chain := chainID.Chain()

	return func(registersByAccount *registers.ByAccount) error {

		tracerSpan := tracing.NewTracerSpan()

		transactionInfoParams := environment.DefaultTransactionInfoParams()
		transactionInfoParams.TxBody = &flow.TransactionBody{}

		environmentParams := environment.EnvironmentParams{
			Chain:                 chain,
			RuntimeParams:         environment.DefaultRuntimeParams(),
			ProgramLoggerParams:   environment.DefaultProgramLoggerParams(),
			TransactionInfoParams: transactionInfoParams,
		}

		basicMigrationRuntime := NewBasicMigrationRuntime(registersByAccount)

		transactionPreparer, err := util.NewTransactionPreparer(basicMigrationRuntime.TransactionState)
		if err != nil {
			return fmt.Errorf("failed to create transaction preparer: %w", err)
		}

		transactionEnvironment := environment.NewTransactionEnvironment(
			tracerSpan,
			environmentParams,
			transactionPreparer,
		)

		runtime := environment.NewRuntime(environmentParams.RuntimeParams)
		runtime.SetEnvironment(transactionEnvironment)

		programLogger := environment.NewProgramLogger(
			tracerSpan,
			environmentParams.ProgramLoggerParams,
		)
		systemContracts := environment.NewSystemContracts(
			chain,
			tracerSpan,
			programLogger,
			runtime,
		)

		_, err = systemContracts.Invoke(
			environment.ContractFunctionSpec{
				AddressFromChain: environment.ServiceAddress,
				LocationName:     systemcontracts.ContractNameEVM,
				FunctionName:     "setupHeartbeat",
			},
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to setup EVM heartbeat: %w", err)
		}

		return nil
	}
}
