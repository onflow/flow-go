package generate_authorization_fixes

import (
	"encoding/json"
	"strings"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/pretty"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
)

func checkContract(
	contract AddressContract,
	mr *migrations.InterpreterMigrationRuntime,
	contractsForPrettyPrinting map[common.Location][]byte,
	reporter reporters.ReportWriter,
	programs map[common.Location]*interpreter.Program,
) {
	location := contract.Location
	code := contract.Code

	log.Info().Msgf("checking contract %s ...", location)

	// Check contract code
	const getAndSetProgram = true
	program, err := mr.ContractAdditionHandler.ParseAndCheckProgram(code, location, getAndSetProgram)
	if err != nil {

		// Pretty print the error
		var builder strings.Builder
		errorPrinter := pretty.NewErrorPrettyPrinter(&builder, false)

		printErr := errorPrinter.PrettyPrintError(err, location, contractsForPrettyPrinting)

		var errorDetails string
		if printErr == nil {
			errorDetails = builder.String()
		} else {
			errorDetails = err.Error()
		}

		log.Error().Msgf(
			"error checking contract %s: %s",
			location,
			errorDetails,
		)

		reporter.Write(contractCheckingFailure{
			AccountAddress: location.Address,
			ContractName:   location.Name,
			Code:           string(code),
			Error:          errorDetails,
		})

		return
	}

	// Record the checked program for future use
	programs[location] = program

	reporter.Write(contractCheckingSuccess{
		AccountAddress: location.Address,
		ContractName:   location.Name,
		Code:           string(code),
	})

	log.Info().Msgf("finished checking contract %s", location)
}

type contractCheckingFailure struct {
	AccountAddress common.Address
	ContractName   string
	Code           string
	Error          string
}

var _ json.Marshaler = contractCheckingFailure{}

func (e contractCheckingFailure) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"address"`
		ContractName   string `json:"name"`
		Code           string `json:"code"`
		Error          string `json:"error"`
	}{
		Kind:           "checking-failure",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		ContractName:   e.ContractName,
		Code:           e.Code,
		Error:          e.Error,
	})
}

type contractCheckingSuccess struct {
	AccountAddress common.Address
	ContractName   string
	Code           string
}

var _ json.Marshaler = contractCheckingSuccess{}

func (e contractCheckingSuccess) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"address"`
		ContractName   string `json:"name"`
		Code           string `json:"code"`
	}{
		Kind:           "checking-success",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		ContractName:   e.ContractName,
		Code:           e.Code,
	})
}
