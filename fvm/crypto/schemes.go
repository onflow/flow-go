package crypto

type SignatureScheme byte

const (
	PLAIN    SignatureScheme = iota // 0x0
	WEBAUTHN                        // 0x01
)
