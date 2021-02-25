package utils

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"

	"github.com/onflow/flow-go/ledger"
)

// PathByUint8 returns a path (32 bytes) given a uint8
func PathByUint8(inp uint8) ledger.Path {
	b := make([]byte, 32)
	b[0] = inp
	return ledger.Path(b)
}

// PathByUint16 returns a path (32 bytes) given a uint16
func PathByUint16(inp uint16) ledger.Path {
	b := make([]byte, 32)
	binary.BigEndian.PutUint16(b, inp)
	return ledger.Path(b)
}

// LightPayload returns a payload with 2 byte key and 2 byte value
func LightPayload(key uint16, value uint16) *ledger.Payload {
	k := ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: Uint16ToBinary(key)}}}
	v := ledger.Value(Uint16ToBinary(value))
	return &ledger.Payload{Key: k, Value: v}
}

// LightPayload8 returns a payload with 1 byte key and 1 byte value
func LightPayload8(key uint8, value uint8) *ledger.Payload {
	k := ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: []byte{key}}}}
	v := ledger.Value([]byte{value})
	return &ledger.Payload{Key: k, Value: v}
}

// KeyPartFixture returns a key part fixture
func KeyPartFixture(typ uint16, val string) ledger.KeyPart {
	kp1t := uint16(typ)
	kp1v := []byte(val)
	return ledger.NewKeyPart(kp1t, kp1v)
}

// QueryFixture returns a query fixture
func QueryFixture() *ledger.Query {
	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	k1p1 := ledger.NewKeyPart(uint16(1), []byte("1"))
	k1p2 := ledger.NewKeyPart(uint16(22), []byte("2"))
	k1 := ledger.NewKey([]ledger.KeyPart{k1p1, k1p2})

	k2p1 := ledger.NewKeyPart(uint16(1), []byte("3"))
	k2p2 := ledger.NewKeyPart(uint16(22), []byte("4"))
	k2 := ledger.NewKey([]ledger.KeyPart{k2p1, k2p2})

	u, _ := ledger.NewQuery(sc, []ledger.Key{k1, k2})
	return u
}

// UpdateFixture returns an update fixture
func UpdateFixture() *ledger.Update {
	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	k1p1 := ledger.NewKeyPart(uint16(1), []byte("1"))
	k1p2 := ledger.NewKeyPart(uint16(22), []byte("2"))
	k1 := ledger.NewKey([]ledger.KeyPart{k1p1, k1p2})
	v1 := ledger.Value([]byte{'A'})

	k2p1 := ledger.NewKeyPart(uint16(1), []byte("3"))
	k2p2 := ledger.NewKeyPart(uint16(22), []byte("4"))
	k2 := ledger.NewKey([]ledger.KeyPart{k2p1, k2p2})
	v2 := ledger.Value([]byte{'B'})

	u, _ := ledger.NewUpdate(sc, []ledger.Key{k1, k2}, []ledger.Value{v1, v2})
	return u
}

// RootHashFixture returns a root hash fixture
func RootHashFixture() ledger.RootHash {
	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	return ledger.RootHash(sc)
}

// TrieProofFixture returns a trie proof fixture
func TrieProofFixture() (*ledger.TrieProof, ledger.State) {
	p := ledger.NewTrieProof()
	p.Path = PathByUint16(330)
	p.Payload = LightPayload8('A', 'A')
	p.Inclusion = true
	p.Flags = []byte{byte(130), byte(0)}
	p.Interims = make([][]byte, 0)
	interim1, _ := hex.DecodeString("accb0399dd2b3a7a48618b2376f5e61d822e0c7736b044c364a05c2904a2f315")
	interim2, _ := hex.DecodeString("f3fba426a2f01c342304e3ca7796c3980c62c625f7fd43105ad5afd92b165542")
	p.Interims = append(p.Interims, interim1)
	p.Interims = append(p.Interims, interim2)
	p.Steps = uint8(7)
	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	return p, ledger.State(sc)
}

