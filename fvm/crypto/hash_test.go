package crypto_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"crypto/sha256"
	"crypto/sha512"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"

	"github.com/onflow/cadence/runtime"

	gocrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// TestPrefixedHash is a specific test for prefixed hashing
func TestPrefixedHash(t *testing.T) {

	hashingAlgoToTestingAlgo := map[hash.HashingAlgorithm](func([]byte) []byte){
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

func TestHashWithTag(t *testing.T) {
	t.Run("tag too long", func(t *testing.T) {
		algorithms := []hash.HashingAlgorithm{
			hash.SHA2_256,
			hash.SHA2_384,
			hash.SHA3_256,
			hash.SHA3_384,
		}

		okTag := [32]byte{}
		longTag := [33]byte{}

		for i, algorithm := range algorithms {
			t.Run(fmt.Sprintf("algo %d: %v", i, algorithm), func(t *testing.T) {
				_, err := crypto.HashWithTag(algorithm, string(longTag[:]), []byte("some data"))
				require.Error(t, err)
			})

			t.Run(fmt.Sprintf("algo %d: %v - control (tag ok)", i, algorithm), func(t *testing.T) {
				_, err := crypto.HashWithTag(algorithm, string(okTag[:]), []byte("some data"))
				require.NoError(t, err)
			})
		}
	})
}

func TestVerifySignatureFromRuntime(t *testing.T) {
	t.Run("verify should fail on incorrect combinations", func(t *testing.T) {
		correctCombinations := make(map[runtime.SignatureAlgorithm]map[runtime.HashAlgorithm]struct{})

		correctCombinations[runtime.SignatureAlgorithmBLS_BLS12_381] = make(map[runtime.HashAlgorithm]struct{})
		correctCombinations[runtime.SignatureAlgorithmBLS_BLS12_381][runtime.HashAlgorithmKMAC128_BLS_BLS12_381] = struct{}{}
		correctCombinations[runtime.SignatureAlgorithmECDSA_P256] = make(map[runtime.HashAlgorithm]struct{})
		correctCombinations[runtime.SignatureAlgorithmECDSA_P256][runtime.HashAlgorithmSHA2_256] = struct{}{}
		correctCombinations[runtime.SignatureAlgorithmECDSA_P256][runtime.HashAlgorithmSHA3_256] = struct{}{}
		correctCombinations[runtime.SignatureAlgorithmECDSA_secp256k1] = make(map[runtime.HashAlgorithm]struct{})
		correctCombinations[runtime.SignatureAlgorithmECDSA_secp256k1][runtime.HashAlgorithmSHA2_256] = struct{}{}
		correctCombinations[runtime.SignatureAlgorithmECDSA_secp256k1][runtime.HashAlgorithmSHA3_256] = struct{}{}

		signatureAlgos := []runtime.SignatureAlgorithm{
			runtime.SignatureAlgorithmECDSA_P256,
			runtime.SignatureAlgorithmECDSA_secp256k1,
			runtime.SignatureAlgorithmBLS_BLS12_381,
		}
		hashAlgos := []runtime.HashAlgorithm{
			runtime.HashAlgorithmSHA2_256,
			runtime.HashAlgorithmSHA2_384,
			runtime.HashAlgorithmSHA3_256,
			runtime.HashAlgorithmSHA3_384,
			runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
		}

		for _, s := range signatureAlgos {
			for _, h := range hashAlgos {
				t.Run(fmt.Sprintf("combination: %v, %v", s, h), func(t *testing.T) {
					seed := make([]byte, 256)
					rand.Read(seed)
					pk, err := gocrypto.GeneratePrivateKey(crypto.RuntimeToCryptoSigningAlgorithm(s), seed)
					require.NoError(t, err)

					tag := string(flow.UserDomainTag[:])
					var hasher hash.Hasher
					if h != runtime.HashAlgorithmKMAC128_BLS_BLS12_381 {
						hasher, err = crypto.NewPrefixedHashing(crypto.RuntimeToCryptoHashingAlgorithm(h), tag)
						require.NoError(t, err)
					} else {
						hasher = gocrypto.NewBLSKMAC(tag)
					}

					signature := make([]byte, 0)
					sig, err := pk.Sign([]byte("some data"), hasher)
					if _, shouldBeOk := correctCombinations[s][h]; shouldBeOk {
						require.NoError(t, err)
					}

					if sig != nil {
						signature = sig.Bytes()
					}

					ok, err := crypto.VerifySignatureFromRuntime(
						crypto.NewDefaultSignatureVerifier(),
						signature,
						tag,
						[]byte("some data"),
						pk.PublicKey().Encode(),
						s,
						h,
					)

					if _, shouldBeOk := correctCombinations[s][h]; shouldBeOk {
						require.NoError(t, err)
						require.True(t, ok)
					} else {
						require.Error(t, err)
						require.False(t, ok)
					}
				})
			}
		}
	})

	t.Run("BLS verification tag size > 32 bytes should pass", func(t *testing.T) {
		seed := make([]byte, 256)
		rand.Read(seed)
		pk, err := gocrypto.GeneratePrivateKey(gocrypto.BLSBLS12381, seed)
		require.NoError(t, err)

		tag := make([]byte, 256)
		rand.Read(tag)
		hasher := gocrypto.NewBLSKMAC(string(tag))

		signature := make([]byte, 0)
		sig, err := pk.Sign([]byte("some data"), hasher)
		require.NoError(t, err)

		if sig != nil {
			signature = sig.Bytes()
		}

		ok, err := crypto.VerifySignatureFromRuntime(
			crypto.NewDefaultSignatureVerifier(),
			signature,
			string(tag),
			[]byte("some data"),
			pk.PublicKey().Encode(),
			runtime.SignatureAlgorithmBLS_BLS12_381,
			runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
		)

		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("tag combinations", func(t *testing.T) {

		cases := []struct {
			hashTag   string
			verifyTag string
			require   func(t *testing.T, sigOk bool, err error)
		}{
			{
				hashTag:   "user",
				verifyTag: "user",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			}, {
				hashTag:   string(flow.UserDomainTag[:]),
				verifyTag: "user",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			}, {
				hashTag:   "user",
				verifyTag: string(flow.UserDomainTag[:]),
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			}, {
				hashTag:   "random_tag",
				verifyTag: "random_tag",
				require: func(t *testing.T, sigOk bool, err error) {
					require.Error(t, err)
				},
			},
		}

		signatureAlgos := []runtime.SignatureAlgorithm{
			runtime.SignatureAlgorithmECDSA_P256,
			runtime.SignatureAlgorithmECDSA_secp256k1,
		}
		hashAlgos := []runtime.HashAlgorithm{
			runtime.HashAlgorithmSHA2_256,
			runtime.HashAlgorithmSHA3_256,
		}

		for _, c := range cases {
			for _, s := range signatureAlgos {
				for _, h := range hashAlgos {
					t.Run(fmt.Sprintf("hash tag: %v, verify tag: %v [%v, %v]", c.hashTag, c.verifyTag, s, h), func(t *testing.T) {
						seed := make([]byte, 256)
						rand.Read(seed)
						pk, err := gocrypto.GeneratePrivateKey(crypto.RuntimeToCryptoSigningAlgorithm(s), seed)
						require.NoError(t, err)

						hasher, err := crypto.NewPrefixedHashing(crypto.RuntimeToCryptoHashingAlgorithm(h), c.hashTag)
						require.NoError(t, err)

						sig, err := pk.Sign([]byte("some data"), hasher)
						require.NoError(t, err)
						signature := sig.Bytes()

						ok, err := crypto.VerifySignatureFromRuntime(
							crypto.NewDefaultSignatureVerifier(),
							signature,
							c.verifyTag,
							[]byte("some data"),
							pk.PublicKey().Encode(),
							s,
							h,
						)

						c.require(t, ok, err)
					})
				}
			}
		}
	})
}
