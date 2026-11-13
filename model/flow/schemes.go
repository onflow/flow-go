package flow

type AuthenticationScheme byte

const (
	PlainScheme    AuthenticationScheme = iota // 0x0
	WebAuthnScheme                             // 0x01
	InvalidScheme                              // 0x02
)

// webAuthnEnabled gates acceptance of the [WebAuthnScheme] authentication scheme on
// transaction signatures. WebAuthn support is currently disabled; signatures submitted
// under the WebAuthn scheme are rejected at the scheme-dispatch layer in
// [TransactionSignature.ValidateExtensionDataAndReconstructMessage].
const webAuthnEnabled = false

func AuthenticationSchemeFromByte(b byte) AuthenticationScheme {
	if b < byte(InvalidScheme) {
		return AuthenticationScheme(b)
	}
	return InvalidScheme
}

func (s AuthenticationScheme) String() string {
	switch s {
	case PlainScheme:
		return "PlainScheme"
	case WebAuthnScheme:
		return "WebAuthnScheme"
	default:
		return "InvalidScheme"
	}
}
