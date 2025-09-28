package flow_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	mrand "math/rand"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/go-ethereum/rlp"

	fvmCrypto "github.com/onflow/flow-go/fvm/crypto"
	modelrlp "github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransaction_SignatureOrdering(t *testing.T) {

	proposerAddress := unittest.RandomAddressFixture()
	proposerKeyIndex := uint32(1)
	proposerSequenceNumber := uint64(42)
	proposerSignature := []byte{1, 2, 3}

	authorizerAddress := unittest.RandomAddressFixture()
	authorizerKeyIndex := uint32(0)
	authorizerSignature := []byte{4, 5, 6}

	payerAddress := unittest.RandomAddressFixture()
	payerKeyIndex := uint32(0)
	payerSignature := []byte{7, 8, 9}

	tx, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(`transaction(){}`)).
		SetProposalKey(proposerAddress, proposerKeyIndex, proposerSequenceNumber).
		AddPayloadSignature(proposerAddress, proposerKeyIndex, proposerSignature).
		SetPayer(payerAddress).
		AddEnvelopeSignature(payerAddress, payerKeyIndex, payerSignature).
		AddAuthorizer(authorizerAddress).
		AddPayloadSignature(authorizerAddress, authorizerKeyIndex, authorizerSignature).
		Build()
	require.NoError(t, err)

	require.Len(t, tx.PayloadSignatures, 2)

	signatureA := tx.PayloadSignatures[0]
	signatureB := tx.PayloadSignatures[1]

	assert.Equal(t, proposerAddress, signatureA.Address)
	assert.Equal(t, authorizerAddress, signatureB.Address)
}

func TestTransaction_Status(t *testing.T) {
	statuses := map[flow.TransactionStatus]string{
		flow.TransactionStatusUnknown:   "UNKNOWN",
		flow.TransactionStatusPending:   "PENDING",
		flow.TransactionStatusFinalized: "FINALIZED",
		flow.TransactionStatusExecuted:  "EXECUTED",
		flow.TransactionStatusSealed:    "SEALED",
		flow.TransactionStatusExpired:   "EXPIRED",
	}

	for status, value := range statuses {
		assert.Equal(t, status.String(), value)
	}
}

