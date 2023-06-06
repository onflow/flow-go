package environment_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func Test_accountLocalIDGenerator_GenerateAccountID(t *testing.T) {
	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		meter := envMock.NewMeter(t)
		meter.On(
			"MeterComputation",
			common.ComputationKind(environment.ComputationKindGenerateAccountLocalID),
			uint(1),
		).Return(nil)

		accounts := envMock.NewAccounts(t)
		accounts.On("GenerateAccountLocalID", flow.ConvertAddress(address)).
			Return(uint64(1), nil)

		generator := environment.NewAccountLocalIDGenerator(
			tracing.NewMockTracerSpan(),
			meter,
			accounts,
		)

		id, err := generator.GenerateAccountID(address)
		require.NoError(t, err)
		require.Equal(t, uint64(1), id)
	})
	t.Run("error in meter", func(t *testing.T) {
		expectedErr := errors.New("error in meter")

		meter := envMock.NewMeter(t)
		meter.On(
			"MeterComputation",
			common.ComputationKind(environment.ComputationKindGenerateAccountLocalID),
			uint(1),
		).Return(expectedErr)

		accounts := envMock.NewAccounts(t)

		generator := environment.NewAccountLocalIDGenerator(
			tracing.NewMockTracerSpan(),
			meter,
			accounts,
		)

		_, err := generator.GenerateAccountID(address)
		require.ErrorIs(t, err, expectedErr)
	})
	t.Run("err in accounts", func(t *testing.T) {
		expectedErr := errors.New("error in accounts")

		meter := envMock.NewMeter(t)
		meter.On(
			"MeterComputation",
			common.ComputationKind(environment.ComputationKindGenerateAccountLocalID),
			uint(1),
		).Return(nil)

		accounts := envMock.NewAccounts(t)
		accounts.On("GenerateAccountLocalID", flow.ConvertAddress(address)).
			Return(uint64(0), expectedErr)

		generator := environment.NewAccountLocalIDGenerator(
			tracing.NewMockTracerSpan(),
			meter,
			accounts,
		)

		_, err := generator.GenerateAccountID(address)
		require.ErrorIs(t, err, expectedErr)
	})
}
