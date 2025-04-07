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
	AuthenticatorData []byte `rlp:"authenticatorData"`
	ClientDataJson    []byte `rlp:"clientDataJson"`
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
	if clientData.Type != WebAuthnTypeGet || len(clientData.Challenge) != WebAuthnChallengeLength || len(clientData.Origin) == 0 {
		return nil, errors.New("invalid client data")
	}
	return clientData, err
}

// Helper functions for WebAuthn related operations

// As per https://github.com/onflow/flips/blob/tarak/webauthn/protocol/20250203-webauthn-credential-support.md#fvm-transaction-validation-changes
// check UP is set, BS is not set if BE is not set, AT is only set if attested data is included, ED is set only if extension data is included. If any of the checks fail, return "invalid".
func validateFlags(flags byte) error {
	// Parse flags
	if userPresent := (flags & 0x01) != 0; !userPresent {
		return errors.New("invalid flags: user presence (UP) not set")
	}

	backupEligibility := (flags & 0x08) != 0 // Bit 3: Backup Eligibility (BE).
	backupState := (flags & 0x10) != 0       // Bit 4: Backup State (BS).

	if backupState && !backupEligibility {
		return errors.New("invalid flags: backup state (BS) set without backup eligibility (BE)")
	}

	// attestationCredentialData := (flags & 0x40) != 0 // Bit 6: Attestation Credential Data (AT).
	// extensionData := (flags & 0x80) != 0 // Bit 7: Extension Data (ED).

	return nil
}
