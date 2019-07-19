

# crypto
`import "github.com/dapperlabs/bamboo-node/pkg/crypto"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Subdirectories](#pkg-subdirectories)

## <a name="pkg-overview">Overview</a>



## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [type AlgoName](#AlgoName)
* [type BLS_BLS12381Algo](#BLS_BLS12381Algo)
  * [func (a *BLS_BLS12381Algo) GeneratePrKey() PrKey](#BLS_BLS12381Algo.GeneratePrKey)
  * [func (a *BLS_BLS12381Algo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature](#BLS_BLS12381Algo.SignBytes)
  * [func (a *BLS_BLS12381Algo) SignHash(k PrKey, h Hash) Signature](#BLS_BLS12381Algo.SignHash)
  * [func (a *BLS_BLS12381Algo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature](#BLS_BLS12381Algo.SignStruct)
  * [func (a *BLS_BLS12381Algo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) bool](#BLS_BLS12381Algo.VerifyBytes)
  * [func (a *BLS_BLS12381Algo) VerifyHash(pk PubKey, s Signature, h Hash) bool](#BLS_BLS12381Algo.VerifyHash)
  * [func (a *BLS_BLS12381Algo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) bool](#BLS_BLS12381Algo.VerifyStruct)
* [type Encoder](#Encoder)
* [type Hash](#Hash)
* [type Hash32](#Hash32)
  * [func (h *Hash32) IsEqual(input Hash) bool](#Hash32.IsEqual)
  * [func (h *Hash32) String() string](#Hash32.String)
  * [func (h *Hash32) ToBytes() []byte](#Hash32.ToBytes)
* [type Hash64](#Hash64)
* [type HashAlgo](#HashAlgo)
  * [func (a *HashAlgo) AddBytes(data []byte)](#HashAlgo.AddBytes)
  * [func (a *HashAlgo) AddStruct(struc Encoder)](#HashAlgo.AddStruct)
  * [func (a *HashAlgo) Name() AlgoName](#HashAlgo.Name)
* [type Hasher](#Hasher)
  * [func NewHashAlgo(name AlgoName) Hasher](#NewHashAlgo)
* [type PrKey](#PrKey)
* [type PrKeyBLS_BLS12381](#PrKeyBLS_BLS12381)
  * [func (sk *PrKeyBLS_BLS12381) AlgoName() AlgoName](#PrKeyBLS_BLS12381.AlgoName)
  * [func (sk *PrKeyBLS_BLS12381) ComputePubKey()](#PrKeyBLS_BLS12381.ComputePubKey)
  * [func (sk *PrKeyBLS_BLS12381) KeySize() int](#PrKeyBLS_BLS12381.KeySize)
  * [func (sk *PrKeyBLS_BLS12381) Pubkey() PubKey](#PrKeyBLS_BLS12381.Pubkey)
* [type PubKey](#PubKey)
* [type PubKey_BLS_BLS12381](#PubKey_BLS_BLS12381)
  * [func (pk *PubKey_BLS_BLS12381) KeySize() int](#PubKey_BLS_BLS12381.KeySize)
* [type SignAlgo](#SignAlgo)
  * [func (a *SignAlgo) Name() AlgoName](#SignAlgo.Name)
  * [func (a *SignAlgo) SignatureSize() int](#SignAlgo.SignatureSize)
* [type Signature](#Signature)
* [type Signature48](#Signature48)
  * [func (s *Signature48) String() string](#Signature48.String)
  * [func (s *Signature48) ToBytes() []byte](#Signature48.ToBytes)
* [type Signer](#Signer)
  * [func NewSignatureAlgo(name AlgoName) Signer](#NewSignatureAlgo)


#### <a name="pkg-files">Package files</a>
[bls.go](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go) [encoder.go](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/encoder.go) [hash.go](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go) [sha3.go](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sha3.go) [sign.go](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go) [types.go](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/types.go)


## <a name="pkg-constants">Constants</a>
``` go
const (
    // Lengths of hash outputs in bytes
    HashLengthSha2_256 = 32
    HashLengthSha3_256 = 32
    HashLengthSha3_512 = 64

    // BLS12-381
    SignatureLengthBLS_BLS12381 = 48
    PrKeyLengthBLS_BLS12381     = 32
    PubKeyLengthBLS_BLS12381    = 96
)
```




## <a name="AlgoName">type</a> [AlgoName](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/types.go?s=56:76#L4)
``` go
type AlgoName string
```
AlgoName is the supported algos type


``` go
const (
    // Hashing supported algorithms
    SHA3_256 AlgoName = "SHA3_256"

    // Signing supported algorithms
    BLS_BLS12381 = "BLS_BLS12381"
)
```









## <a name="BLS_BLS12381Algo">type</a> [BLS_BLS12381Algo](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=368:465#L24)
``` go
type BLS_BLS12381Algo struct {
    // G1 and G2 curves will be added here
    //...
    //...
    *SignAlgo
}

