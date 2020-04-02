package trie

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"os"
	"strconv"
	"testing"
)

var s *SMT

func BenchmarkSMTCreation(b *testing.B) {
	for n := 0; n < b.N; n++ {
		s = newTestSMT(b, 255, 50000, 100, 1000, 100)
		s.database.SafeClose()
	}
}

func BenchmarkKVGen(b *testing.B) {
	for n := 0; n < b.N; n++ {
		generateRandomKVPairs(1000)
	}
}

func BenchmarkUpdate(b *testing.B) {
	benchmarks := []int{10, 20, 30, 40} //, 100, 1000, 5000, 10000}
	for _, mark := range benchmarks {
		mark := mark // Workaround for scopelint issue
		b.Run(strconv.Itoa(mark), func(b *testing.B) {
			s = newTestSMT(b, 255, 50000, 100, 1000, 100)
			defer os.RemoveAll("./db/")
			keys, values := generateRandomKVPairs(mark)

			_ = s.Update(keys, values)
			s.database.SafeClose()
		})
	}
}

func BenchmarkReadSMT(b *testing.B) {
	benchmarks := []struct {
		size    int
		trusted bool
	}{
		{10, true},
		{50, true},
		{100, true},
		{1000, true},
		{10, false},
		{50, false},
		{100, false},
		{1000, false},
		// {10000, true},
		// {10000, false},
	}

	s = newTestSMT(b, 255, 50000, 100, 1000, 100)

	defer os.RemoveAll("./db/")
	keys, values := generateRandomKVPairs(1000)
	_ = s.Update(keys, values)
	for _, mark := range benchmarks {
		mark := mark // Workaround for scopelint issue
		readKeys := getRandomKeys(keys, mark.size)
		b.Run(strconv.Itoa(mark.size), func(b *testing.B) {
			_, _, err := s.Read(readKeys, mark.trusted, s.GetRoot().value)
			if err != nil {
				b.Error(err)
			}
		})
	}

	s.database.SafeClose()

}

func BenchmarkVerifyProof(b *testing.B) {
	benchmarks := []struct {
		size    int
		trusted bool
	}{
		{10, false},
		{50, false},
		{100, false},
		{1000, false},
	}

	s = newTestSMT(b, 255, 50000, 100, 1000, 100)

	defer os.RemoveAll("./db/")
	keys, values := generateRandomKVPairs(1000)
	_ = s.Update(keys, values)
	for _, mark := range benchmarks {
		readKeys := getRandomKeys(keys, mark.size)
		values, proofs, err := s.Read(readKeys, mark.trusted, s.GetRoot().value)
		if err != nil {
			b.Error(err)
		}
		b.Run(strconv.Itoa(mark.size), func(b *testing.B) {
			for i, key := range readKeys {
				res := VerifyInclusionProof(key, values[i], proofs.flags[i], proofs.proofs[i], proofs.sizes[i], s.GetRoot().value, s.height)
				if !res {
					b.Error("Incorrect")
				}
			}
		})
	}

	s.database.SafeClose()
}

func BenchmarkVerifyHistoricalStates(b *testing.B) {
	benchmarks := []struct {
		size    int
		trusted bool
	}{
		{10, false},
		{50, false},
		{100, false},
		{1000, false},
	}

	new_smt := newTestSMT(b, 255, 50000, 100, 1000, 100)

	defer os.RemoveAll("./db/")
	keys, values := generateRandomKVPairs(1000)
	_ = new_smt.Update(keys, values)
	oldRoot := new_smt.GetRoot().value
	_, newvalues := generateRandomKVPairs(1000)
	err := new_smt.Update(keys, newvalues)
	if err != nil {
		b.Error(err)
	}
	for _, mark := range benchmarks {
		readKeys := getRandomKeys(keys, mark.size)
		values, proofs, read_err := new_smt.Read(readKeys, mark.trusted, oldRoot)
		if err != nil {
			b.Error(read_err)
		}
		b.Run(strconv.Itoa(mark.size), func(b *testing.B) {
			for i, key := range readKeys {
				res := VerifyInclusionProof(key, values[i], proofs.flags[i], proofs.proofs[i], proofs.sizes[i], oldRoot, new_smt.height)
				if !res {
					b.Error("Incorrect")
				}
			}
		})
	}

	new_smt.database.SafeClose()
	new_smt.historicalStates[hex.EncodeToString(oldRoot)].SafeClose()

}

func generateRandomKVPairs(num int) ([][]byte, [][]byte) {
	keys := make([][]byte, 0)
	values := make([][]byte, 0)
	var key []byte
	var value []byte
	for i := 0; i < num; i++ {
		key = make([]byte, 32)
		rand.Read(key)
		keys = append(keys, key)
		rand.Read(value)
		values = append(values, value)
	}

	keys = sortKeys(keys)
	return keys, values
}

func sortKeys(keys [][]byte) [][]byte {
	if len(keys) <= 1 {
		return keys
	}

	res := make([][]byte, 0)

	split := int(len(keys) / 2)

	left := sortKeys(keys[:split])
	right := sortKeys(keys[split:])

	i, j := 0, 0
	for i < len(left) && j < len(right) {
		if bytes.Compare(left[i], right[j]) < 0 {
			res = append(res, left[i])
			i++
		} else if bytes.Compare(left[i], right[j]) > 0 {
			res = append(res, right[j])
			j++
		} else {
			res = append(res, left[i])
			i++
			j++
		}
	}
	for i < len(left) {
		res = append(res, left[i])
		i++
	}
	for j < len(right) {
		res = append(res, right[j])
		j++
	}

	return res
}

func getRandomKeys(keys [][]byte, num int) [][]byte {
	if len(keys) == num {
		return keys
	}
	res := make([][]byte, 0)
	hit := make(map[int]bool)
	var target int
	for len(res) < num {
		target = rand.Intn(len(keys))
		if !hit[target] {
			res = append(res, keys[target])
			hit[target] = true
		}
	}

	return res
}
