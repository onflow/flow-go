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

func Benchmark_PayloadSnapshot(b *testing.B) {
	benchMerge := func(
		b *testing.B,
		payloadsNum int, changesNum []int,
	) {
		b.Run("merge_"+strconv.Itoa(payloadsNum), func(b *testing.B) {
			benchmarkMerge(b, payloadsNum, changesNum)
		})
	}

	benchCreate := func(
		b *testing.B,
		payloadsNum int,
	) {
		b.Run("create_"+strconv.Itoa(payloadsNum), func(b *testing.B) {
			benchmarkCreate(b, payloadsNum)
		})
	}

	benchCreate(b, 1000)
	benchCreate(b, 100000)
	benchCreate(b, 10000000)

	benchMerge(b, 1000, []int{10, 100, 1000})
	benchMerge(b, 100000, []int{10, 1000, 100000})
	benchMerge(b, 10000000, []int{10, 10000, 10000000})

}

func randomPayload(size int) []byte {
	payload := make([]byte, size)
	_, _ = rand.Read(payload)
	return payload
}

func createPayloads(payloadsNum int) []*ledger.Payload {
	// 1kb payloads
	payload := randomPayload(1024)

	payloads := make([]*ledger.Payload, payloadsNum)
	for i := 0; i < payloadsNum; i++ {
		p := make([]byte, len(payload))
		copy(p, payload)
		payloads[i] = ledger.NewPayload(
			convert.RegisterIDToLedgerKey(flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(i),
			}),
			p,
		)
	}
	return payloads
}

func benchmarkMerge(b *testing.B, payloadsNum int, changes []int) {

	creatChanges := func(
		existingPayloadCount int,
		changePayloads int,
		deletePayloads int,
		createPayloads int,
	) map[flow.RegisterID]flow.RegisterValue {
		// half of the changes should be new payloads
		changes := make(map[flow.RegisterID]flow.RegisterValue)
		payload := randomPayload(1024)
		for i := 0; i < changePayloads; i++ {
			p := make([]byte, len(payload))
			copy(p, payload)

			// get random index
			index := rand.Intn(existingPayloadCount)

			changes[flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(index),
			}] = p
		}

		for i := 0; i < deletePayloads; i++ {

			// get random index
			index := rand.Intn(existingPayloadCount)

			changes[flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(index),
			}] = []byte{}
		}

		payload = randomPayload(1024)
		for i := 0; i < createPayloads; i++ {
			p := make([]byte, len(payload))
			copy(p, payload)
			changes[flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(i + existingPayloadCount),
			}] = p
		}

		return changes
	}

	payloads := createPayloads(payloadsNum)

	for _, changesNum := range changes {
		b.Run("changes_"+strconv.Itoa(changesNum), func(b *testing.B) {

			// third of the changes should be new payloads
			// third of the changes should be existing payloads
			// third of the changes should be removals
			change, remove, add := changesNum/3, changesNum/3, changesNum-2*(changesNum/3)

			changes := creatChanges(len(payloads), change, remove, add)

			b.Run("MapBasedPayloadSnapshot", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					snapshot, err := util.NewMapBasedPayloadSnapshot(payloads)
					require.NoError(b, err)
					b.StartTimer()

					_, err = snapshot.ApplyChangesAndGetNewPayloads(changes, nil, zerolog.Nop())
					require.NoError(b, err)
				}
			})

			b.Run("PayloadSnapshot", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					snapshot, err := util.NewPayloadSnapshot(payloads)
					require.NoError(b, err)
					b.StartTimer()

					_, err = snapshot.ApplyChangesAndGetNewPayloads(changes, nil, zerolog.Nop())
					require.NoError(b, err)
				}
			})
		})
	}
}

func benchmarkCreate(
	b *testing.B,
	payloadsNum int,
) {

	payloads := createPayloads(payloadsNum)

	b.Run("MapBasedPayloadSnapshot", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := util.NewMapBasedPayloadSnapshot(payloads)
			require.NoError(b, err)
		}
	})

	b.Run("PayloadSnapshot", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := util.NewPayloadSnapshot(payloads)
			require.NoError(b, err)
		}
	})
}
