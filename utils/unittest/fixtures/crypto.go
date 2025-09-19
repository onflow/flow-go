package fixtures

import "github.com/onflow/crypto"

var PrivateKey privateKeyFactory

type privateKeyFactory struct{}

type PrivateKeyOption func(*CryptoGenerator, *privateKeyConfig)

type privateKeyConfig struct {
	seed []byte
}

// WithSeed is an option that sets the seed of the private key.
func (f privateKeyFactory) WithSeed(seed []byte) PrivateKeyOption {
	return func(g *CryptoGenerator, config *privateKeyConfig) {
		config.seed = seed
	}
}

type CryptoGenerator struct {
	privateKeyFactory

	random *RandomGenerator
}

func NewCryptoGenerator(random *RandomGenerator) *CryptoGenerator {
	return &CryptoGenerator{
		random: random,
	}
}

// PrivateKey generates a [crypto.PrivateKey].
func (g *CryptoGenerator) PrivateKey(algo crypto.SigningAlgorithm, opts ...PrivateKeyOption) crypto.PrivateKey {
	config := &privateKeyConfig{}

	for _, opt := range opts {
		opt(g, config)
	}

	if len(config.seed) == 0 {
		config.seed = g.random.RandomBytes(crypto.KeyGenSeedMinLen)
	}

	Assertf(len(config.seed) == crypto.KeyGenSeedMinLen, "seed must be %d bytes, got %d", crypto.KeyGenSeedMinLen, len(config.seed))

	pk, err := crypto.GeneratePrivateKey(algo, config.seed)
	NoError(err)
	return pk
}

// StakingPrivateKey generates a staking [crypto.PrivateKey] using the crypto.BLSBLS12381 algorithm.
func (g *CryptoGenerator) StakingPrivateKey(opts ...PrivateKeyOption) crypto.PrivateKey {
	return g.PrivateKey(crypto.BLSBLS12381, opts...)
}

// NetworkingPrivateKey generates a networking [crypto.PrivateKey] using the crypto.ECDSAP256 algorithm.
func (g *CryptoGenerator) NetworkingPrivateKey(opts ...PrivateKeyOption) crypto.PrivateKey {
	return g.PrivateKey(crypto.ECDSAP256, opts...)
}

// PublicKeys generates a list of [crypto.PublicKey].
func (g *CryptoGenerator) PublicKeys(n int, algo crypto.SigningAlgorithm, opts ...PrivateKeyOption) []crypto.PublicKey {
	keys := make([]crypto.PublicKey, n)
	for i := range n {
		keys[i] = g.PrivateKey(algo, opts...).PublicKey()
	}
	return keys
}
