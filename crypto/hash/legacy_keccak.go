package hash

const (
	rateKECCAK_256 = 136

	dsByteKECCAK = byte(0x1)
)

// NewKECCAK_256 returns a new instance of legacy KECCAK-256 hasher.
func NewKECCAK_256() Hasher {
	return &spongeState{
		algo:      KECCAK_256,
		rate:      rateKECCAK_256,
		dsByte:    dsByteKECCAK,
		outputLen: HashLenKECCAK_256,
		bufIndex:  bufNilValue,
		bufSize:   bufNilValue,
	}
}
