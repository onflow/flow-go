package handler_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func BenchmarkStorage(b *testing.B) { benchmarkStorageGrowth(b, 100, 100) }

// TODO: fix me
// benchmark
func benchmarkStorageGrowth(b *testing.B, accountCount, setupKittyCount int) {
	testutils.RunWithTestBackend(b, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(b, backend, func(rootAddr flow.Address) {
			testutils.RunWithDeployedContract(b,
				testutils.GetDummyKittyTestContract(b),
				backend,
				rootAddr,
				func(tc *testutils.TestContract) {
					handler := SetupHandler(b, backend, rootAddr)
					numOfAccounts := 100000
					accounts := make([]types.Account, numOfAccounts)
					// setup several of accounts
					// note that trie growth is the function of number of accounts
					for i := 0; i < numOfAccounts; i++ {
						account := handler.AccountByAddress(handler.AllocateAddress(), true)
						account.Deposit(types.NewFlowTokenVault(types.Balance(100)))
						accounts[i] = account
					}
					backend.DropEvents()
					// mint kitties
					for i := 0; i < setupKittyCount; i++ {
						account := accounts[i%accountCount]
						matronId := testutils.RandomBigInt(1000)
						sireId := testutils.RandomBigInt(1000)
						generation := testutils.RandomBigInt(1000)
						genes := testutils.RandomBigInt(1000)
						require.NotNil(b, account)
						account.Call(
							tc.DeployedAt,
							tc.MakeCallData(b,
								"CreateKitty",
								matronId,
								sireId,
								generation,
								genes,
							),
							300_000_000,
							types.Balance(0),
						)
						require.Equal(b, 2, len(backend.Events()))
						backend.DropEvents() // this would make things lighter
					}

					// measure the impact of mint after the setup phase
					// db.ResetReporter()
					// db.DropCache()

					accounts[0].Call(
						tc.DeployedAt,
						tc.MakeCallData(b,
							"CreateKitty",
							testutils.RandomBigInt(1000),
							testutils.RandomBigInt(1000),
							testutils.RandomBigInt(1000),
							testutils.RandomBigInt(1000),
						),
						300_000_000,
						types.Balance(0),
					)

					// b.ReportMetric(float64(db.BytesRetrieved()), "bytes_read")
					// b.ReportMetric(float64(db.BytesStored()), "bytes_written")
					b.ReportMetric(float64(backend.TotalStorageSize()), "total_storage_size")
				})
		})
	})
}
