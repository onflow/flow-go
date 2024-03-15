package util_test

import (
	"crypto/rand"
	"encoding/hex"
	rand2 "math/rand"
	"runtime"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func TestGroupPayloadsByAccount(t *testing.T) {
	log := zerolog.New(zerolog.NewTestWriter(t))
	payloads := generateRandomPayloads(1000000)
	tmp := make([]*ledger.Payload, len(payloads))
	copy(tmp, payloads)

	groups := util.GroupPayloadsByAccount(log, payloads, 0)

	require.Greater(t, groups.Len(), 1)
}

func TestGroupPayloadsByAccountForDataRace(t *testing.T) {
	log := zerolog.New(zerolog.NewTestWriter(t))

	const accountSize = 4
	var payloads []*ledger.Payload
	for i := 0; i < accountSize; i++ {
		payloads = append(payloads, generateRandomPayloadsWithAddress(generateRandomAddress(), 100_000)...)
	}

	const nWorkers = 8
	groups := util.GroupPayloadsByAccount(log, payloads, nWorkers)
	require.Equal(t, accountSize, groups.Len())
}

func TestGroupPayloadsByAccountCompareResults(t *testing.T) {
	log := zerolog.Nop()
	payloads := generateRandomPayloads(1000000)
	tmp1 := make([]*ledger.Payload, len(payloads))
	tmp2 := make([]*ledger.Payload, len(payloads))
	copy(tmp1, payloads)
	copy(tmp2, payloads)

	groups1 := util.GroupPayloadsByAccount(log, tmp1, 0)
	groups2 := util.GroupPayloadsByAccount(log, tmp2, runtime.NumCPU())

	groups3 := map[common.Address][]*ledger.Payload{}
	for _, payload := range payloads {
		key, err := payload.Key()
		require.NoError(t, err)
		registerID, err := convert.LedgerKeyToRegisterID(key)
		require.NoError(t, err)
		address, err := common.BytesToAddress([]byte(registerID.Owner))
		require.NoError(t, err)
		if _, ok := groups3[address]; !ok {
			groups3[address] = []*ledger.Payload{}
		}
		groups3[address] = append(groups3[address], payload)
	}

	require.Equal(t, groups1.Len(), groups2.Len())
	require.Equal(t, groups1.Len(), len(groups3))
	for {
		group1, err1 := groups1.Next()
		group2, err2 := groups2.Next()

		require.NoError(t, err1)
		require.NoError(t, err2)

		if group1 == nil {
			require.Nil(t, group2)
			break
		}

		require.Equal(t, group1.Address, group2.Address)
		require.Equal(t, len(group1.Payloads), len(group2.Payloads))
		require.ElementsMatch(t, group1.Payloads, group2.Payloads)
		require.Equal(t, len(group1.Payloads), len(groups3[group1.Address]))
		require.ElementsMatch(t, group1.Payloads, groups3[group1.Address])
	}
}

func BenchmarkGroupPayloadsByAccount(b *testing.B) {
	log := zerolog.Nop()
	payloads := generateRandomPayloads(10000000)
	tmp := make([]*ledger.Payload, len(payloads))

	bench := func(b *testing.B, nWorker int) func(b *testing.B) {
		return func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				copy(tmp, payloads)
				b.StartTimer()
				util.GroupPayloadsByAccount(log, tmp, nWorker)
			}
		}
	}

	b.Run("1 worker", bench(b, 1))
	b.Run("2 worker", bench(b, 2))
	b.Run("4 worker", bench(b, 4))
	b.Run("8 worker", bench(b, 8))
	b.Run("max worker", bench(b, runtime.NumCPU()))
}

// GeneratePayloads generates n random payloads
// with a random number of payloads per account (exponentially distributed)
func generateRandomPayloads(n int) []*ledger.Payload {
	const meanPayloadsPerAccount = 100
	const minPayloadsPerAccount = 1

	payloads := make([]*ledger.Payload, 0, n)

	for i := 0; i < n; {

		registersForAccount := minPayloadsPerAccount + int(rand2.ExpFloat64()*(meanPayloadsPerAccount-minPayloadsPerAccount))
		if registersForAccount > n-i {
			registersForAccount = n - i
		}
		i += registersForAccount

		accountKey := generateRandomAccountKey()
		for j := 0; j < registersForAccount; j++ {
			payloads = append(payloads,
				ledger.NewPayload(
					accountKey,
					[]byte(generateRandomString(10)),
				))
		}
	}

	return payloads
}

func generateRandomPayloadsWithAddress(address string, n int) []*ledger.Payload {
	const meanPayloadsPerAccount = 100
	const minPayloadsPerAccount = 1

	payloads := make([]*ledger.Payload, 0, n)

	for i := 0; i < n; {

		registersForAccount := minPayloadsPerAccount + int(rand2.ExpFloat64()*(meanPayloadsPerAccount-minPayloadsPerAccount))
		if registersForAccount > n-i {
			registersForAccount = n - i
		}
		i += registersForAccount

		accountKey := convert.RegisterIDToLedgerKey(flow.RegisterID{
			Owner: address,
			Key:   generateRandomString(10),
		})
		for j := 0; j < registersForAccount; j++ {
			payloads = append(payloads,
				ledger.NewPayload(
					accountKey,
					[]byte(generateRandomString(10)),
				))
		}
	}

	return payloads
}

func generateRandomAccountKey() ledger.Key {
	return convert.RegisterIDToLedgerKey(flow.RegisterID{
		Owner: generateRandomAddress(),
		Key:   generateRandomString(10),
	})
}

func generateRandomString(i int) string {
	buf := make([]byte, i)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func generateRandomAddress() string {
	buf := make([]byte, flow.AddressLength)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return string(buf)
}