// TestTransactionAuthenticationSchemes tests transaction signature verifications
// with a focus on authentication schemes.
func TestTransactionAuthenticationSchemes(t *testing.T) {
	seedLength := 32
	h := hash.SHA2_256
	s := crypto.ECDSAP256
	payerAddress := unittest.AddressFixture()
	authorizerAddress := flow.EmptyAddress
	require.NotEqual(t, payerAddress, authorizerAddress)

	transactionBody := flow.TransactionBody{
		Script: []byte("some script"),
		Arguments: [][]byte{
			[]byte("arg1"),
		},
		ReferenceBlockID: flow.HashToID([]byte("some block id")),
		GasLimit:         1000,
		Payer:            payerAddress,
		ProposalKey: flow.ProposalKey{
			Address:        authorizerAddress,
			KeyIndex:       0,
			SequenceNumber: 0,
		},
		Authorizers: []flow.Address{
			authorizerAddress,
		},
		PayloadSignatures: []flow.TransactionSignature{
			{
				Address:       authorizerAddress,
				KeyIndex:      0,
				Signature:     []byte("signature"), // Mock signature, not validated
				SignerIndex:   0,
				ExtensionData: unittest.RandomBytes(3),
			},
		},
		EnvelopeSignatures: []flow.TransactionSignature{
			{
				Address:       payerAddress,
				KeyIndex:      0,
				Signature:     []byte("placeholder"),
				SignerIndex:   0,
				ExtensionData: unittest.RandomBytes(3),
			},
		},
	}

	// test transaction envelope canonical form constructions
	t.Run("Transaction envelope canonical form", func(t *testing.T) {

		legacyEnvelopeSignatureCanonicalForm := func(tb flow.TransactionBody) []byte {
			return fingerprint.Fingerprint(struct {
				Payload           interface{}
				PayloadSignatures interface{}
			}{
				tb.PayloadCanonicalForm(),
				[]interface{}{
					// Expected canonical form of payload signature
					struct {
						SignerIndex uint
						KeyID       uint
						Signature   []byte
					}{
						SignerIndex: uint(tb.PayloadSignatures[0].SignerIndex),
						KeyID:       uint(tb.PayloadSignatures[0].KeyIndex),
						Signature:   tb.PayloadSignatures[0].Signature,
					},
				},
			})
		}
		randomExtensionData := unittest.RandomBytes(20)

		cases := []struct {
			payloadExtensionData                   []byte
			expectedEnvelopeSignatureCanonicalForm func(tb flow.TransactionBody) []byte
		}{
			{
				// nil extension data
				payloadExtensionData:                   nil,
				expectedEnvelopeSignatureCanonicalForm: legacyEnvelopeSignatureCanonicalForm,
			}, {
				// empty extension data
				payloadExtensionData:                   []byte{},
				expectedEnvelopeSignatureCanonicalForm: legacyEnvelopeSignatureCanonicalForm,
			}, {
				// zero scheme identifier
				payloadExtensionData:                   []byte{0x0},
				expectedEnvelopeSignatureCanonicalForm: legacyEnvelopeSignatureCanonicalForm,
			}, {
				// zero scheme identifier but invalid extension (should be taken into account in the canonical form)
				payloadExtensionData: []byte{0x0, 1, 2, 3},
				expectedEnvelopeSignatureCanonicalForm: func(tb flow.TransactionBody) []byte {
					return fingerprint.Fingerprint(struct {
						Payload           interface{}
						PayloadSignatures interface{}
					}{
						tb.PayloadCanonicalForm(),
						[]interface{}{
							// Expected canonical form of payload signature
							struct {
								SignerIndex   uint
								KeyID         uint
								Signature     []byte
								ExtensionData []byte
							}{
								SignerIndex:   uint(tb.PayloadSignatures[0].SignerIndex),
								KeyID:         uint(tb.PayloadSignatures[0].KeyIndex),
								Signature:     tb.PayloadSignatures[0].Signature,
								ExtensionData: []byte{0x0, 1, 2, 3},
							},
						},
					})
				},
			}, {
				// non-plain authentication scheme
				payloadExtensionData: slices.Concat([]byte{0x5}, randomExtensionData[:]),
				expectedEnvelopeSignatureCanonicalForm: func(tb flow.TransactionBody) []byte {
					return fingerprint.Fingerprint(struct {
						Payload           interface{}
						PayloadSignatures interface{}
					}{
						tb.PayloadCanonicalForm(),
						[]interface{}{
							// Expected canonical form of payload signature
							struct {
								SignerIndex   uint
								KeyID         uint
								Signature     []byte
								ExtensionData []byte
							}{
								SignerIndex:   uint(tb.PayloadSignatures[0].SignerIndex),
								KeyID:         uint(tb.PayloadSignatures[0].KeyIndex),
								Signature:     tb.PayloadSignatures[0].Signature,
								ExtensionData: slices.Concat([]byte{0x5}, randomExtensionData[:]),
							},
						},
					})
				},
			}, {
				// webauthn scheme
				payloadExtensionData: slices.Concat([]byte{0x1}, randomExtensionData[:]),
				expectedEnvelopeSignatureCanonicalForm: func(tb flow.TransactionBody) []byte {
					return fingerprint.Fingerprint(struct {
						Payload           interface{}
						PayloadSignatures interface{}
					}{
						tb.PayloadCanonicalForm(),
						[]interface{}{
							// Expected canonical form of payload signature
							struct {
								SignerIndex   uint
								KeyID         uint
								Signature     []byte
								ExtensionData []byte
							}{
								SignerIndex:   uint(tb.PayloadSignatures[0].SignerIndex),
								KeyID:         uint(tb.PayloadSignatures[0].KeyIndex),
								Signature:     tb.PayloadSignatures[0].Signature,
								ExtensionData: slices.Concat([]byte{0x1}, randomExtensionData[:]),
							},
						},
					})
				},
			},
		}
		// test all cases
		for _, c := range cases {
			t.Run(fmt.Sprintf("auth scheme, payloadExtensionData: %v", c.payloadExtensionData), func(t *testing.T) {
				transactionBody.PayloadSignatures[0].ExtensionData = c.payloadExtensionData
				transactionMessage := transactionBody.EnvelopeMessage()

				// generate expected envelope data
				expectedEnvelopeMessage := c.expectedEnvelopeSignatureCanonicalForm(transactionBody)
				// compare canonical forms
				require.Equal(t, transactionMessage, expectedEnvelopeMessage)
			})
		}
	})

	// test transaction canonical form constructions (ID computation)
	t.Run("Transaction canonical form", func(t *testing.T) {

		legacyTransactionCanonicalForm := func(tb flow.TransactionBody) []byte {
			return fingerprint.Fingerprint(struct {
				Payload            interface{}
				PayloadSignatures  interface{}
				EnvelopeSignatures interface{}
			}{
				tb.PayloadCanonicalForm(),
				[]interface{}{
					// Expected canonical form of payload signature
					struct {
						SignerIndex uint
						KeyID       uint
						Signature   []byte
					}{
						SignerIndex: uint(tb.PayloadSignatures[0].SignerIndex),
						KeyID:       uint(tb.PayloadSignatures[0].KeyIndex),
						Signature:   tb.PayloadSignatures[0].Signature,
					},
				},
				[]interface{}{
					// Expected canonical form of payload signature
					struct {
						SignerIndex uint
						KeyID       uint
						Signature   []byte
					}{
						SignerIndex: uint(tb.EnvelopeSignatures[0].SignerIndex),
						KeyID:       uint(tb.EnvelopeSignatures[0].KeyIndex),
						Signature:   tb.EnvelopeSignatures[0].Signature,
					},
				},
			})
		}

		randomExtensionData := unittest.RandomBytes(20)

		cases := []struct {
			payloadExtensionData             []byte
			expectedTransactionCanonicalForm func(tb flow.TransactionBody) []byte
		}{
			// nil extension data
			{
				payloadExtensionData:             nil,
				expectedTransactionCanonicalForm: legacyTransactionCanonicalForm,
			},
			// empty extension data
			{
				payloadExtensionData:             []byte{},
				expectedTransactionCanonicalForm: legacyTransactionCanonicalForm,
			}, {
				// zero extension data
				payloadExtensionData:             []byte{0x0},
				expectedTransactionCanonicalForm: legacyTransactionCanonicalForm,
			}, {
				// webauthn scheme
				payloadExtensionData: slices.Concat([]byte{0x1}, randomExtensionData[:]),
				expectedTransactionCanonicalForm: func(tb flow.TransactionBody) []byte {
					return fingerprint.Fingerprint(struct {
						Payload            interface{}
						PayloadSignatures  interface{}
						EnvelopeSignatures interface{}
					}{
						tb.PayloadCanonicalForm(),
						[]interface{}{
							// Expected canonical form of payload signature
							struct {
								SignerIndex   uint
								KeyID         uint
								Signature     []byte
								ExtensionData []byte
							}{
								SignerIndex:   uint(tb.PayloadSignatures[0].SignerIndex),
								KeyID:         uint(tb.PayloadSignatures[0].KeyIndex),
								Signature:     tb.PayloadSignatures[0].Signature,
								ExtensionData: slices.Concat([]byte{0x1}, randomExtensionData[:]),
							},
						},
						[]interface{}{
							// Expected canonical form of payload signature
							struct {
								SignerIndex   uint
								KeyID         uint
								Signature     []byte
								ExtensionData []byte
							}{
								SignerIndex:   uint(tb.EnvelopeSignatures[0].SignerIndex),
								KeyID:         uint(tb.EnvelopeSignatures[0].KeyIndex),
								Signature:     tb.EnvelopeSignatures[0].Signature,
								ExtensionData: slices.Concat([]byte{0x1}, randomExtensionData[:]),
							},
						},
					})
				},
			},
		}
		// test all cases
		for _, c := range cases {
			t.Run(fmt.Sprintf("auth scheme (payloadExtensionData): %v", c.payloadExtensionData), func(t *testing.T) {
				transactionBody.PayloadSignatures[0].ExtensionData = c.payloadExtensionData
				transactionBody.EnvelopeSignatures[0].ExtensionData = c.payloadExtensionData
				transactionID := transactionBody.ID()

				// generate expected envelope data
				sha3 := hash.NewSHA3_256()
				expectedID := flow.Identifier(sha3.ComputeHash(c.expectedTransactionCanonicalForm(transactionBody)))
				// compare canonical forms
				require.Equal(t, transactionID, expectedID)
			})
		}
	})

	// test `VerifySignatureFromTransaction` in the plain authentication scheme
	t.Run("plain authentication scheme", func(t *testing.T) {
		cases := []struct {
			payloadExtensionData []byte
			extensionOk          bool
			signatureOk          bool
		}{
			{
				// nil extension data
				payloadExtensionData: nil,
				extensionOk:          true,
				signatureOk:          true,
			},
			{
				// empty extension data
				payloadExtensionData: []byte{},
				extensionOk:          true,
				signatureOk:          true,
			}, {
				// correct extension data
				payloadExtensionData: []byte{0x0},
				extensionOk:          true,
				signatureOk:          true,
			}, {
				// incorrect extension data
				payloadExtensionData: []byte{0x1},
				extensionOk:          false,
				signatureOk:          false,
			}, {
				// incorrect extension data: correct identifier but with extra bytes
				payloadExtensionData: []byte{0, 1, 2, 3},
				extensionOk:          false,
				signatureOk:          false,
			},
		}
		// test all cases
		for _, c := range cases {
			// payload data (the transaction envelope to sign/verify)
			payload := unittest.RandomBytes(20)
			t.Run(fmt.Sprintf("auth scheme (payloadExtensionData): %v", c.payloadExtensionData), func(t *testing.T) {
				seed := make([]byte, seedLength)
				_, err := rand.Read(seed)
				require.NoError(t, err)
				sk, err := crypto.GeneratePrivateKey(s, seed)
				require.NoError(t, err)
				hasher, err := fvmCrypto.NewPrefixedHashing(h, flow.TransactionTagString)
				require.NoError(t, err)
				signature, err := sk.Sign(payload, hasher)
				require.NoError(t, err)

				transactionBody.PayloadSignatures[0].ExtensionData = c.payloadExtensionData
				transactionBody.PayloadSignatures[0].Signature = signature

				extensionDataValid, message := transactionBody.PayloadSignatures[0].ValidateExtensionDataAndReconstructMessage(payload)
				signatureValid, err := fvmCrypto.VerifySignatureFromTransaction(signature, message, sk.PublicKey(), h)

				require.NoError(t, err)
				require.Equal(t, c.extensionOk, extensionDataValid)
				if c.extensionOk {
					require.Equal(t, c.signatureOk, signatureValid)
				}
			})
		}
	})

	// test `VerifySignatureFromTransaction` in the WebAuthn authentication scheme
	t.Run("webauthn authentication scheme", func(t *testing.T) {
		hasher, err := fvmCrypto.NewPrefixedHashing(hash.SHA2_256, flow.TransactionTagString)
		require.NoError(t, err)

		transactionMessage := transactionBody.EnvelopeMessage()
		authNChallenge := hasher.ComputeHash(transactionMessage)
		authNChallengeBase64Url := base64.RawURLEncoding.EncodeToString(authNChallenge)
		validUserFlag := byte(0x01)
		validClientDataOrigin := "https://testing.com"
		rpIDHash := unittest.RandomBytes(32)
		sigCounter := unittest.RandomBytes(4)

		// For use in cases where you're testing the other value
		validAuthenticatorData := slices.Concat(rpIDHash, []byte{validUserFlag}, sigCounter)
		validClientDataJSON := map[string]string{
			"type":      flow.WebAuthnTypeGet,
			"challenge": authNChallengeBase64Url,
			"origin":    validClientDataOrigin,
		}

		cases := []struct {
			description       string
			authenticatorData []byte
			clientDataJSON    map[string]string
			extensionOk       bool
			signatureOk       bool
		}{
			{
				description:       "Cannot be just the scheme, not enough extension data",
				authenticatorData: []byte{},
				clientDataJSON:    map[string]string{},
				extensionOk:       false,
				signatureOk:       false,
			}, {
				description:       "invalid user flag, UP not set",
				authenticatorData: slices.Concat(rpIDHash, []byte{0x0}, sigCounter),
				clientDataJSON:    validClientDataJSON,
				extensionOk:       false,
				signatureOk:       false,
			}, {
				description:       "invalid user flag, extensions exist but flag AT and ED are not set",
				authenticatorData: slices.Concat(rpIDHash, []byte{validUserFlag}, sigCounter, unittest.RandomBytes(1+mrand.Intn(20))),
				clientDataJSON:    validClientDataJSON,
				extensionOk:       false,
				signatureOk:       false,
			}, {
				description:       "invalid user flag, extensions do not exist but flag AT is set",
				authenticatorData: slices.Concat(rpIDHash, []byte{validUserFlag | 0x40}, sigCounter),
				clientDataJSON: map[string]string{
					"type":      flow.WebAuthnTypeGet,
					"challenge": authNChallengeBase64Url,
					"origin":    validClientDataOrigin,
				},
				extensionOk: false,
				signatureOk: false,
			}, {
				description:       "invalid user flag, extensions do not exist but flag ED is set",
				authenticatorData: slices.Concat(rpIDHash, []byte{validUserFlag | 0x80}, sigCounter),
				clientDataJSON: map[string]string{
					"type":      flow.WebAuthnTypeGet,
					"challenge": authNChallengeBase64Url,
					"origin":    validClientDataOrigin,
				},
				extensionOk: false,
				signatureOk: false,
			}, {
				description:       "invalid client data type",
				authenticatorData: validAuthenticatorData,
				clientDataJSON: map[string]string{
					"type":      "invalid_type",
					"challenge": authNChallengeBase64Url,
					"origin":    validClientDataOrigin,
				},
				extensionOk: false,
				signatureOk: false,
			}, {
				description:       "empty origin (valid)",
				authenticatorData: validAuthenticatorData,
				clientDataJSON: map[string]string{
					"type":      flow.WebAuthnTypeGet,
					"challenge": authNChallengeBase64Url,
					"origin":    "",
				},
				extensionOk: true,
				signatureOk: true,
			}, {
				description:       "valid authn scheme signature",
				authenticatorData: validAuthenticatorData,
				clientDataJSON:    validClientDataJSON,
				extensionOk:       true,
				signatureOk:       true,
			},
			{
				description:       "more client data fields",
				authenticatorData: validAuthenticatorData,
				clientDataJSON: map[string]string{
					"type":      flow.WebAuthnTypeGet,
					"challenge": authNChallengeBase64Url,
					"origin":    validClientDataOrigin,
					"other1":    "random",
					"other2":    "random",
				},
				extensionOk: true,
				signatureOk: true,
			},
		}

		// run all cases above
		for _, c := range cases {
			t.Run(fmt.Sprintf("auth scheme - %s (authenticatorData)", c.description), func(t *testing.T) {
				// This will be the equivalent of possible client side actions, while mocking out the majority of
				// the webauthn process.
				// Could eventually consider using flow-go-sdk here if it makes sense
				seed := make([]byte, seedLength)
				_, err := rand.Read(seed)
				require.NoError(t, err)
				sk, err := crypto.GeneratePrivateKey(s, seed)
				require.NoError(t, err)

				// generate the extension data, based on the client data and authenticator data indicated by the test case
				clientDataJsonBytes, err := json.Marshal(c.clientDataJSON)
				require.NoError(t, err)

				extensionData := flow.WebAuthnExtensionData{
					AuthenticatorData: c.authenticatorData,
					ClientDataJson:    clientDataJsonBytes,
				}

				// RLP Encode the extension data
				// This is the equivalent of the client side encoding
				extensionDataRLPBytes := modelrlp.NewMarshaler().MustMarshal(extensionData)

				// Construct the message to sign in the same way a client would, as per
				// https://github.com/onflow/flips/blob/tarak/webauthn/protocol/20250203-webauthn-credential-support.md#fvm-transaction-validation-changes
				var clientDataHash [hash.HashLenSHA2_256]byte
				hash.ComputeSHA2_256(&clientDataHash, clientDataJsonBytes)
				messageToSign := slices.Concat(c.authenticatorData, clientDataHash[:])

				// Sign as "client"
				accountHasher, err := fvmCrypto.NewPrefixedHashing(h, "")
				require.NoError(t, err)
				signature, err := sk.Sign(messageToSign, accountHasher)
				require.NoError(t, err)

				// Verify as "server"
				transactionBody.PayloadSignatures[0].ExtensionData = slices.Concat([]byte{byte(flow.WebAuthnScheme)}, extensionDataRLPBytes[:])
				transactionBody.PayloadSignatures[0].Signature = signature

				extensionDataValid, message := transactionBody.PayloadSignatures[0].ValidateExtensionDataAndReconstructMessage(transactionMessage)
				signatureValid, err := fvmCrypto.VerifySignatureFromTransaction(signature, message, sk.PublicKey(), h)
				require.NoError(t, err)
				require.Equal(t, c.extensionOk, extensionDataValid)
				if c.extensionOk {
					require.Equal(t, c.signatureOk, signatureValid)
				}
			})
		}
	})

	t.Run("invalid authentication schemes", func(t *testing.T) {
		cases := []struct {
			description string
			scheme      flow.AuthenticationScheme
			extensionOk bool
		}{
			{
				description: "invalid scheme (0x02)",
				scheme:      flow.InvalidScheme,
				extensionOk: false,
			}, {
				description: "invalid scheme, parsed using AuthenticationSchemeFromByte (0xFF)",
				scheme:      flow.AuthenticationSchemeFromByte(0xFF),
				extensionOk: false,
			},
		}

		for _, c := range cases {

			t.Run(fmt.Sprintf("%s - auth scheme - %v", c.description, c.scheme), func(t *testing.T) {
				// apply the extention
				transactionBody.PayloadSignatures[0].ExtensionData = []byte{byte(c.scheme)}

				// Validate the transaction signature extension
				transactionMessage := unittest.RandomBytes(20)
				extensionDataValid, message := transactionBody.PayloadSignatures[0].ValidateExtensionDataAndReconstructMessage(transactionMessage)
				require.Nil(t, message)
				require.Equal(t, c.extensionOk, extensionDataValid)
			})

		}
	})
}

