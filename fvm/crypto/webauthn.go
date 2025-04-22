package crypto

import (
	"encoding/json"
	"errors"
)

// Consolidation of WebAuthn related constants, types and functions
// All WebAuthn related constants, types and functions should be defined here
// and used in crypto.go

const WebAuthnChallengeLength = 32
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

func (w *WebAuthnExtensionData) GetCollectedClientData() (*CollectedClientData, error) {
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
	if userPresent := (flags & 0x01) != 0; !userPresent {
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
	if len(extensions) == 0 && (attestationCredentialData || extensionData) {
		return errors.New("invalid flags: Attestation Credential Data (AT) Extension Data (ED) flag set without corrosponding extension data")
	} else if len(extensions) != 0 && !(attestationCredentialData && extensionData) {
		return errors.New("invalid flags: Extension Data (ED) flag set without corrosponding extension data")
	}

	// If all checks pass, return nil
	return nil
}
