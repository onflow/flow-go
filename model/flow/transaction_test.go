package flow_test

import (
	"crypto/rand"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/go-ethereum/rlp"

	fvmCrypto "github.com/onflow/flow-go/fvm/crypto"
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
				Payload           any
				PayloadSignatures any
			}{
				tb.PayloadCanonicalForm(),
				[]any{
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
						Payload           any
						PayloadSignatures any
					}{
						tb.PayloadCanonicalForm(),
						[]any{
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
						Payload           any
						PayloadSignatures any
					}{
						tb.PayloadCanonicalForm(),
						[]any{
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
				Payload            any
				PayloadSignatures  any
				EnvelopeSignatures any
			}{
				tb.PayloadCanonicalForm(),
				[]any{
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
				[]any{
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
				// non-plain authentication scheme
				payloadExtensionData: slices.Concat([]byte{0x5}, randomExtensionData[:]),
				expectedTransactionCanonicalForm: func(tb flow.TransactionBody) []byte {
					return fingerprint.Fingerprint(struct {
						Payload            any
						PayloadSignatures  any
						EnvelopeSignatures any
					}{
						tb.PayloadCanonicalForm(),
						[]any{
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
						[]any{
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
								ExtensionData: slices.Concat([]byte{0x5}, randomExtensionData[:]),
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

	// WebAuthn authentication scheme is disabled; any signature submitted under the
	// WebAuthn scheme (0x01) must be rejected as invalid extension data.
	t.Run("webauthn authentication scheme is disabled", func(t *testing.T) {
		// Any extension data starting with the WebAuthn scheme byte is rejected,
		// regardless of the remaining bytes.
		transactionBody.PayloadSignatures[0].ExtensionData = []byte{byte(flow.WebAuthnScheme)}
		extensionDataValid, message := transactionBody.PayloadSignatures[0].ValidateExtensionDataAndReconstructMessage(transactionBody.EnvelopeMessage())
		require.False(t, extensionDataValid)
		require.Nil(t, message)

		transactionBody.PayloadSignatures[0].ExtensionData = slices.Concat([]byte{byte(flow.WebAuthnScheme)}, unittest.RandomBytes(32))
		extensionDataValid, message = transactionBody.PayloadSignatures[0].ValidateExtensionDataAndReconstructMessage(transactionBody.EnvelopeMessage())
		require.False(t, extensionDataValid)
		require.Nil(t, message)
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