// TestTransactionBodyID_Malleability provides basic validation that [flow.TransactionBody] is not malleable.
func TestTransactionBodyID_Malleability(t *testing.T) {
	txbody := unittest.TransactionBodyFixture()
	unittest.RequireEntityNonMalleable(t, &txbody, unittest.WithTypeGenerator[flow.TransactionSignature](func() flow.TransactionSignature {
		return unittest.TransactionSignatureFixture()
	}))
}

// TestTransactionBody_Fingerprint provides basic validation that the [TransactionBody] fingerprint
// is equivalent to its canonical RLP encoding.
func TestTransactionBody_Fingerprint(t *testing.T) {
	txbody := unittest.TransactionBodyFixture()
	fp1 := txbody.Fingerprint()
	fp2 := fingerprint.Fingerprint(txbody)
	fp3, err := rlp.EncodeToBytes(txbody)
	require.NoError(t, err)
	assert.Equal(t, fp1, fp2)
	assert.Equal(t, fp2, fp3)
}

// TestNewTransactionBody verifies that NewTransactionBody constructs a valid TransactionBody
// when given all required fields, and returns an error if any mandatory field is missing.
//
// Test Cases:
//
// 1. Valid input:
//   - Payer is non-empty and Script is non-empty.
//   - Ensures a TransactionBody is returned with all fields populated correctly.
//
// 2. Empty Script:
//   - Script slice is empty.
//   - Ensures an error is returned mentioning "Script must not be empty".
func TestNewTransactionBody(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture()

		tb, err := flow.NewTransactionBody(utb)
		assert.NoError(t, err)
		assert.NotNil(t, tb)

		assert.Equal(t, flow.TransactionBody(utb), *tb)
	})

	t.Run("empty Script", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture(func(u *flow.UntrustedTransactionBody) {
			u.Script = []byte{}
		})

		tb, err := flow.NewTransactionBody(utb)
		assert.Error(t, err)
		assert.Nil(t, tb)
		assert.Contains(t, err.Error(), "Script must not be empty")
	})
}

// UntrustedTransactionBodyFixture returns an UntrustedTransactionBody
// pre‐populated with sane defaults. Any opts override those defaults.
func UntrustedTransactionBodyFixture(opts ...func(*flow.UntrustedTransactionBody)) flow.UntrustedTransactionBody {
	u := flow.UntrustedTransactionBody(unittest.TransactionBodyFixture())
	for _, opt := range opts {
		opt(&u)
	}
	return u
}
