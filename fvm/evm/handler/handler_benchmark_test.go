package handler_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func BenchmarkStorage(b *testing.B) { benchmarkStorageGrowth(b, 10_000, 10_000) }

// benchmark
func benchmarkStorageGrowth(b *testing.B, accountCount, mintCount int) {
	testutils.RunWithTestBackend(b, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(b, backend, func(rootAddr flow.Address) {
			testutils.RunWithDeployedContract(b,
				testutils.GetDummyKittyTestContract(b),
				backend,
				rootAddr,
				func(tc *testutils.TestContract) {
					db, handler := SetupHandler(b, backend, rootAddr)
					numOfAccounts := 100000
					accounts := make([]types.Account, numOfAccounts)
					for i := 0; i < numOfAccounts; i++ {
						account := handler.AccountByAddress(handler.AllocateAddress(), true)
						account.Deposit(types.NewFlowTokenVault(types.Balance(100)))
						accounts[i] = account
					}
					backend.DropEvents()
					// random accounts reading the state
					for i := 0; i < mintCount; i++ {
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
						fmt.Println(i, ">> read:", db.BytesRetrieved(), " written:", db.BytesStored())
						db.ResetReporter()

						require.Equal(b, 2, len(backend.Events()))
						backend.DropEvents()

						db.DropCache()
					}

					fmt.Println("total storage size:", backend.TotalStorageSize())

					// secondAccount := handler.AccountByAddress(handler.AllocateAddress(), true)

					// account.Call(
					// 	tc.DeployedAt,
					// 	tc.MakeCallData(b,
					// 		"Transfer",
					// 		account.Address().ToCommon(),
					// 		secondAccount.Address().ToCommon(),
					// 		big.NewInt(100),
					// 	),
					// 	300_000_000,
					// 	types.Balance(0),
					// )
					// fmt.Println("random transfer", ">> read:", db.BytesRetrieved(), " written:", db.BytesStored())

					// TODO add random transfers (to evaluate the assumptions)

					b.Fatal("XXX")
				})

			// TODO check events
		})
	})

	// b.ReportMetric(float64(totalUpdateTimeMS/steps), "update_time_(ms)")
	// b.ReportMetric(float64(totalUpdateTimeMS*1000000/totalRegOperation), "update_time_per_reg_(ns)")

	// b.ReportMetric(float64(totalProofSize/steps), "proof_size_(MB)")
	// b.ReportMetric(float64(totalPTrieConstTimeMS/steps), "ptrie_const_time_(ms)")

}

// account data is important

// Trie growth is the function of number of accounts [so if is deployed per account we don't have issue]
// it also doesn't grow that bad
