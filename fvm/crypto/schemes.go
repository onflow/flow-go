package crypto

type AuthenticationScheme byte

const (
	PLAIN    AuthenticationScheme = iota // 0x0
	WEBAUTHN                             // 0x01
	INVALID                              // 0x02
)

func AuthenticationSchemeFromByte(b byte) AuthenticationScheme {
	switch b {
	case 0x0:
		return PLAIN
	case 0x01:
		return WEBAUTHN
	default:
		return INVALID
	}
}
