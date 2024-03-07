package util_test

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func Benchmark_Merge(b *testing.B) {

	merge := func(b *testing.B, payloadsNum, changesNum int) {
		b.Run("merge_"+strconv.Itoa(payloadsNum)+"_"+strconv.Itoa(changesNum), func(b *testing.B) {
			benchmarkMerge(b, payloadsNum, changesNum)
		})
	}

	create := func(b *testing.B, payloadsNum int) {
		b.Run("create_"+strconv.Itoa(payloadsNum), func(b *testing.B) {
			benchmarkCreate(b, payloadsNum)
		})
	}

	create(b, 1000)
	create(b, 100000)
	create(b, 10000000)

	merge(b, 1000, 10)
	merge(b, 1000, 100)
	merge(b, 1000, 1000)

	merge(b, 100000, 10)
	merge(b, 100000, 1000)
	merge(b, 100000, 100000)

	merge(b, 10000000, 10)
	merge(b, 10000000, 10000)
	merge(b, 10000000, 10000000)
}

func randomPayload(size int) []byte {
	payload := make([]byte, size)
	_, _ = rand.Read(payload)
	return payload
}

func createPayloads(payloadsNum int) []*ledger.Payload {
	// 1kb payloads

	payloads := make([]*ledger.Payload, payloadsNum)
	for i := 0; i < payloadsNum; i++ {
		payloads[i] = ledger.NewPayload(
			convert.RegisterIDToLedgerKey(flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(i),
			}),
			randomPayload(1024),
		)
	}
	return payloads
}

func benchmarkMerge(b *testing.B, payloadsNum, changesNum int) {

	creatChanges := func(lastIndex, changed, deleted, new int) map[flow.RegisterID]flow.RegisterValue {
		// half of the changes should be new payloads
		changes := make(map[flow.RegisterID]flow.RegisterValue)
		for i := 0; i < changed; i++ {

			// get random index
			index := rand.Intn(lastIndex)

			changes[flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(index),
			}] = randomPayload(1024)
		}

		for i := 0; i < deleted; i++ {

			// get random index
			index := rand.Intn(lastIndex)

			changes[flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(index),
			}] = []byte{}
		}

		for i := 0; i < new; i++ {
			changes[flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(i + lastIndex),
			}] = []byte{}
		}

		return changes
	}

	payloads := createPayloads(payloadsNum)

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		change, remove, add := changesNum/3, changesNum/3, changesNum/3

		// half of the changes should be new payloads
		changes := creatChanges(len(payloads), change, remove, add)

		snapshot, err := util.NewPayloadSnapshot(payloads)
		require.NoError(b, err)

		b.StartTimer()

		_, err = snapshot.ApplyChangesAndGetNewPayloads(changes, nil, zerolog.Nop())
		require.NoError(b, err)
	}
}

func benchmarkCreate(b *testing.B, payloadsNum int) {
	payloads := createPayloads(payloadsNum)
	for i := 0; i < b.N; i++ {
		_, err := util.NewPayloadSnapshot(payloads)
		require.NoError(b, err)
	}
}
