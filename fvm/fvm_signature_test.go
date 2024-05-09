package fvm_test

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	fvmCrypto "github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
)

var createMessage = func(m string) (signableMessage []byte, message cadence.Array) {
	signableMessage = []byte(m)
	message = testutil.BytesToCadenceArray(signableMessage)
	return signableMessage, message
}

func TestKeyListSignature(t *testing.T) {

	t.Parallel()

	type signatureAlgorithm struct {
		name       string
		seedLength int
		algorithm  crypto.SigningAlgorithm
	}

	signatureAlgorithms := []signatureAlgorithm{
		{"ECDSA_P256", crypto.KeyGenSeedMinLen, crypto.ECDSAP256},
		{"ECDSA_secp256k1", crypto.KeyGenSeedMinLen, crypto.ECDSASecp256k1},
	}

	type hashAlgorithm struct {
		name   string
		hasher func(string) hash.Hasher
	}

	// Hardcoded tag as required by the crypto.keyList Cadence contract
	// TODO: update to a random tag once the Cadence contract is updated
	// to accept custom tags
	tag := "FLOW-V0.0-user"

	hashAlgorithms := []hashAlgorithm{
		{
			"SHA3_256",
			func(tag string) hash.Hasher {
				hasher, err := fvmCrypto.NewPrefixedHashing(hash.SHA3_256, tag)
				require.Nil(t, err)
				return hasher
			},
		},
		{
			"SHA2_256",
			func(tag string) hash.Hasher {
				hasher, err := fvmCrypto.NewPrefixedHashing(hash.SHA2_256, tag)
				require.Nil(t, err)
				return hasher
			},
		},
		{
			"KECCAK_256",
			func(tag string) hash.Hasher {
				hasher, err := fvmCrypto.NewPrefixedHashing(hash.Keccak_256, tag)
				require.Nil(t, err)
				return hasher
			},
		},
	}

	testForHash := func(signatureAlgorithm signatureAlgorithm, hashAlgorithm hashAlgorithm) {

		code := []byte(
			fmt.Sprintf(
				`
                      import Crypto

                      access(all)
                      fun main(
                          rawPublicKeys: [[UInt8]],
                          message: [UInt8],
                          signatures: [[UInt8]],
                          weight: UFix64,
                      ): Bool {
                          let keyList = Crypto.KeyList()

                          for rawPublicKey in rawPublicKeys {
                              keyList.add(
                                  PublicKey(
                                      publicKey: rawPublicKey,
                                      signatureAlgorithm: SignatureAlgorithm.%s
                                  ),
                                  hashAlgorithm: HashAlgorithm.%s,
                                  weight: weight,
                              )
                          }

                          let signatureSet: [Crypto.KeyListSignature] = []

                          var i = 0
                          for signature in signatures {
                              signatureSet.append(
                                  Crypto.KeyListSignature(
                                      keyIndex: i,
                                      signature: signature
                                  )
                              )
                              i = i + 1
                          }

                          return keyList.verify(
                              signatureSet: signatureSet,
                              signedData: message,
                              domainSeparationTag: "%s"
                          )
                      }
                    `,
				signatureAlgorithm.name,
				hashAlgorithm.name,
				tag,
			),
		)

		t.Run(fmt.Sprintf("%s %s", signatureAlgorithm.name, hashAlgorithm.name), func(t *testing.T) {

			createKey := func() (privateKey crypto.PrivateKey, publicKey cadence.Array) {
				seed := make([]byte, signatureAlgorithm.seedLength)

				var err error

				_, err = rand.Read(seed)
				require.NoError(t, err)

				privateKey, err = crypto.GeneratePrivateKey(signatureAlgorithm.algorithm, seed)
				require.NoError(t, err)

				publicKey = testutil.BytesToCadenceArray(
					privateKey.PublicKey().Encode(),
				)

				return privateKey, publicKey
			}

			signMessage := func(privateKey crypto.PrivateKey, message []byte) cadence.Array {
				signature, err := privateKey.Sign(message, hashAlgorithm.hasher(tag))
				require.NoError(t, err)

				return testutil.BytesToCadenceArray(signature)
			}

			t.Run("Single key", newVMTest().run(
				func(
					t *testing.T,
					vm fvm.VM,
					chain flow.Chain,
					ctx fvm.Context,
					snapshotTree snapshot.SnapshotTree,
				) {
					privateKey, publicKey := createKey()
					signableMessage, message := createMessage("foo")
					signature := signMessage(privateKey, signableMessage)
					weight, _ := cadence.NewUFix64("1.0")

					publicKeys := cadence.NewArray([]cadence.Value{
						publicKey,
					})

					signatures := cadence.NewArray([]cadence.Value{
						signature,
					})

					t.Run("Valid", func(t *testing.T) {
						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.NoError(t, output.Err)

						assert.Equal(t, cadence.NewBool(true), output.Value)
					})

					t.Run("Invalid message", func(t *testing.T) {
						_, invalidRawMessage := createMessage("bar")

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(invalidRawMessage),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.NoError(t, output.Err)

						assert.Equal(t, cadence.NewBool(false), output.Value)
					})

					t.Run("Invalid signature", func(t *testing.T) {
						invalidPrivateKey, _ := createKey()
						invalidRawSignature := signMessage(invalidPrivateKey, signableMessage)

						invalidRawSignatures := cadence.NewArray([]cadence.Value{
							invalidRawSignature,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(invalidRawSignatures),
							jsoncdc.MustEncode(weight),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.NoError(t, output.Err)

						assert.Equal(t, cadence.NewBool(false), output.Value)
					})

					t.Run("Malformed public key", func(t *testing.T) {
						invalidPublicKey := testutil.BytesToCadenceArray([]byte{1, 2, 3})

						invalidPublicKeys := cadence.NewArray([]cadence.Value{
							invalidPublicKey,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(invalidPublicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						require.NoError(t, err)
						require.Error(t, output.Err)
					})
				},
			))

			t.Run("Multiple keys", newVMTest().run(
				func(
					t *testing.T,
					vm fvm.VM,
					chain flow.Chain,
					ctx fvm.Context,
					snapshotTree snapshot.SnapshotTree,
				) {
					privateKeyA, publicKeyA := createKey()
					privateKeyB, publicKeyB := createKey()
					privateKeyC, publicKeyC := createKey()

					publicKeys := cadence.NewArray([]cadence.Value{
						publicKeyA,
						publicKeyB,
						publicKeyC,
					})

					signableMessage, message := createMessage("foo")

					signatureA := signMessage(privateKeyA, signableMessage)
					signatureB := signMessage(privateKeyB, signableMessage)
					signatureC := signMessage(privateKeyC, signableMessage)

					weight, _ := cadence.NewUFix64("0.5")

					t.Run("3 of 3", func(t *testing.T) {
						signatures := cadence.NewArray([]cadence.Value{
							signatureA,
							signatureB,
							signatureC,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.NoError(t, output.Err)

						assert.Equal(t, cadence.NewBool(true), output.Value)
					})

					t.Run("2 of 3", func(t *testing.T) {
						signatures := cadence.NewArray([]cadence.Value{
							signatureA,
							signatureB,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.NoError(t, output.Err)

						assert.Equal(t, cadence.NewBool(true), output.Value)
					})

					t.Run("1 of 3", func(t *testing.T) {
						signatures := cadence.NewArray([]cadence.Value{
							signatureA,
						})

						script := fvm.Script(code).WithArguments(
							jsoncdc.MustEncode(publicKeys),
							jsoncdc.MustEncode(message),
							jsoncdc.MustEncode(signatures),
							jsoncdc.MustEncode(weight),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.NoError(t, output.Err)

						assert.Equal(t, cadence.NewBool(false), output.Value)
					})
				},
			))
		})
	}

	for _, signatureAlgorithm := range signatureAlgorithms {
		for _, hashAlgorithm := range hashAlgorithms {
			testForHash(signatureAlgorithm, hashAlgorithm)
		}
	}

	testForHash(signatureAlgorithm{
		"BLS_BLS12_381",
		crypto.KeyGenSeedMinLen,
		crypto.BLSBLS12381,
	}, hashAlgorithm{
		"KMAC128_BLS_BLS12_381",
		func(tag string) hash.Hasher {
			return msig.NewBLSHasher(tag)
		},
	})
}

func TestBLSMultiSignature(t *testing.T) {

	t.Parallel()

	type signatureAlgorithm struct {
		name       string
		seedLength int
		algorithm  crypto.SigningAlgorithm
	}

	signatureAlgorithms := []signatureAlgorithm{
		{"BLS_BLS12_381", crypto.KeyGenSeedMinLen, crypto.BLSBLS12381},
		{"ECDSA_P256", crypto.KeyGenSeedMinLen, crypto.ECDSAP256},
		{"ECDSA_secp256k1", crypto.KeyGenSeedMinLen, crypto.ECDSASecp256k1},
	}
	BLSSignatureAlgorithm := signatureAlgorithms[0]

	randomSK := func(t *testing.T, signatureAlgorithm signatureAlgorithm) crypto.PrivateKey {
		seed := make([]byte, signatureAlgorithm.seedLength)
		n, err := rand.Read(seed)
		require.Equal(t, n, signatureAlgorithm.seedLength)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(signatureAlgorithm.algorithm, seed)
		require.NoError(t, err)
		return sk
	}

	testVerifyPoP := func() {
		t.Run("verifyBLSPoP", newVMTest().run(
			func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {

				code := func(signatureAlgorithm signatureAlgorithm) []byte {
					return []byte(
						fmt.Sprintf(
							`
								import Crypto
		
								access(all)
								fun main(
									publicKey: [UInt8],
									proof: [UInt8]
								): Bool {
									let p = PublicKey(
										publicKey: publicKey, 
										signatureAlgorithm: SignatureAlgorithm.%s
									)
									return p.verifyPoP(proof)
								}
								`,
							signatureAlgorithm.name,
						),
					)
				}

				t.Run("valid and correct BLS key", func(t *testing.T) {

					sk := randomSK(t, BLSSignatureAlgorithm)
					publicKey := testutil.BytesToCadenceArray(
						sk.PublicKey().Encode(),
					)

					proof, err := crypto.BLSGeneratePOP(sk)
					require.NoError(t, err)
					pop := testutil.BytesToCadenceArray(
						proof,
					)

					script := fvm.Script(code(BLSSignatureAlgorithm)).WithArguments(
						jsoncdc.MustEncode(publicKey),
						jsoncdc.MustEncode(pop),
					)

					_, output, err := vm.Run(ctx, script, snapshotTree)
					assert.NoError(t, err)
					assert.NoError(t, output.Err)
					assert.Equal(t, cadence.NewBool(true), output.Value)

				})

				t.Run("valid but incorrect BLS key", func(t *testing.T) {

					sk := randomSK(t, BLSSignatureAlgorithm)
					publicKey := testutil.BytesToCadenceArray(
						sk.PublicKey().Encode(),
					)

					otherSk := randomSK(t, BLSSignatureAlgorithm)
					proof, err := crypto.BLSGeneratePOP(otherSk)
					require.NoError(t, err)

					pop := testutil.BytesToCadenceArray(
						proof,
					)
					script := fvm.Script(code(BLSSignatureAlgorithm)).WithArguments(
						jsoncdc.MustEncode(publicKey),
						jsoncdc.MustEncode(pop),
					)

					_, output, err := vm.Run(ctx, script, snapshotTree)
					assert.NoError(t, err)
					assert.NoError(t, output.Err)
					assert.Equal(t, cadence.NewBool(false), output.Value)

				})

				for _, signatureAlgorithm := range signatureAlgorithms[1:] {
					t.Run("valid non BLS key/"+signatureAlgorithm.name, func(t *testing.T) {
						sk := randomSK(t, signatureAlgorithm)
						publicKey := testutil.BytesToCadenceArray(
							sk.PublicKey().Encode(),
						)

						random := make([]byte, crypto.SignatureLenBLSBLS12381)
						_, err := rand.Read(random)
						require.NoError(t, err)
						pop := testutil.BytesToCadenceArray(
							random,
						)

						script := fvm.Script(code(signatureAlgorithm)).WithArguments(
							jsoncdc.MustEncode(publicKey),
							jsoncdc.MustEncode(pop),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.Error(t, output.Err)
					})
				}
			},
		))
	}

	testBLSSignatureAggregation := func() {
		t.Run("aggregateBLSSignatures", newVMTest().run(
			func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {

				code := []byte(
					`
							import Crypto
	
							access(all) fun main(
							signatures: [[UInt8]],
							): [UInt8]? {
								return BLS.aggregateSignatures(signatures)!
							}
						`,
				)

				// random message
				input := make([]byte, 100)
				_, err := rand.Read(input)
				require.NoError(t, err)

				// generate keys and signatures
				numSigs := 50
				sigs := make([]crypto.Signature, 0, numSigs)

				kmac := msig.NewBLSHasher("test tag")
				for i := 0; i < numSigs; i++ {
					sk := randomSK(t, BLSSignatureAlgorithm)
					// a valid BLS signature
					s, err := sk.Sign(input, kmac)
					require.NoError(t, err)
					sigs = append(sigs, s)
				}

				t.Run("valid BLS signatures", func(t *testing.T) {

					signatures := make([]cadence.Value, 0, numSigs)
					for _, sig := range sigs {
						s := testutil.BytesToCadenceArray(sig)
						signatures = append(signatures, s)
					}

					script := fvm.Script(code).WithArguments(
						jsoncdc.MustEncode(cadence.Array{
							Values: signatures,
							ArrayType: &cadence.VariableSizedArrayType{
								ElementType: &cadence.VariableSizedArrayType{
									ElementType: cadence.UInt8Type,
								},
							},
						}),
					)

					_, output, err := vm.Run(ctx, script, snapshotTree)
					assert.NoError(t, err)
					assert.NoError(t, output.Err)

					expectedSig, err := crypto.AggregateBLSSignatures(sigs)
					require.NoError(t, err)
					assert.Equal(t, cadence.Optional{Value: testutil.BytesToCadenceArray(expectedSig)}, output.Value)
				})

				t.Run("at least one invalid BLS signature", func(t *testing.T) {

					signatures := make([]cadence.Value, 0, numSigs)
					// alter one random signature
					tmp := sigs[numSigs/2]
					sigs[numSigs/2] = crypto.BLSInvalidSignature()

					for _, sig := range sigs {
						s := testutil.BytesToCadenceArray(sig)
						signatures = append(signatures, s)
					}

					script := fvm.Script(code).WithArguments(
						jsoncdc.MustEncode(cadence.Array{
							Values: signatures,
							ArrayType: &cadence.VariableSizedArrayType{
								ElementType: &cadence.VariableSizedArrayType{
									ElementType: cadence.UInt8Type,
								},
							},
						}),
					)

					// revert the change
					sigs[numSigs/2] = tmp

					_, output, err := vm.Run(ctx, script, snapshotTree)
					assert.NoError(t, err)
					assert.Error(t, output.Err)
					assert.Equal(t, nil, output.Value)
				})

				t.Run("empty signature list", func(t *testing.T) {

					signatures := []cadence.Value{}
					script := fvm.Script(code).WithArguments(
						jsoncdc.MustEncode(cadence.Array{
							Values: signatures,
							ArrayType: &cadence.VariableSizedArrayType{
								ElementType: &cadence.VariableSizedArrayType{
									ElementType: cadence.UInt8Type,
								},
							},
						}),
					)

					_, output, err := vm.Run(ctx, script, snapshotTree)
					assert.NoError(t, err)
					assert.Error(t, output.Err)
					assert.Equal(t, nil, output.Value)
				})
			},
		))
	}

	testKeyAggregation := func() {
		t.Run("aggregateBLSPublicKeys", newVMTest().run(
			func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {

				code := func(signatureAlgorithm signatureAlgorithm) []byte {
					return []byte(
						fmt.Sprintf(
							`
								import Crypto
		
								access(all) fun main(
									publicKeys: [[UInt8]]
								): [UInt8]? {
									let pks: [PublicKey] = []
									for pk in publicKeys {
										pks.append(PublicKey(
											publicKey: pk, 
											signatureAlgorithm: SignatureAlgorithm.%s
										))
									}
									return BLS.aggregatePublicKeys(pks)!.publicKey
								}
								`,
							signatureAlgorithm.name,
						),
					)
				}

				pkNum := 100
				pks := make([]crypto.PublicKey, 0, pkNum)

				t.Run("valid BLS keys", func(t *testing.T) {

					publicKeys := make([]cadence.Value, 0, pkNum)
					for i := 0; i < pkNum; i++ {
						sk := randomSK(t, BLSSignatureAlgorithm)
						pk := sk.PublicKey()
						pks = append(pks, pk)
						publicKeys = append(
							publicKeys,
							testutil.BytesToCadenceArray(pk.Encode()),
						)
					}

					script := fvm.Script(code(BLSSignatureAlgorithm)).WithArguments(
						jsoncdc.MustEncode(cadence.Array{
							Values: publicKeys,
							ArrayType: &cadence.VariableSizedArrayType{
								ElementType: &cadence.VariableSizedArrayType{
									ElementType: cadence.UInt8Type,
								},
							},
						}),
					)

					_, output, err := vm.Run(ctx, script, snapshotTree)
					assert.NoError(t, err)
					assert.NoError(t, output.Err)
					expectedPk, err := crypto.AggregateBLSPublicKeys(pks)
					require.NoError(t, err)

					assert.Equal(t, cadence.Optional{Value: testutil.BytesToCadenceArray(expectedPk.Encode())}, output.Value)
				})

				for _, signatureAlgorithm := range signatureAlgorithms[1:] {
					t.Run("non BLS keys/"+signatureAlgorithm.name, func(t *testing.T) {

						publicKeys := make([]cadence.Value, 0, pkNum)
						for i := 0; i < pkNum; i++ {
							sk := randomSK(t, signatureAlgorithm)
							pk := sk.PublicKey()
							pks = append(pks, pk)
							publicKeys = append(
								publicKeys,
								testutil.BytesToCadenceArray(sk.PublicKey().Encode()),
							)
						}

						script := fvm.Script(code(signatureAlgorithm)).WithArguments(
							jsoncdc.MustEncode(cadence.Array{
								Values: publicKeys,
								ArrayType: &cadence.VariableSizedArrayType{
									ElementType: &cadence.VariableSizedArrayType{
										ElementType: cadence.UInt8Type,
									},
								},
							}),
						)

						_, output, err := vm.Run(ctx, script, snapshotTree)
						assert.NoError(t, err)
						assert.Error(t, output.Err)
					})
				}

				t.Run("empty list", func(t *testing.T) {

					var publicKeys []cadence.Value
					script := fvm.Script(code(BLSSignatureAlgorithm)).WithArguments(
						jsoncdc.MustEncode(cadence.Array{
							Values: publicKeys,
							ArrayType: &cadence.VariableSizedArrayType{
								ElementType: &cadence.VariableSizedArrayType{
									ElementType: cadence.UInt8Type,
								},
							},
						}),
					)

					_, output, err := vm.Run(ctx, script, snapshotTree)
					assert.NoError(t, err)
					assert.Error(t, output.Err)
					assert.Equal(t, nil, output.Value)
				})
			},
		))
	}

	testBLSCombinedAggregations := func() {
		t.Run("Combined Aggregations", newVMTest().run(
			func(
				t *testing.T,
				vm fvm.VM,
				chain flow.Chain,
				ctx fvm.Context,
				snapshotTree snapshot.SnapshotTree,
			) {

				message, cadenceMessage := createMessage("random_message")
				tag := "random_tag"

				code := []byte(`
							import Crypto

							access(all) fun main(
								publicKeys: [[UInt8]],
								signatures: [[UInt8]],
								message:  [UInt8],
								tag: String,
							): Bool {
								let pks: [PublicKey] = []
								for pk in publicKeys {
									pks.append(PublicKey(
										publicKey: pk,
										signatureAlgorithm: SignatureAlgorithm.BLS_BLS12_381
									))
								}
								let aggPk = BLS.aggregatePublicKeys(pks)!
								let aggSignature = BLS.aggregateSignatures(signatures)!
								let boo = aggPk.verify(
									signature: aggSignature, 
									signedData: message, 
									domainSeparationTag: tag, 
									hashAlgorithm: HashAlgorithm.KMAC128_BLS_BLS12_381)
								return boo
							}
							`)

				num := 50
				publicKeys := make([]cadence.Value, 0, num)
				signatures := make([]cadence.Value, 0, num)

				kmac := msig.NewBLSHasher(string(tag))
				for i := 0; i < num; i++ {
					sk := randomSK(t, BLSSignatureAlgorithm)
					pk := sk.PublicKey()
					publicKeys = append(
						publicKeys,
						testutil.BytesToCadenceArray(pk.Encode()),
					)
					sig, err := sk.Sign(message, kmac)
					require.NoError(t, err)
					signatures = append(
						signatures,
						testutil.BytesToCadenceArray(sig),
					)
				}

				script := fvm.Script(code).WithArguments(
					jsoncdc.MustEncode(cadence.Array{ // keys
						Values: publicKeys,
						ArrayType: &cadence.VariableSizedArrayType{
							ElementType: &cadence.VariableSizedArrayType{
								ElementType: cadence.UInt8Type,
							},
						},
					}),
					jsoncdc.MustEncode(cadence.Array{ // signatures
						Values: signatures,
						ArrayType: &cadence.VariableSizedArrayType{
							ElementType: &cadence.VariableSizedArrayType{
								ElementType: cadence.UInt8Type,
							},
						},
					}),
					jsoncdc.MustEncode(cadenceMessage),
					jsoncdc.MustEncode(cadence.String(tag)),
				)

				_, output, err := vm.Run(ctx, script, snapshotTree)
				assert.NoError(t, err)
				assert.NoError(t, output.Err)
				assert.Equal(t, cadence.NewBool(true), output.Value)
			},
		))
	}

	testVerifyPoP()
	testKeyAggregation()
	testBLSSignatureAggregation()
	testBLSCombinedAggregations()
}
