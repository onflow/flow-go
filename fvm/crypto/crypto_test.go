package crypto_test

import (
	"crypto/rand"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gocrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
)

func TestHashWithTag(t *testing.T) {
	t.Run("tag too long", func(t *testing.T) {
		algorithms := []hash.HashingAlgorithm{
			hash.SHA2_256,
			hash.SHA2_384,
			hash.SHA3_256,
			hash.SHA3_384,
			hash.Keccak_256,
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

	correctCombinations := map[runtime.SignatureAlgorithm]map[runtime.HashAlgorithm]struct{}{

		runtime.SignatureAlgorithmBLS_BLS12_381: {
			runtime.HashAlgorithmKMAC128_BLS_BLS12_381: {},
		},
		runtime.SignatureAlgorithmECDSA_P256: {
			runtime.HashAlgorithmSHA2_256:   {},
			runtime.HashAlgorithmSHA3_256:   {},
			runtime.HashAlgorithmKECCAK_256: {},
		},
		runtime.SignatureAlgorithmECDSA_secp256k1: {
			runtime.HashAlgorithmSHA2_256:   {},
			runtime.HashAlgorithmSHA3_256:   {},
			runtime.HashAlgorithmKECCAK_256: {},
		},
	}

	t.Run("verify should fail on incorrect combinations", func(t *testing.T) {

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
			runtime.HashAlgorithmKECCAK_256,
		}

		for _, s := range signatureAlgos {
			for _, h := range hashAlgos {
				t.Run(fmt.Sprintf("combination: %v, %v", s, h), func(t *testing.T) {
					seed := make([]byte, seedLength)
					_, err := rand.Read(seed)
					require.NoError(t, err)
					pk, err := gocrypto.GeneratePrivateKey(crypto.RuntimeToCryptoSigningAlgorithm(s), seed)
					require.NoError(t, err)

					tag := "random_tag"
					var hasher hash.Hasher
					if h != runtime.HashAlgorithmKMAC128_BLS_BLS12_381 {
						hasher, err = crypto.NewPrefixedHashing(crypto.RuntimeToCryptoHashingAlgorithm(h), tag)
						require.NoError(t, err)
					} else {
						hasher = msig.NewBLSHasher(tag)
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

	t.Run("BLS tag combinations", func(t *testing.T) {
		cases := []struct {
			signTag   string
			verifyTag string
			require   func(t *testing.T, sigOk bool, err error)
		}{
			{
				signTag:   "random_tag",
				verifyTag: "random_tag",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			},
			{
				signTag:   "",
				verifyTag: "",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			}, {
				signTag:   "padding test",
				verifyTag: "padding test" + string([]byte{0, 0, 0, 0, 0}),
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			}, {
				signTag:   "valid tag",
				verifyTag: "different valid tag",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			}, {
				signTag:   "a very large tag with more than thirty two bytes",
				verifyTag: "a very large tag with more than thirty two bytes",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			},
		}

		for _, c := range cases {
			seed := make([]byte, seedLength)
			_, err := rand.Read(seed)
			require.NoError(t, err)
			pk, err := gocrypto.GeneratePrivateKey(gocrypto.BLSBLS12381, seed)
			require.NoError(t, err)

			hasher := msig.NewBLSHasher(string(c.signTag))
			signature := make([]byte, 0)
			sig, err := pk.Sign([]byte("some data"), hasher)
			require.NoError(t, err)

			if sig != nil {
				signature = sig.Bytes()
			}

			ok, err := crypto.VerifySignatureFromRuntime(
				signature,
				string(c.verifyTag),
				[]byte("some data"),
				pk.PublicKey().Encode(),
				runtime.SignatureAlgorithmBLS_BLS12_381,
				runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
			)

			c.require(t, ok, err)
		}
	})

	t.Run("ECDSA tag combinations", func(t *testing.T) {

		cases := []struct {
			signTag   string
			verifyTag string
			require   func(t *testing.T, sigOk bool, err error)
		}{
			{
				signTag:   "random_tag",
				verifyTag: "random_tag",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			},
			{
				signTag:   "",
				verifyTag: "",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			}, {
				signTag:   "padding test",
				verifyTag: "padding test" + string([]byte{0, 0, 0, 0, 0}),
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			}, {
				signTag:   "valid tag",
				verifyTag: "different valid tag",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			}, {
				signTag:   "valid tag",
				verifyTag: "a very large tag with more than thirty two bytes",
				require: func(t *testing.T, sigOk bool, err error) {
					require.Error(t, err)
					require.False(t, sigOk)
				},
			},
		}

		for _, c := range cases {
			for s, hMaps := range correctCombinations {
				if s == runtime.SignatureAlgorithmBLS_BLS12_381 {
					// skip BLS to only cover ECDSA in this test
					continue
				}
				for h := range hMaps {
					t.Run(fmt.Sprintf("hash tag: %v, verify tag: %v [%v, %v]", c.signTag, c.verifyTag, s, h), func(t *testing.T) {

						seed := make([]byte, seedLength)
						_, err := rand.Read(seed)
						require.NoError(t, err)
						pk, err := gocrypto.GeneratePrivateKey(crypto.RuntimeToCryptoSigningAlgorithm(s), seed)
						require.NoError(t, err)

						hasher, err := crypto.NewPrefixedHashing(crypto.RuntimeToCryptoHashingAlgorithm(h), c.signTag)
						require.NoError(t, err)

						data := []byte("some data")
						sig, err := pk.Sign(data, hasher)
						require.NoError(t, err)
						signature := sig.Bytes()

						ok, err := crypto.VerifySignatureFromRuntime(
							signature,
							c.verifyTag,
							data,
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

func TestVerifySignatureFromTransaction(t *testing.T) {

	// make sure the seed length is larger than miniumum seed lengths of all signature schemes
	seedLength := 64

	correctCombinations := map[gocrypto.SigningAlgorithm]map[hash.HashingAlgorithm]struct{}{
		gocrypto.ECDSAP256: {
			hash.SHA2_256: {},
			hash.SHA3_256: {},
		},
		gocrypto.ECDSASecp256k1: {
			hash.SHA2_256: {},
			hash.SHA3_256: {},
		},
	}

	t.Run("verify should fail on incorrect combinations", func(t *testing.T) {

		signatureAlgos := []gocrypto.SigningAlgorithm{
			gocrypto.ECDSAP256,
			gocrypto.ECDSASecp256k1,
			gocrypto.BLSBLS12381,
		}
		hashAlgos := []hash.HashingAlgorithm{
			hash.SHA2_256,
			hash.SHA2_384,
			hash.SHA3_256,
			hash.SHA3_384,
			hash.KMAC128,
			hash.Keccak_256,
		}

		for _, s := range signatureAlgos {
			for _, h := range hashAlgos {
				t.Run(fmt.Sprintf("combination: %v, %v", s, h), func(t *testing.T) {
					seed := make([]byte, seedLength)
					_, err := rand.Read(seed)
					require.NoError(t, err)
					sk, err := gocrypto.GeneratePrivateKey(s, seed)
					require.NoError(t, err)

					tag := string(flow.TransactionDomainTag[:])
					var hasher hash.Hasher
					if h != hash.KMAC128 {
						hasher, err = crypto.NewPrefixedHashing(h, tag)
						require.NoError(t, err)
					} else {
						hasher = msig.NewBLSHasher(tag)
					}

					signature := make([]byte, 0)
					data := []byte("some_data")
					sig, err := sk.Sign(data, hasher)
					if _, shouldBeOk := correctCombinations[s][h]; shouldBeOk {
						require.NoError(t, err)
					}

					if sig != nil {
						signature = sig.Bytes()
					}

					ok, err := crypto.VerifySignatureFromTransaction(signature, data, sk.PublicKey(), h)

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

	t.Run("tag combinations", func(t *testing.T) {

		cases := []struct {
			signTag string
			require func(t *testing.T, sigOk bool, err error)
		}{
			{
				signTag: string(flow.TransactionDomainTag[:]),
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.True(t, sigOk)
				},
			},
			{
				signTag: "",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			}, {
				signTag: "random_tag",
				require: func(t *testing.T, sigOk bool, err error) {
					require.NoError(t, err)
					require.False(t, sigOk)
				},
			},
		}

		for _, c := range cases {
			for s, hMaps := range correctCombinations {
				for h := range hMaps {
					t.Run(fmt.Sprintf("sign tag: %v [%v, %v]", c.signTag, s, h), func(t *testing.T) {
						seed := make([]byte, seedLength)
						_, err := rand.Read(seed)
						require.NoError(t, err)
						sk, err := gocrypto.GeneratePrivateKey(s, seed)
						require.NoError(t, err)

						hasher, err := crypto.NewPrefixedHashing(h, c.signTag)
						require.NoError(t, err)

						data := []byte("some data")
						sig, err := sk.Sign(data, hasher)
						require.NoError(t, err)
						signature := sig.Bytes()

						ok, err := crypto.VerifySignatureFromTransaction(signature, data, sk.PublicKey(), h)
						c.require(t, ok, err)
					})
				}
			}
		}
	})
}

func TestValidatePublicKey(t *testing.T) {

	validPublicKey := func(t *testing.T, s runtime.SignatureAlgorithm) []byte {
		seed := make([]byte, gocrypto.KeyGenSeedMinLen)
		_, err := rand.Read(seed)
		require.NoError(t, err)
		sk, err := gocrypto.GeneratePrivateKey(crypto.RuntimeToCryptoSigningAlgorithm(s), seed)
		require.NoError(t, err)
		return sk.PublicKey().Encode()
	}

	t.Run("Unknown algorithm should return false", func(t *testing.T) {
		err := crypto.ValidatePublicKey(runtime.SignatureAlgorithmUnknown, validPublicKey(t, runtime.SignatureAlgorithmECDSA_P256))
		require.Error(t, err)
	})

	t.Run("valid public key should return true", func(t *testing.T) {
		signatureAlgos := []runtime.SignatureAlgorithm{
			runtime.SignatureAlgorithmECDSA_P256,
			runtime.SignatureAlgorithmECDSA_secp256k1,
			runtime.SignatureAlgorithmBLS_BLS12_381,
		}
		for i, s := range signatureAlgos {
			t.Run(fmt.Sprintf("case %v: %v", i, s), func(t *testing.T) {
				err := crypto.ValidatePublicKey(s, validPublicKey(t, s))
				require.NoError(t, err)
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
				// This may cause flakiness depending on the public key
				// deserialization scheme used!!
				key[0] ^= 1 // alter one bit of the valid key
				err := crypto.ValidatePublicKey(s, key)
				require.Errorf(t, err, "key is %#x", key)
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
		runtime.HashAlgorithmKECCAK_256:            hash.Keccak_256,
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

func TestVerifySignatureFromRuntime_error_handling_produces_valid_utf8_for_invalid_sign_algo(t *testing.T) {

	invalidSignatureAlgo := runtime.SignatureAlgorithm(164)

	_, err := crypto.VerifySignatureFromRuntime(
		nil, "", nil, nil, invalidSignatureAlgo, 0,
	)

	require.True(t, errors.IsValueError(err))

	require.Contains(t, err.Error(), fmt.Sprintf("%d", invalidSignatureAlgo))

	errorString := err.Error()
	assert.True(t, utf8.ValidString(errorString))

	// check if they can encoded and decoded using CBOR
	marshalledBytes, err := cbor.Marshal(errorString)
	require.NoError(t, err)

	var unmarshalledString string

	err = cbor.Unmarshal(marshalledBytes, &unmarshalledString)
	require.NoError(t, err)

	require.Equal(t, errorString, unmarshalledString)
}

func TestVerifySignatureFromRuntime_error_handling_produces_valid_utf8_for_invalid_hash_algo(t *testing.T) {

	invalidHashAlgo := runtime.HashAlgorithm(164)

	_, err := crypto.VerifySignatureFromRuntime(
		nil, "", nil, nil, runtime.SignatureAlgorithmECDSA_P256, invalidHashAlgo,
	)

	require.True(t, errors.IsValueError(err))

	require.Contains(t, err.Error(), fmt.Sprintf("%d", invalidHashAlgo))

	errorString := err.Error()
	assert.True(t, utf8.ValidString(errorString))

	// check if they can encoded and decoded using CBOR
	marshalledBytes, err := cbor.Marshal(errorString)
	require.NoError(t, err)

	var unmarshalledString string

	err = cbor.Unmarshal(marshalledBytes, &unmarshalledString)
	require.NoError(t, err)

	require.Equal(t, errorString, unmarshalledString)
}

func TestVerifySignatureFromRuntime_error_handling_produces_valid_utf8_for_invalid_public_key(t *testing.T) {

	invalidPublicKey := []byte{0xc3, 0x28} // some invalid UTF8

	_, err := crypto.VerifySignatureFromRuntime(
		nil, "random_tag", nil, invalidPublicKey, runtime.SignatureAlgorithmECDSA_P256, runtime.HashAlgorithmSHA2_256,
	)

	require.True(t, errors.IsValueError(err))
	errorString := err.Error()

	require.Contains(t, errorString, fmt.Sprintf("%x", invalidPublicKey))

	assert.True(t, utf8.ValidString(errorString))

	// check if they can encoded and decoded using CBOR
	marshalledBytes, err := cbor.Marshal(errorString)
	require.NoError(t, err)

	var unmarshalledString string

	err = cbor.Unmarshal(marshalledBytes, &unmarshalledString)
	require.NoError(t, err)

	require.Equal(t, errorString, unmarshalledString)
}
