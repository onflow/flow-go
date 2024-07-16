package handler_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func BenchmarkProposalGrowth(b *testing.B) { benchmarkBlockProposalGrowth(b, 1000) }

func benchmarkBlockProposalGrowth(b *testing.B, txCounts int) {
	testutils.RunWithTestBackend(b, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(b, backend, func(rootAddr flow.Address) {

			bs := handler.NewBlockStore(backend, rootAddr)
			for i := 0; i < txCounts; i++ {
				bp, err := bs.BlockProposal()
				require.NoError(b, err)
				res := testutils.RandomResultFixture(b)
				bp.AppendTransaction(res)
				err = bs.UpdateBlockProposal(bp)
				require.NoError(b, err)
			}

			// check the impact of updating block proposal after x number of transactions
			backend.ResetStats()
			startTime := time.Now()
			bp, err := bs.BlockProposal()
			require.NoError(b, err)
			res := testutils.RandomResultFixture(b)
			bp.AppendTransaction(res)
			err = bs.UpdateBlockProposal(bp)
			require.NoError(b, err)

			b.ReportMetric(float64(time.Since(startTime).Nanoseconds()), "proposal_update_time_ns")
			b.ReportMetric(float64(backend.TotalBytesRead()), "proposal_update_bytes_read")
			b.ReportMetric(float64(backend.TotalBytesWritten()), "proposal_update_bytes_written")
			b.ReportMetric(float64(backend.TotalStorageSize()), "proposal_update_total_storage_size")

			// check the impact of block commit after x number of transactions
			backend.ResetStats()
			startTime = time.Now()
			bp, err = bs.BlockProposal()
			require.NoError(b, err)
			err = bs.CommitBlockProposal(bp)
			require.NoError(b, err)

			b.ReportMetric(float64(time.Since(startTime).Nanoseconds()), "block_commit_time_ns")
			b.ReportMetric(float64(backend.TotalBytesRead()), "block_commit_bytes_read")
			b.ReportMetric(float64(backend.TotalBytesWritten()), "block_commit_bytes_written")
			b.ReportMetric(float64(backend.TotalStorageSize()), "block_commit_total_storage_size")
		})
	})
}
