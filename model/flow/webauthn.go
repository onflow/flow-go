package flow

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/onflow/crypto/hash"
)

// Consolidation of WebAuthn related constants, types and functions
// All WebAuthn related constants, types and functions should be defined here

const webAuthnChallengeLength = 32
const webAuthnExtensionDataMinimumLength = 37
const WebAuthnTypeGet = "webauthn.get"

type WebAuthnExtensionData struct {
	// The WebAuthn extension data
	AuthenticatorData []byte
	ClientDataJson    []byte
}

type CollectedClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

func (w *WebAuthnExtensionData) GetUnmarshalledCollectedClientData() (*CollectedClientData, error) {
	clientData := new(CollectedClientData)
	err := json.Unmarshal(w.ClientDataJson, clientData)
	if err != nil {
		return nil, err
	}
	return clientData, err
}

// Helper functions for WebAuthn related operations

// As per https://github.com/onflow/flips/blob/tarak/webauthn/protocol/20250203-webauthn-credential-support.md#fvm-transaction-validation-changes
// check UP is set, BS is not set if BE is not set, AT is only set if attested data is included, ED is set only if extension data is included. If any of the checks fail, return "invalid".
func validateFlags(flags byte, extensions []byte) error {
	// Parse flags
	if userPresent := (flags & 0x01) != 0; !userPresent { // Bit 0: User Present (UP).
		return errors.New("invalid flags: user presence (UP) not set")
	}

	backupEligibility := (flags & 0x08) != 0 // Bit 3: Backup Eligibility (BE).
	backupState := (flags & 0x10) != 0       // Bit 4: Backup State (BS).

	if backupState && !backupEligibility {
		return errors.New("invalid flags: backup state (BS) set without backup eligibility (BE)")
	}

	attestationCredentialData := (flags & 0x40) != 0 // Bit 6: Attestation Credential Data (AT).
	extensionData := (flags & 0x80) != 0             // Bit 7: Extension Data (ED).

	// For now, just check if there is a mismatch in expected state,
	// i.e. no extension data but flags are set.
	if (len(extensions) > 0) != (attestationCredentialData || extensionData) {
		return errors.New("invalid flags: Attestation Credential Data (AT) or Extension Data (ED) flag are not matching the corresponding extension data")
	}

	// If all checks pass, return nil
	return nil
}

func validateWebAuthNExtensionData(extensionData []byte, payload []byte) (bool, []byte) {
	// See FLIP 264 for more details
	if len(extensionData) == 0 {
		return false, nil
	}
	rlpEncodedWebAuthnData := extensionData[1:]
	decodedWebAuthnData := &WebAuthnExtensionData{}
	if err := rlp.DecodeBytes(rlpEncodedWebAuthnData, decodedWebAuthnData); err != nil {
		return false, nil
	}

	clientData, err := decodedWebAuthnData.GetUnmarshalledCollectedClientData()
	if err != nil {
		return false, nil
	}

	// base64url decode the challenge, as that's the encoding used client side according to https://www.w3.org/TR/webauthn-3/#dictionary-client-data
	clientDataChallenge, err := base64.RawURLEncoding.DecodeString(clientData.Challenge)
	if err != nil {
		return false, nil
	}

	if strings.Compare(clientData.Type, WebAuthnTypeGet) != 0 || len(clientDataChallenge) != webAuthnChallengeLength {
		// invalid client data
		return false, nil
	}

	// make sure the challenge is the hash of the transaction payload
	hasher := hash.NewSHA2_256()
	_, err = hasher.Write(TransactionDomainTag[:])
	if err != nil {
		return false, nil
	}
	_, err = hasher.Write(payload)
	if err != nil {
		return false, nil
	}
	computedChallenge := hasher.SumHash()
	if !computedChallenge.Equal(clientDataChallenge) {
		return false, nil
	}

	// Validate authenticatorData
	if len(decodedWebAuthnData.AuthenticatorData) < webAuthnExtensionDataMinimumLength {
		return false, nil
	}

	// extract rpIdHash, userFlags, sigCounter, extensions
	rpIdHash := decodedWebAuthnData.AuthenticatorData[:webAuthnChallengeLength]
	userFlags := decodedWebAuthnData.AuthenticatorData[webAuthnChallengeLength]
	extensions := decodedWebAuthnData.AuthenticatorData[webAuthnExtensionDataMinimumLength:]
	if bytes.Equal(TransactionDomainTag[:], rpIdHash) {
		return false, nil
	}

	// validate user flags according to FLIP 264
	if err := validateFlags(userFlags, extensions); err != nil {
		return false, nil
	}

	clientDataHash := hash.NewSHA2_256().ComputeHash(decodedWebAuthnData.ClientDataJson)

	return true, slices.Concat(decodedWebAuthnData.AuthenticatorData, clientDataHash)
}
