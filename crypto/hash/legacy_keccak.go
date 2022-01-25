package hash

const (
	rateKeccak_256 = 136

	dsByteKeccak = byte(0x1)
)

// NewKeccak_256 returns a new instance of legacy Keccak-256 hasher.
func NewKeccak_256() Hasher {
	return &spongeState{
		algo:      Keccak_256,
		rate:      rateKeccak_256,
		dsByte:    dsByteKeccak,
		outputLen: HashLenKeccak_256,
		bufIndex:  bufNilValue,
		bufSize:   bufNilValue,
	}
}
