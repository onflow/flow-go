package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func BenchmarkRetrieveApprovals(b *testing.B) {
	dbtest.BenchWithDB(b, func(b *testing.B, db storage.DB) {
		b.Run("RetrieveApprovals", func(b *testing.B) {
			approval := unittest.ResultApprovalFixture()
			require.NoError(b, db.WithReaderBatchWriter(storage.OnlyWriter(operation.InsertResultApproval(approval))))

			b.ResetTimer()

			approvalID := approval.ID()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					var stored flow.ResultApproval
					require.NoError(b, operation.RetrieveResultApproval(approvalID, &stored)(db.Reader()))
				}
			})

		})
	})
}

func BenchmarkInsertApproval(b *testing.B) {
	dbtest.BenchWithDB(b, func(b *testing.B, db storage.DB) {
		b.Run("InsertApprovals", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				approval := unittest.ResultApprovalFixture()
				require.NoError(b, db.WithReaderBatchWriter(storage.OnlyWriter(operation.InsertResultApproval(approval))))
			}
		})
	})
}
