package crypto

type AuthenticationScheme byte

const (
	PLAIN    AuthenticationScheme = iota // 0x0
	WEBAUTHN                             // 0x01
)