// TrieBatchProofFixture returns a trie batch proof fixture
func TrieBatchProofFixture() (*ledger.TrieBatchProof, ledger.State) {
	p1 := ledger.NewTrieProof()
	p1.Path = PathByUint16(330)
	p1.Payload = LightPayload8('A', 'A')
	p1.Inclusion = true
	p1.Flags = []byte{byte(130), byte(0)}
	p1.Interims = make([][]byte, 0)
	p1interim1, _ := hex.DecodeString("accb0399dd2b3a7a48618b2376f5e61d822e0c7736b044c364a05c2904a2f315")
	p1interim2, _ := hex.DecodeString("f3fba426a2f01c342304e3ca7796c3980c62c625f7fd43105ad5afd92b165542")
	p1.Interims = append(p1.Interims, p1interim1)
	p1.Interims = append(p1.Interims, p1interim2)
	p1.Steps = uint8(7)

	p2 := ledger.NewTrieProof()
	p2.Path = PathByUint16(33354)
	p2.Payload = LightPayload8('C', 'C')
	p2.Inclusion = true
	p2.Flags = []byte{byte(129), byte(0)}
	p2.Interims = make([][]byte, 0)
	p2interim1, _ := hex.DecodeString("fbb89a7115c48406de7b976049223484c4a9fa1a018f36e13e2b5a6d02d29de2")
	p2interim2, _ := hex.DecodeString("71710ddc0967ef8fdc4e29444ae114165e339423799f9b7a05772cf19b71a9fd")
	p2.Interims = append(p2.Interims, p2interim1)
	p2.Interims = append(p2.Interims, p2interim2)
	p2.Steps = uint8(8)

	bp := ledger.NewTrieBatchProof()
	bp.Proofs = append(bp.Proofs, p1)
	bp.Proofs = append(bp.Proofs, p2)

	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	return bp, ledger.State(sc)
}

// RandomPathsRandLen generate m random paths (size: byteSize),
// the number of paths, m, is also randomly selected from the range [1, maxN]
func RandomPathsRandLen(maxN int, byteSize int) []ledger.Path {
	numberOfPaths := rand.Intn(maxN) + 1
	return RandomPaths(numberOfPaths, byteSize)
}

// RandomPaths generates n random (no repetition) fixed sized (byteSize) paths
func RandomPaths(n int, byteSize int) []ledger.Path {
	paths := make([]ledger.Path, 0, n)
	alreadySelectPaths := make(map[string]bool)
	i := 0
	for i < n {
		path := make([]byte, byteSize)
		rand.Read(path)
		// deduplicate
		if _, found := alreadySelectPaths[string(path)]; !found {
			paths = append(paths, ledger.Path(path))
			alreadySelectPaths[string(path)] = true
			i++
		}
	}
	return paths
}

// RandomPayload returns a random payload
func RandomPayload(minByteSize int, maxByteSize int) *ledger.Payload {
	keyByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	keydata := make([]byte, keyByteSize)
	rand.Read(keydata)
	key := ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0, Value: keydata}}}
	valueByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	valuedata := make([]byte, valueByteSize)
	rand.Read(valuedata)
	value := ledger.Value(valuedata)
	return &ledger.Payload{Key: key, Value: value}
}

// RandomPayloads returns n random payloads
func RandomPayloads(n int, minByteSize int, maxByteSize int) []*ledger.Payload {
	res := make([]*ledger.Payload, 0)
	for i := 0; i < n; i++ {
		res = append(res, RandomPayload(minByteSize, maxByteSize))
	}
	return res
}

// RandomValues returns n random values with variable sizes (minByteSize <= size < maxByteSize)
func RandomValues(n int, minByteSize, maxByteSize int) []ledger.Value {
	if minByteSize > maxByteSize {
		panic("minByteSize cannot be smaller then maxByteSize")
	}
	values := make([]ledger.Value, 0)
	for i := 0; i < n; i++ {
		var byteSize = maxByteSize
		if minByteSize < maxByteSize {
			byteSize = minByteSize + rand.Intn(maxByteSize-minByteSize)
		}
		value := make([]byte, byteSize)
		rand.Read(value)
		values = append(values, value)
	}
	return values
}

// RandomUniqueKeys generates n random keys (each with m random key parts)
func RandomUniqueKeys(n, m, minByteSize, maxByteSize int) []ledger.Key {
	if minByteSize > maxByteSize {
		panic("minByteSize cannot be smaller then maxByteSize")
	}
	keys := make([]ledger.Key, 0)
	alreadySelectKeys := make(map[string]bool)
	i := 0
	for i < n {
		keyParts := make([]ledger.KeyPart, 0)
		for j := 0; j < m; j++ {
			byteSize := maxByteSize
			if minByteSize < maxByteSize {
				byteSize = minByteSize + rand.Intn(maxByteSize-minByteSize)
			}
			keyPartData := make([]byte, byteSize)
			rand.Read(keyPartData)
			keyParts = append(keyParts, ledger.NewKeyPart(uint16(j), keyPartData))
		}
		key := ledger.NewKey(keyParts)

		// deduplicate
		if _, found := alreadySelectKeys[key.String()]; !found {
			keys = append(keys, key)
			alreadySelectKeys[key.String()] = true
			i++
		} else {
			fmt.Println("Already existing")
		}
	}
	return keys
}

// RandomUniqueKeysRandomN generate n (0<n<maxN) random keys (each m random key part),
func RandomUniqueKeysRandomN(maxN, m, minByteSize, maxByteSize int) []ledger.Key {
	numberOfKeys := rand.Intn(maxN) + 1
	// at least return 1 keys
	if numberOfKeys == 0 {
		numberOfKeys = 1
	}
	return RandomUniqueKeys(numberOfKeys, m, minByteSize, maxByteSize)
}
