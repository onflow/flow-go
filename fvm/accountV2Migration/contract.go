package accountV2Migration

import (
	_ "embed"

	"github.com/onflow/cadence/common"
	cadenceErrors "github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed AccountV2Migration.cdc
var ContractCode []byte

const ContractName = "AccountV2Migration"

const scheduleAccountV2MigrationFunctionName = "scheduleAccountV2Migration"

// scheduleAccountV2MigrationType is the type of the `scheduleAccountV2Migration` function.
// This defines the signature as `func(addressStartIndex: UInt64, count: UInt64): Bool`
var scheduleAccountV2MigrationType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier:     "addressStartIndex",
			TypeAnnotation: sema.UInt64TypeAnnotation,
		},
		{
			Identifier:     "count",
			TypeAnnotation: sema.UInt64TypeAnnotation,
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.BoolType),
}

func DeclareFunctions(environment runtime.Environment, chainID flow.ChainID) {
	declareScheduleAccountV2MigrationFunction(environment, chainID)
	declareGetAccountStorageFormatFunction(environment, chainID)
}

func declareScheduleAccountV2MigrationFunction(environment runtime.Environment, chainID flow.ChainID) {

	functionType := scheduleAccountV2MigrationType

	functionValue := stdlib.StandardLibraryValue{
		Name: scheduleAccountV2MigrationFunctionName,
		Type: functionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			functionType,
			func(invocation interpreter.Invocation) interpreter.Value {
				inter := invocation.Interpreter

				// Get interpreter storage

				storage := inter.Storage()

				runtimeStorage, ok := storage.(*runtime.Storage)
				if !ok {
					panic(cadenceErrors.NewUnexpectedError("interpreter storage is not a runtime.Storage"))
				}

				// Check the number of arguments

				actualArgumentCount := len(invocation.Arguments)
				expectedArgumentCount := len(functionType.Parameters)

				if actualArgumentCount != expectedArgumentCount {
					panic(errors.NewInvalidArgumentErrorf(
						"incorrect number of arguments: got %d, expected %d",
						actualArgumentCount,
						expectedArgumentCount,
					))
				}

				// Get addressStartIndex argument

				firstArgument := invocation.Arguments[0]
				addressStartIndexValue, ok := firstArgument.(interpreter.UInt64Value)
				if !ok {
					panic(errors.NewInvalidArgumentErrorf(
						"incorrect type for argument 0: got `%s`, expected `%s`",
						firstArgument.StaticType(inter),
						sema.UInt64Type,
					))
				}
				addressStartIndex := uint64(addressStartIndexValue)

				// Get count argument

				secondArgument := invocation.Arguments[1]
				countValue, ok := secondArgument.(interpreter.UInt64Value)
				if !ok {
					panic(errors.NewInvalidArgumentErrorf(
						"incorrect type for argument 1: got `%s`, expected `%s`",
						secondArgument.StaticType(inter),
						sema.UInt64Type,
					))
				}
				count := uint64(countValue)

				// Schedule the account V2 migration for addresses

				addressGenerator := chainID.Chain().NewAddressGeneratorAtIndex(addressStartIndex)
				for i := uint64(0); i < count; i++ {
					address := addressGenerator.CurrentAddress()
					if !runtimeStorage.ScheduleV2Migration(common.Address(address)) {
						return interpreter.FalseValue
					}

					_, err := addressGenerator.NextAddress()
					if err != nil {
						panic(err)
					}
				}

				return interpreter.TrueValue
			},
		),
	}

	sc := systemcontracts.SystemContractsForChain(chainID)

	accountV2MigrationLocation := common.NewAddressLocation(
		nil,
		common.Address(sc.AccountV2Migration.Address),
		ContractName,
	)

	environment.DeclareValue(
		functionValue,
		accountV2MigrationLocation,
	)
}

const getAccountStorageFormatFunctionName = "getAccountStorageFormat"

// getAccountStorageFormatType is the type of the `getAccountStorageFormat` function.
// This defines the signature as `func(address: Address): UInt8`
var getAccountStorageFormatType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier:     "address",
			TypeAnnotation: sema.AddressTypeAnnotation,
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UInt8Type),
}

func declareGetAccountStorageFormatFunction(environment runtime.Environment, chainID flow.ChainID) {

	functionType := getAccountStorageFormatType

	functionValue := stdlib.StandardLibraryValue{
		Name: getAccountStorageFormatFunctionName,
		Type: functionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			functionType,
			func(invocation interpreter.Invocation) interpreter.Value {
				inter := invocation.Interpreter

				// Get interpreter storage

				storage := inter.Storage()

				runtimeStorage, ok := storage.(*runtime.Storage)
				if !ok {
					panic(cadenceErrors.NewUnexpectedError("interpreter storage is not a runtime.Storage"))
				}

				// Check the number of arguments

				actualArgumentCount := len(invocation.Arguments)
				expectedArgumentCount := len(functionType.Parameters)

				if actualArgumentCount != expectedArgumentCount {
					panic(errors.NewInvalidArgumentErrorf(
						"incorrect number of arguments: got %d, expected %d",
						actualArgumentCount,
						expectedArgumentCount,
					))
				}

				// Get addressStartIndex argument

				firstArgument := invocation.Arguments[0]
				addressValue, ok := firstArgument.(interpreter.AddressValue)
				if !ok {
					panic(errors.NewInvalidArgumentErrorf(
						"incorrect type for argument 0: got `%s`, expected `%s`",
						firstArgument.StaticType(inter),
						sema.TheAddressType,
					))
				}
				address := common.Address(addressValue)

				// Get and return the storage format for the account

				return interpreter.UInt8Value(runtimeStorage.AccountStorageFormat(address))
			},
		),
	}

	sc := systemcontracts.SystemContractsForChain(chainID)

	accountV2MigrationLocation := common.NewAddressLocation(
		nil,
		common.Address(sc.AccountV2Migration.Address),
		ContractName,
	)

	environment.DeclareValue(
		functionValue,
		accountV2MigrationLocation,
	)
}

const MigratedEventTypeQualifiedIdentifier = ContractName + ".Migrated"
