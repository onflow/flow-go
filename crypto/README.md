# Flow Cryptography

The Go package provides all the cryptography tools needed by Flow blockchain.  
Most of the primitives and protocols can be used in other projects and are not specific to Flow.

Flow is an ongoing project, new features will still be added and modifications will still be made to improve security and performance of the cryptography package.

NOTE: The package does not provide any security against side channel or tampering attacks.

## Package import

Cloning Flow repository and following the [installation steps](https://github.com/dapperlabs/flow-go) builds the necessary tools to use Flow cryptography. 

<!--- If you only wish to import the Flow cryptography package to your Go project, please follow the following steps:

- Get Flow cryptography package
```
go get "github.com/dapperlabs/flow-go/crypto"
```
- Install [CMake](https://cmake.org/install/), which is used for building the package.
- Build the package dependencies
```
cd $GOPATH/pkg/mod/github.com/flow-go/crypto
go generate
``` -->

## Algorithms

### Hashing and Message Authentication Code:

`crypto/hash` provides the hashing and MAC algorithms required for Flow. All algorithm implement the generic interface `Hasher`. All digests are of the generic type `Hash`.

*Hashing* :
 * Sha3: 256 and 384 output size
 * Sha2: 256 and 384 output size

*MAC* :
 * KMAC: 128 variant

### Signature schemes 

All signature schemes use the generic interfaces of `PrivateKey` and `PublicKey`. All signatures are of the generic type `Signature`.

 * ECDSA
    * public keys are uncompressed.
    * ephemeral key is derived from the private key, hash and an external entropy using a CSPRNG.
    * only supports NIST P-256 (secp256r1) and secp256k1 curves.
    * key generation requires a uniformly-distributed seed. 

 * BLS
    * only supports [BLS 12-381](https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md) curve.
    * is optimized for shorter signatures (on G1) 
    * public keys are longer (on G2)
    * supports [compressed](https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md#serialization) and uncompressed serialization of G1/G2 points.
    * hash to curve is using the [optimized SWU map](https://eprint.iacr.org/2019/403.pdf).
    * expanding the message is using KMAC 128 with a domain separation tag.
    * signature verification includes a membership check in in G1.
    * membership check in G2 is provided outside signature verification.
    * membership checks in G1 and G2 are using a naive scalar multiplication by the group order.

 * Future features:
    * tools for BLS aggregations and batch verification
    * membership checks in G1 and G2 using [Bowe's method](https://eprint.iacr.org/2019/814.pdf)
    * support a G1/G2 swap (signatures on G2 and public keys on G1)
 
### PRNG

 * Xorshift128+

## Protocols

### Threshold Signature

 * BLS-based threshold signature 
    * [non interactive](https://www.iacr.org/archive/pkc2003/25670031/25670031.pdf) threshold signature reconstruction.
    * supports only BLS 12-381 curve with the same features above.
    * the threshold value (t) if fixed to t = floor((n-1)/2) for an optimal unforgeabiliyty and robustness.
    * (t+1) signatures are required to reconstruct the threshold signature.
    * a centralized key generation is provided.
    * provides a stateless api and a stateful api. 

 * Future features:
    * the threshold value (t) is an input parameter.
    * support a partial signature reconstruction in the stateful api to avoid a long final reconstruction. 


### Discrete-Log based distributed key generation

 All supported Distributed Key Generation protocols are [discrete log based](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.50.2737&rep=rep1&type=pdf) and are implemented for the same BLS setup on the BLS 12-381 curve. The protocols generate key sets for the BLS-based threshold signature. 

 * Feldman VSS
    * simple centralized verifiable secret sharing
    * the threshold value (t) if fixed to t = floor((n-1)/2) for an optimal unforgeabiliyty and robustness.
    * 1-to-1 messaging is not encrypted, the caller must make sure the 1-to-1 messaging channel preserves confidentialiy. 
    * 1-to-n broadcasting assume all destination nodes receive the same copy of the message.
 * Feldman VSS Qual
    * based on Feldman VSS.
    * implements a complaint mechanism to qualify/disqualify the leader.
 * Joint Feldman (Pedersen)
    * decentralized generation
    * based on multiple parallel instances of Feldman VSS Qual with multiple leaders.
    * same assumptions about the 1-to-1 and 1-to-n messaging channels as Feldman VSS. 





