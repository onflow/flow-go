package testutils

import (
	"math"
	"math/big"
	"strings"
	"testing"

	gethABI "github.com/ethereum/go-ethereum/accounts/abi"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/testutils/contracts"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type TestContract struct {
	ABI        string
	ByteCode   []byte
	DeployedAt types.Address
}

func MakeCallData(t testing.TB, abiString string, name string, args ...interface{}) []byte {
	abi, err := gethABI.JSON(strings.NewReader(abiString))
	require.NoError(t, err)
	call, err := abi.Pack(name, args...)
	require.NoError(t, err)
	return call
}

func (tc *TestContract) MakeCallData(t testing.TB, name string, args ...interface{}) []byte {
	return MakeCallData(t, tc.ABI, name, args...)
}

func (tc *TestContract) SetDeployedAt(deployedAt types.Address) {
	tc.DeployedAt = deployedAt
}

func GetStorageTestContract(tb testing.TB) *TestContract {
	return &TestContract{
		ABI:      contracts.TestContractABIJSON,
		ByteCode: contracts.TestContractBytes,
	}
}

func GetDummyKittyTestContract(t testing.TB) *TestContract {
	return &TestContract{
		ABI:      contracts.DummyKittyContractABIJSON,
		ByteCode: contracts.DummyKittyContractBytes,
	}
}

func RunWithDeployedContract(t testing.TB, tc *TestContract, led atree.Ledger, flowEVMRootAddress flow.Address, f func(*TestContract)) {
	DeployContract(t, types.NewAddress(gethCommon.Address{}), tc, led, flowEVMRootAddress)
	f(tc)
}

func DeployContract(t testing.TB, caller types.Address, tc *TestContract, led atree.Ledger, flowEVMRootAddress flow.Address) {
	// deploy contract
	e := emulator.NewEmulator(led, flowEVMRootAddress)

	ctx := types.NewDefaultBlockContext(2)

	bl, err := e.NewReadOnlyBlockView(ctx)
	require.NoError(t, err)

	nonce, err := bl.NonceOf(caller)
	require.NoError(t, err)

	blk, err := e.NewBlockView(ctx)
	require.NoError(t, err)

	_, err = blk.DirectCall(
		types.NewDepositCall(
			caller,
			new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)),
			nonce,
		),
	)
	require.NoError(t, err)

	blk2, err := e.NewBlockView(types.NewDefaultBlockContext(3))
	require.NoError(t, err)

	res, err := blk2.DirectCall(
		types.NewDeployCall(
			caller,
			tc.ByteCode,
			math.MaxUint64,
			big.NewInt(0),
			nonce+1,
		),
	)
	require.NoError(t, err)
	tc.SetDeployedAt(res.DeployedContractAddress)
}
