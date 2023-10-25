package handler_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func BenchmarkStorage(b *testing.B) { benchmarkStorageGrowth(1000, b) }

// benchmark
func benchmarkStorageGrowth(count int, b *testing.B) {
	testutils.RunWithTestBackendWithMeterOption(b, true, func(backend types.Backend) {
		testutils.RunWithTestFlowEVMRootAddress(b, backend, func(rootAddr flow.Address) {
			testutils.RunWithDeployedContract(b,
				testutils.GetDummyKittyTestContract(b),
				backend,
				rootAddr,
				func(tc *testutils.TestContract) {
					handler := SetupHandler(b, backend, rootAddr)
					account := handler.AccountByAddress(handler.AllocateAddress(), true)
					account.Deposit(types.NewFlowTokenVault(types.Balance(10000)))
					lastInteractionUsed, err := backend.InteractionUsed()
					require.NoError(b, err)
					for i := 0; i < count; i++ {
						matronId := testutils.RandomBigInt(1000)
						sireId := testutils.RandomBigInt(1000)
						generation := testutils.RandomBigInt(1000)
						genes := testutils.RandomBigInt(1000)
						require.NotNil(b, account)
						account.Call(
							tc.DeployedAt,
							tc.MakeCallData(b, "CreateKitty", matronId, sireId, generation, genes),
							300_000_000,
							types.Balance(0),
						)
						newint, err := backend.InteractionUsed()
						require.NoError(b, err)

						fmt.Println("interaction used:", newint-lastInteractionUsed)
						lastInteractionUsed = newint
					}

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

// TODO reads are too low, some sort of caching is happening somewhere , or data optimizaiton ?

// account data is important

// Trie growth is the function of number of accounts [so if is deployed per account we don't have issue]
// it also doesn't grow that bad
