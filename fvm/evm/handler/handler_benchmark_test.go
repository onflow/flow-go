package handler_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func BenchmarkStorage(b *testing.B) { benchmarkStorageGrowth(1_000_000, b) }

// benchmark
func benchmarkStorageGrowth(count int, b *testing.B) {
	testutils.RunWithTestBackend(b, func(backend types.Backend) {
		testutils.RunWithTestFlowEVMRootAddress(b, backend, func(rootAddr flow.Address) {
			testutils.RunWithDeployedContract(b,
				testutils.GetDummyKittyTestContract(b),
				backend,
				rootAddr,
				func(tc *testutils.TestContract) {
					db, handler := SetupHandler(b, backend, rootAddr)
					account := handler.AccountByAddress(handler.AllocateAddress(), true)
					account.Deposit(types.NewFlowTokenVault(types.Balance(10000)))
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
						fmt.Println(i, ">> read:", db.BytesRetrieved(), " written:", db.BytesStored())
						db.ResetReporter()
						db.DropCache()
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

// account data is important

// Trie growth is the function of number of accounts [so if is deployed per account we don't have issue]
// it also doesn't grow that bad
