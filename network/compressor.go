package network

// Compressor offers compressing and decompressing services for sending and receiving
// a byte slice at network layer.
type Compressor interface {
	Compress([]byte) ([]byte, error)
	Decompress([]byte) ([]byte, error)
}