```
BLS_BLS12381Algo, embeds SignAlgo










### <a name="BLS_BLS12381Algo.GeneratePrKey">func</a> (\*BLS\_BLS12381Algo) [GeneratePrKey](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=847:895#L43)
``` go
func (a *BLS_BLS12381Algo) GeneratePrKey() PrKey
```
GeneratePrKey generates a private key for BLS on BLS12381 curve




### <a name="BLS_BLS12381Algo.SignBytes">func</a> (\*BLS\_BLS12381Algo) [SignBytes](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=1026:1107#L51)
``` go
func (a *BLS_BLS12381Algo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature
```
SignBytes signs an array of bytes




### <a name="BLS_BLS12381Algo.SignHash">func</a> (\*BLS\_BLS12381Algo) [SignHash](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=522:584#L32)
``` go
func (a *BLS_BLS12381Algo) SignHash(k PrKey, h Hash) Signature
```
SignHash implements BLS signature on BLS12381 curve




### <a name="BLS_BLS12381Algo.SignStruct">func</a> (\*BLS\_BLS12381Algo) [SignStruct](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=1204:1287#L57)
``` go
func (a *BLS_BLS12381Algo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature
```
SignStruct signs a structure




### <a name="BLS_BLS12381Algo.VerifyBytes">func</a> (\*BLS\_BLS12381Algo) [VerifyBytes](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=1405:1497#L63)
``` go
func (a *BLS_BLS12381Algo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) bool
```
VerifyBytes verifies a signature of a byte array




### <a name="BLS_BLS12381Algo.VerifyHash">func</a> (\*BLS\_BLS12381Algo) [VerifyHash](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=687:761#L38)
``` go
func (a *BLS_BLS12381Algo) VerifyHash(pk PubKey, s Signature, h Hash) bool
```
VerifyHash implements BLS signature verification on BLS12381 curve




### <a name="BLS_BLS12381Algo.VerifyStruct">func</a> (\*BLS\_BLS12381Algo) [VerifyStruct](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=1619:1713#L69)
``` go
func (a *BLS_BLS12381Algo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) bool
```
VerifyStruct verifies a signature of a structure




## <a name="Encoder">type</a> [Encoder](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/encoder.go?s=127:170#L6)
``` go
type Encoder interface {
    Encode() []byte
}
```
Encoder is an interface of a generic structure










## <a name="Hash">type</a> [Hash](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go?s=588:848#L27)
``` go
type Hash interface {
    // ToBytes returns the bytes representation of a hash
    ToBytes() []byte
    // String returns a Hex string representation of the hash bytes in big endian
    String() string
    // IsEqual tests an equality with a given hash
    IsEqual(Hash) bool
}
```









## <a name="Hash32">type</a> [Hash32](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/types.go?s=560:580#L31)
``` go
type Hash32 [32]byte
```
Hash32 is 256-bits digest










### <a name="Hash32.IsEqual">func</a> (\*Hash32) [IsEqual](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sha3.go?s=1026:1067#L53)
``` go
func (h *Hash32) IsEqual(input Hash) bool
```



### <a name="Hash32.String">func</a> (\*Hash32) [String](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sha3.go?s=844:876#L44)
``` go
func (h *Hash32) String() string
```



### <a name="Hash32.ToBytes">func</a> (\*Hash32) [ToBytes](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sha3.go?s=792:825#L40)
``` go
func (h *Hash32) ToBytes() []byte
```
Hash32 implements Hash




## <a name="Hash64">type</a> [Hash64](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/types.go?s=611:631#L34)
``` go
type Hash64 [64]byte
```
Hash64 is 512-bits digest










## <a name="HashAlgo">type</a> [HashAlgo](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go?s=1312:1388#L55)
``` go
type HashAlgo struct {
    hash.Hash
    // contains filtered or unexported fields
}

```
HashAlgo










### <a name="HashAlgo.AddBytes">func</a> (\*HashAlgo) [AddBytes](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go?s=1536:1576#L67)
``` go
func (a *HashAlgo) AddBytes(data []byte)
```
AddBytes adds bytes to the current hash state




### <a name="HashAlgo.AddStruct">func</a> (\*HashAlgo) [AddStruct](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go?s=1653:1696#L72)
``` go
func (a *HashAlgo) AddStruct(struc Encoder)
```
AddStruct adds a structure to the current hash state




### <a name="HashAlgo.Name">func</a> (\*HashAlgo) [Name](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go?s=1432:1466#L62)
``` go
func (a *HashAlgo) Name() AlgoName
```
Name returns the name of the algorithm




## <a name="Hasher">type</a> [Hasher](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go?s=871:1298#L38)
``` go
type Hasher interface {
    Name() AlgoName
    // Size return the hash output length (a Hash.hash method)
    Size() int
    // Compute hash
    ComputeBytesHash([]byte) Hash
    ComputeStructHash(Encoder) Hash
    // Write adds more bytes to the current hash state (a Hash.hash method)
    AddBytes([]byte)
    AddStruct(Encoder)
    // SumHash returns the hash output and resets the hash state a
    SumHash() Hash
    // Reset resets the hash state
    Reset()
}
```






### <a name="NewHashAlgo">func</a> [NewHashAlgo](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/hash.go?s=158:196#L11)
``` go
func NewHashAlgo(name AlgoName) Hasher
```
NewHashAlgo initializes and chooses a hashing algorithm





## <a name="PrKey">type</a> [PrKey](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=1680:1952#L69)
``` go
type PrKey interface {
    // returns the name of the algorithm related to the private key
    AlgoName() AlgoName
    // return the size in bytes
    KeySize() int
    // computes the pub key associated with the private key
    ComputePubKey()
    // returns the public key
    Pubkey() PubKey
}
```
PrKey is an unspecified signature scheme private key










## <a name="PrKeyBLS_BLS12381">type</a> [PrKeyBLS_BLS12381](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=1868:2036#L75)
``` go
type PrKeyBLS_BLS12381 struct {
    // contains filtered or unexported fields
}

```
PrKeyBLS_BLS12381 is the private key of BLS using BLS12_381, it implements PrKey










### <a name="PrKeyBLS_BLS12381.AlgoName">func</a> (\*PrKeyBLS\_BLS12381) [AlgoName](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=2038:2086#L84)
``` go
func (sk *PrKeyBLS_BLS12381) AlgoName() AlgoName
```



### <a name="PrKeyBLS_BLS12381.ComputePubKey">func</a> (\*PrKeyBLS\_BLS12381) [ComputePubKey](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=2194:2238#L92)
``` go
func (sk *PrKeyBLS_BLS12381) ComputePubKey()
```



### <a name="PrKeyBLS_BLS12381.KeySize">func</a> (\*PrKeyBLS\_BLS12381) [KeySize](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=2114:2156#L88)
``` go
func (sk *PrKeyBLS_BLS12381) KeySize() int
```



### <a name="PrKeyBLS_BLS12381.Pubkey">func</a> (\*PrKeyBLS\_BLS12381) [Pubkey](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=2280:2324#L97)
``` go
func (sk *PrKeyBLS_BLS12381) Pubkey() PubKey
```



## <a name="PubKey">type</a> [PubKey](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=2010:2079#L81)
``` go
type PubKey interface {
    // return the size in bytes
    KeySize() int
}
```
PubKey is an unspecified signature scheme public key










## <a name="PubKey_BLS_BLS12381">type</a> [PubKey_BLS_BLS12381](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=2487:2575#L106)
``` go
type PubKey_BLS_BLS12381 struct {
    // contains filtered or unexported fields
}

```
PubKey_BLS_BLS12381 is the public key of BLS using BLS12_381, it implements PubKey










### <a name="PubKey_BLS_BLS12381.KeySize">func</a> (\*PubKey\_BLS\_BLS12381) [KeySize](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=2577:2621#L111)
``` go
func (pk *PubKey_BLS_BLS12381) KeySize() int
```



## <a name="SignAlgo">type</a> [SignAlgo](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=1000:1113#L39)
``` go
type SignAlgo struct {
    PrKeyLength     int
    PubKeyLength    int
    SignatureLength int
    // contains filtered or unexported fields
}

```
SignAlgo










### <a name="SignAlgo.Name">func</a> (\*SignAlgo) [Name](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=1157:1191#L47)
``` go
func (a *SignAlgo) Name() AlgoName
```
Name returns the name of the algorithm




### <a name="SignAlgo.SignatureSize">func</a> (\*SignAlgo) [SignatureSize](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=1270:1308#L52)
``` go
func (a *SignAlgo) SignatureSize() int
```
SignatureSize returns the size of a signature in bytes




## <a name="Signature">type</a> [Signature](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=1420:1609#L59)
``` go
type Signature interface {
    // ToBytes returns the bytes representation of a signature
    ToBytes() []byte
    // String returns a hex string representation of signature bytes
    String() string
}
```
Signature is unspecified signature scheme signature










## <a name="Signature48">type</a> [Signature48](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/types.go?s=670:695#L37)
``` go
type Signature48 [48]byte
```
Signature48 is 384-bits signature










### <a name="Signature48.String">func</a> (\*Signature48) [String](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=144:181#L14)
``` go
func (s *Signature48) String() string
```



### <a name="Signature48.ToBytes">func</a> (\*Signature48) [ToBytes](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/bls.go?s=87:125#L10)
``` go
func (s *Signature48) ToBytes() []byte
```



## <a name="Signer">type</a> [Signer](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=447:986#L22)
``` go
type Signer interface {
    Name() AlgoName
    // Size return the signature output length
    SignatureSize() int
    // Signature functions
    SignHash(PrKey, Hash) Signature
    SignBytes(PrKey, []byte, Hasher) Signature
    SignStruct(PrKey, Encoder, Hasher) Signature
    // Verification functions
    VerifyHash(PubKey, Signature, Hash) bool
    VerifyBytes(PubKey, Signature, []byte, Hasher) bool
    VerifyStruct(PubKey, Signature, Encoder, Hasher) bool
    // Private key functions
    GeneratePrKey() PrKey // to be changed to accept a randomness source as an input
}
```
Signer interface







### <a name="NewSignatureAlgo">func</a> [NewSignatureAlgo](https://github.com/dapperlabs/bamboo-node/tree/master/pkg/crypto/sign.go?s=125:168#L8)
``` go
func NewSignatureAlgo(name AlgoName) Signer
```
NewSignatureAlgo initializes and chooses a signature scheme









- - -
Generated by [godoc2md](http://godoc.org/github.com/lanre-ade/godoc2md)
