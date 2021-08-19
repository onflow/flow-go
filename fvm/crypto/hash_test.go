package crypto_test

import (
	"math/rand"
	"testing"
	"time"

	"crypto/sha256"
	"crypto/sha512"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// TestPrefixedHash is a specific test for prefixed hashing
func TestPrefixedHash(t *testing.T) {

	hashingAlgoToTestingAlgo := map[hash.HashingAlgorithm]func([]byte) []byte{
		hash.SHA2_256: func(data []byte) []byte {
			h := sha256.Sum256(data)
			return h[:]
		},
		hash.SHA3_256: func(data []byte) []byte {
			h := sha3.Sum256(data)
			return h[:]
		},
		hash.SHA2_384: func(data []byte) []byte {
			h := sha512.Sum384(data)
			return h[:]
		},
		hash.SHA3_384: func(data []byte) []byte {
			h := sha3.Sum384(data)
			return h[:]
		},
	}

	r := time.Now().UnixNano()
	rand.Seed(r)
	t.Logf("math rand seed is %d", r)

	for hashAlgo, testFunction := range hashingAlgoToTestingAlgo {
		t.Run(hashAlgo.String()+" with a prefix", func(t *testing.T) {
			for i := 32; i < 5000; i++ {
				// first 32 bytes of data are the tag
				data := make([]byte, i)
				rand.Read(data)
				expected := testFunction(data)

				tag := string(data[:flow.DomainTagLength])
				message := data[flow.DomainTagLength:]
				hasher, err := crypto.NewPrefixedHashing(hashAlgo, tag)
				require.NoError(t, err)
				h := hasher.ComputeHash(message)
				assert.Equal(t, expected, []byte(h))
			}
		})

		t.Run(hashAlgo.String()+" without a prefix", func(t *testing.T) {
			for i := 0; i < 5000; i++ {
				data := make([]byte, i)
				rand.Read(data)
				expected := testFunction(data)

				tag := ""
				hasher, err := crypto.NewPrefixedHashing(hashAlgo, tag)
				require.NoError(t, err)
				h := hasher.ComputeHash(data)
				assert.Equal(t, expected, []byte(h))
			}
		})
	}

	t.Run("non supported algorithm", func(t *testing.T) {
		_, err := crypto.NewPrefixedHashing(hash.KMAC128, "")
		require.Error(t, err)
	})
}
