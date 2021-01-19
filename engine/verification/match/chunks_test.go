package match_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/engine/verification/match"
	"github.com/onflow/flow-go/model/flow"
)

// when maxAttempt is set to 3, CanTry will only return true for the first 3 times.
func TestCanTry(t *testing.T) {
	t.Run("maxAttempt=3", func(t *testing.T) {
		maxAttempt := 3
		chunks := match.NewChunks(10)
		c := ChunkWithIndex(flow.Identifier{0x11}, 0)
		chunk := match.NewChunkStatus(c, flow.Identifier{0xaa}, flow.Identifier{0xbb})
		chunks.Add(chunk)
		results := []bool{}
		for i := 0; i < 5; i++ {
			results = append(results, match.CanTry(maxAttempt, chunk))
			chunks.IncrementAttempt(chunk.ID())
		}
		require.Equal(t, []bool{true, true, true, false, false}, results)
	})
}

// determine size of encoded Execution Receipts and results
func TestExecutionResultSize(t *testing.T) {
	result := unittest.ExecutionResultFixture()
	jsonResult, _ := json.Marshal(result)
	fmt.Printf("ExecutionResult:\n %s\n", string(jsonResult))
	fmt.Printf("sizeof(ExecutionResult) = %d bytes\n", len(jsonResult))

	receipt := unittest.ExecutionReceiptFixture()
	jsonReceipt, _ := json.Marshal(receipt)
	fmt.Printf("ExecutionReceipt:\n %s\n", string(jsonReceipt))
	fmt.Printf("sizeof(ExecutionReceipt) = %d bytes\n", len(jsonReceipt))
}
