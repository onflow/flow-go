package random

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/chacha20"
)

// We use Chacha20, to build a cryptographically secure random number generator
// that uses the ChaCha algorithm.
//
// ChaCha is a stream cipher designed by Daniel J. Bernstein[^1], that we use as a PRG. It is
// an improved variant of the Salsa20 cipher family.
//
// We use Chacha20 with a 256-bit key, a 192-bit stream identifier and a 32-bit counter as
// as specified in RFC 8439 [^2].
// The encryption key is used as the PRG seed while the stream identifier is used as a nonce
// to customize the PRG. The PRG outputs are the successive encryptions of a constant message.
//
// A 32-bit counter over 64-byte blocks allows 256 GiB of output before cycling,
// and the stream identifier allows 2^192 unique streams of output per seed.
// It is the caller's responsibility to avoid the PRG output cycling.
//
// [^1]: D. J. Bernstein, [*ChaCha, a variant of Salsa20*](
//       https://cr.yp.to/chacha.html)
//
// [^2]: [RFC 8439: ChaCha20 and Poly1305 for IETF Protocols](
//       https://datatracker.ietf.org/doc/html/rfc8439)

// The PRG core, implements the randCore interface
type chachaCore struct {
	cipher chacha20.Cipher

	// empty message added to minimize allocations and buffer clearing
	emptyMessage [lenEmptyMessage]byte

	// Only used for State/Restore functionality

	// Counter of bytes encrypted so far by the sream cipher.
	// Note this is different than the internal 32-bits counter of the chacha state
	// that counts the encrypted blocks of 512 bits.
	bytesCounter uint64
	// initial seed
	seed [keySize]byte
	// initial customizer
	customizer [nonceSize]byte
}

// The main PRG, implements the Rand interface
type chachaPRG struct {
	genericPRG
	core *chachaCore
}

const (
	keySize   = chacha20.KeySize
	nonceSize = chacha20.NonceSize

	// Chacha20SeedLen is the seed length of the Chacha based PRG, it is fixed to 32 bytes.
	Chacha20SeedLen = keySize
	// Chacha20CustomizerMaxLen is the maximum length of the nonce used as a PRG customizer, it is fixed to 24 bytes.
	// Shorter customizers are padded by zeros to 24 bytes.
	Chacha20CustomizerMaxLen = nonceSize
)

// NewChacha20PRG returns a new Chacha20-based PRG, seeded with
// the input seed (32 bytes) and a customizer (up to 12 bytes).
//
// It is recommended to sample the seed uniformly at random.
// The function errors if the seed is different than 32 bytes,
// or if the customizer is larger than 12 bytes.
// Shorter customizers than 12 bytes are padded by zero bytes.
func NewChacha20PRG(seed []byte, customizer []byte) (*chachaPRG, error) {

	// check the key size
	if len(seed) != Chacha20SeedLen {
		return nil, fmt.Errorf("chacha20 seed length should be %d, got %d", Chacha20SeedLen, len(seed))
	}

	// check the nonce size
	if len(customizer) > Chacha20CustomizerMaxLen {
		return nil, fmt.Errorf("chacha20 streamID should be less than %d bytes", Chacha20CustomizerMaxLen)
	}

	// init the state core
	var core chachaCore
	// core.bytesCounter is set to 0
	copy(core.seed[:], seed)
	copy(core.customizer[:], customizer) // pad the customizer with zero bytes when it's short

	// create the Chacha20 state, initialized with the seed as a key, and the customizer as a streamID.
	chacha, err := chacha20.NewUnauthenticatedCipher(core.seed[:], core.customizer[:])
	if err != nil {
		return nil, fmt.Errorf("chacha20 instance creation failed: %w", err)
	}
	core.cipher = *chacha

	prg := &chachaPRG{
		genericPRG: genericPRG{
			randCore: &core,
		},
		core: &core,
	}
	return prg, nil
}

const lenEmptyMessage = 64

// Read pulls random bytes from the pseudo-random source.
// The randoms are copied into the input buffer, the number of bytes read
// is equal to the buffer input length.
//
// The stream cipher encrypts a stream of a constant message (empty for simplicity).
func (c *chachaCore) Read(buffer []byte) {
	// message to encrypt
	var message []byte

	if len(buffer) <= lenEmptyMessage {
		// use a constant message (used for most of the calls)
		message = c.emptyMessage[:len(buffer)]
	} else {
		// when buffer is large, use is as the message to encrypt,
		// but this requires clearing it first.
		for i := 0; i < len(buffer); i++ {
			buffer[i] = 0
		}
		message = buffer
	}
	c.cipher.XORKeyStream(buffer, message)
	// increase the counter
	c.bytesCounter += uint64(len(buffer))
}

// counter is stored over 8 bytes
const counterBytesLen = 8

// Store returns the internal state of the concatenated Chacha20s
// This is used for serialization/deserialization purposes.
func (c *chachaPRG) Store() []byte {
	bytes := make([]byte, 0, keySize+nonceSize+counterBytesLen)
	counter := make([]byte, counterBytesLen)
	binary.LittleEndian.PutUint64(counter, c.core.bytesCounter)
	// output is seed || streamID || counter
	bytes = append(bytes, c.core.seed[:]...)
	bytes = append(bytes, c.core.customizer[:]...)
	bytes = append(bytes, counter...)
	return bytes
}

// RestoreChacha20PRG creates a chacha20 base PRG based on a previously stored state.
// The created PRG is restored at the same state where the previous PRG was stored.
func RestoreChacha20PRG(stateBytes []byte) (*chachaPRG, error) {
	// input should be seed (32 bytes) || streamID (12 bytes) || bytesCounter (8 bytes)
	const expectedLen = keySize + nonceSize + counterBytesLen

	// check input length
	if len(stateBytes) != expectedLen {
		return nil, fmt.Errorf("Rand state length should be of %d bytes, got %d", expectedLen, len(stateBytes))
	}

	seed := stateBytes[:keySize]
	streamID := stateBytes[keySize : keySize+nonceSize]
	bytesCounter := binary.LittleEndian.Uint64(stateBytes[keySize+nonceSize:])

	// create the Chacha20 instance with seed and streamID
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, streamID)
	if err != nil {
		return nil, fmt.Errorf("Chacha20 instance creation failed: %w", err)
	}
	// set the block counter, each chacha internal block is 512 bits
	const bytesPerBlock = 512 >> 3
	blockCount := uint32(bytesCounter / bytesPerBlock)
	remainingBytes := bytesCounter % bytesPerBlock
	chacha.SetCounter(blockCount)
	// query the remaining bytes and to catch the stored chacha state
	remainderStream := make([]byte, remainingBytes)
	chacha.XORKeyStream(remainderStream, remainderStream)

	core := &chachaCore{
		cipher:       *chacha,
		bytesCounter: bytesCounter,
	}
	copy(core.seed[:], seed)
	copy(core.customizer[:], streamID)

	prg := &chachaPRG{
		genericPRG: genericPRG{
			randCore: core,
		},
		core: core,
	}
	return prg, nil
}
