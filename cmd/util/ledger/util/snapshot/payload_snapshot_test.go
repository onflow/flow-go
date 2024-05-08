package snapshot_test

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func Test_PayloadSnapshot(t *testing.T) {
	t.Parallel()

	payloads := createPayloads(1_000_000)

	f := func(t *testing.T, ty snapshot.MigrationSnapshotType) {
		t.Parallel()

		snapshot1, err := snapshot.NewPayloadSnapshot(zerolog.Nop(), payloads, ty, 1)
		require.NoError(t, err)

		snapshot2, err := snapshot.NewPayloadSnapshot(zerolog.Nop(), payloads, ty, 4)
		require.NoError(t, err)

		payloads1 := snapshot1.PayloadMap()
		payloads2 := snapshot2.PayloadMap()
		for k, v := range payloads1 {
			require.Equal(t, v, payloads2[k])
		}
	}

	t.Run("IndexMapBased", func(t *testing.T) {
		f(t, snapshot.SmallChangeSetSnapshot)
	})

	t.Run("MapBased", func(t *testing.T) {
		f(t, snapshot.LargeChangeSetOrReadonlySnapshot)
	})
}

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
	benchCreate(b, 100000000)

	benchMerge(b, 1000, []int{10, 100, 1000})
	benchMerge(b, 100000, []int{10, 1000, 100000})
	benchMerge(b, 10000000, []int{10, 10000, 10000000})

}

func randomPayload(size int) []byte {
	payload := make([]byte, size)
	_, _ = rand.Read(payload)
	return payload
}

func randomInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
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
			index := randomInt(existingPayloadCount)

			changes[flow.RegisterID{
				Owner: flow.EmptyAddress.String(),
				Key:   strconv.Itoa(index),
			}] = p
		}

		for i := 0; i < deletePayloads; i++ {

			// get random index
			index := randomInt(existingPayloadCount)

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
			change, remove, add := changesNum/2, changesNum/2, changesNum/2

			changes := creatChanges(len(payloads), change, remove, add)

			f := func(b *testing.B, ty snapshot.MigrationSnapshotType) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					snap, err := snapshot.NewPayloadSnapshot(zerolog.Nop(), payloads, ty, 1)
					require.NoError(b, err)
					b.StartTimer()

					_, err = snap.ApplyChangesAndGetNewPayloads(changes, nil, zerolog.Nop())
					require.NoError(b, err)
				}
			}

			b.Run("IndexMapBased", func(b *testing.B) {
				f(b, snapshot.SmallChangeSetSnapshot)
			})

			b.Run("MapBased", func(b *testing.B) {
				f(b, snapshot.LargeChangeSetOrReadonlySnapshot)
			})
		})
	}
}

func benchmarkCreate(
	b *testing.B,
	payloadsNum int,
) {

	payloads := createPayloads(payloadsNum)

	f := func(b *testing.B, ty snapshot.MigrationSnapshotType) {
		for _, workers := range []int{1, 2, 4, 8, 12} {
			b.Run("workers_"+strconv.Itoa(workers), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, err := snapshot.NewPayloadSnapshot(zerolog.Nop(), payloads, ty, workers)
					require.NoError(b, err)
				}
			})
		}
	}

	b.Run("IndexMapBased", func(b *testing.B) {
		f(b, snapshot.SmallChangeSetSnapshot)
	})

	b.Run("MapBased", func(b *testing.B) {
		f(b, snapshot.LargeChangeSetOrReadonlySnapshot)
	})
}
