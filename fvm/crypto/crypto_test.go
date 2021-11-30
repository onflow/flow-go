package crypto_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime"

	gocrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/model/flow"
)

func TestHashWithTag(t *testing.T) {
	t.Run("tag too long", func(t *testing.T) {
		algorithms := []hash.HashingAlgorithm{
			hash.SHA2_256,
			hash.SHA2_384,
			hash.SHA3_256,
			hash.SHA3_384,
		}

		okTag := [flow.DomainTagLength / 2]byte{}   // tag does not exceed 32 bytes
		longTag := [flow.DomainTagLength + 1]byte{} // tag larger that 32 bytes

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

	// make sure the seed length is larger than miniumum seed lengths of all signature schemes
	seedLength := 64

	t.Run("verify should fail on incorrect combinations", func(t *testing.T) {
		correctCombinations := map[runtime.SignatureAlgorithm]map[runtime.HashAlgorithm]struct{}{

			runtime.SignatureAlgorithmBLS_BLS12_381: map[runtime.HashAlgorithm]struct{}{
				runtime.HashAlgorithmKMAC128_BLS_BLS12_381: struct{}{},
			},
			runtime.SignatureAlgorithmECDSA_P256: map[runtime.HashAlgorithm]struct{}{
				runtime.HashAlgorithmSHA2_256: struct{}{},
				runtime.HashAlgorithmSHA3_256: struct{}{},
			},
			runtime.SignatureAlgorithmECDSA_secp256k1: map[runtime.HashAlgorithm]struct{}{
				runtime.HashAlgorithmSHA2_256: struct{}{},
				runtime.HashAlgorithmSHA3_256: struct{}{},
			},
		}

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
					seed := make([]byte, seedLength)
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
		seed := make([]byte, seedLength)
		rand.Read(seed)
		pk, err := gocrypto.GeneratePrivateKey(gocrypto.BLSBLS12381, seed)
		require.NoError(t, err)

		tag := make([]byte, 2*flow.DomainTagLength) // cosntant larger than 32 (padded tag length)
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
			signTag   string
			verifyTag string
			require   func(t *testing.T, sigOk bool, err error)
		}{
			{
				signTag:   "user",
				verifyTag: "user",
				require: func(t *testing.T, sigOk bool, err error) {
					require.Error(t, err)
					require.False(t, sigOk)
				},
			}, {
				signTag:   string(flow.UserDomainTag[:]),
				verifyTag: string(flow.UserDomainTag[:]),
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			}, {
				signTag:   flow.UserTagString,
				verifyTag: flow.UserTagString,
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			}, {
				signTag:   string(flow.UserDomainTag[:]),
				verifyTag: "user",
				require: func(t *testing.T, sigOk bool, err error) {
					require.Error(t, err)
					require.False(t, sigOk)
				},
			}, {
				signTag:   "user",
				verifyTag: string(flow.UserDomainTag[:]),
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			}, {
				signTag:   "random_tag",
				verifyTag: "random_tag",
				require: func(t *testing.T, sigOk bool, err error) {
					require.Error(t, err)
					require.False(t, sigOk)
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
					t.Run(fmt.Sprintf("hash tag: %v, verify tag: %v [%v, %v]", c.signTag, c.verifyTag, s, h), func(t *testing.T) {
						seed := make([]byte, seedLength)
						rand.Read(seed)
						pk, err := gocrypto.GeneratePrivateKey(crypto.RuntimeToCryptoSigningAlgorithm(s), seed)
						require.NoError(t, err)

						hasher, err := crypto.NewPrefixedHashing(crypto.RuntimeToCryptoHashingAlgorithm(h), c.signTag)
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

func TestValidatePublicKey(t *testing.T) {

	// make sure the seed length is larger than miniumum seed lengths of all signature schemes
	seedLength := 64

	validPublicKey := func(t *testing.T, s runtime.SignatureAlgorithm) []byte {
		seed := make([]byte, seedLength)
		rand.Read(seed)
		pk, err := gocrypto.GeneratePrivateKey(crypto.RuntimeToCryptoSigningAlgorithm(s), seed)
		require.NoError(t, err)
		return pk.PublicKey().Encode()
	}

	t.Run("Unknown algorithm should return false", func(t *testing.T) {
		valid, err := crypto.ValidatePublicKey(runtime.SignatureAlgorithmUnknown, validPublicKey(t, runtime.SignatureAlgorithmECDSA_P256))
		require.Error(t, err)
		require.False(t, valid)
	})

	t.Run("valid public key should return true", func(t *testing.T) {
		signatureAlgos := []runtime.SignatureAlgorithm{
			runtime.SignatureAlgorithmECDSA_P256,
			runtime.SignatureAlgorithmECDSA_secp256k1,
			runtime.SignatureAlgorithmBLS_BLS12_381,
		}
		for i, s := range signatureAlgos {
			t.Run(fmt.Sprintf("case %v: %v", i, s), func(t *testing.T) {
				valid, err := crypto.ValidatePublicKey(s, validPublicKey(t, s))
				require.NoError(t, err)
				require.True(t, valid)
			})
		}
	})

	t.Run("invalid public key should return false", func(t *testing.T) {
		signatureAlgos := []runtime.SignatureAlgorithm{
			runtime.SignatureAlgorithmECDSA_P256,
			runtime.SignatureAlgorithmECDSA_secp256k1,
			runtime.SignatureAlgorithmBLS_BLS12_381,
		}
		for i, s := range signatureAlgos {
			t.Run(fmt.Sprintf("case %v: %v", i, s), func(t *testing.T) {
				key := validPublicKey(t, s)
				key[0] ^= 1 // alter one bit of the valid key

				valid, err := crypto.ValidatePublicKey(s, key)
				require.Error(t, err)
				require.False(t, valid)
			})
		}
	})
}

func TestHashingAlgorithmConversion(t *testing.T) {
	hashingAlgoMapping := map[runtime.HashAlgorithm]hash.HashingAlgorithm{
		runtime.HashAlgorithmSHA2_256:              hash.SHA2_256,
		runtime.HashAlgorithmSHA3_256:              hash.SHA3_256,
		runtime.HashAlgorithmSHA2_384:              hash.SHA2_384,
		runtime.HashAlgorithmSHA3_384:              hash.SHA3_384,
		runtime.HashAlgorithmKMAC128_BLS_BLS12_381: hash.KMAC128,
	}

	for runtimeAlgo, cryptoAlgo := range hashingAlgoMapping {
		assert.Equal(t, cryptoAlgo, crypto.RuntimeToCryptoHashingAlgorithm(runtimeAlgo))
		assert.Equal(t, runtimeAlgo, crypto.CryptoToRuntimeHashingAlgorithm(cryptoAlgo))
	}
}

func TestSigningAlgorithmConversion(t *testing.T) {
	signingAlgoMapping := map[runtime.SignatureAlgorithm]gocrypto.SigningAlgorithm{
		runtime.SignatureAlgorithmECDSA_P256:      gocrypto.ECDSAP256,
		runtime.SignatureAlgorithmECDSA_secp256k1: gocrypto.ECDSASecp256k1,
		runtime.SignatureAlgorithmBLS_BLS12_381:   gocrypto.BLSBLS12381,
	}

	for runtimeAlgo, cryptoAlgo := range signingAlgoMapping {
		assert.Equal(t, cryptoAlgo, crypto.RuntimeToCryptoSigningAlgorithm(runtimeAlgo))
		assert.Equal(t, runtimeAlgo, crypto.CryptoToRuntimeSigningAlgorithm(cryptoAlgo))
	}
}
