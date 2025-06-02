package flow

type AuthenticationScheme byte

const (
	PlainScheme    AuthenticationScheme = iota // 0x0
	WebAuthnScheme                             // 0x01
	InvalidScheme                              // 0x02
)

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
