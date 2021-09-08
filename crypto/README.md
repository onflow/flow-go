# Flow Cryptography

This Go package provides the cryptography tools needed by the Flow blockchain.
Most of the primitives and protocols can be used in other projects and are not specific to Flow.

Flow is an ongoing project, which means that new features will still be added and modifications will still be made to improve security and performance of the cryptography package.

Notes:
   - The package has not been fully audited for security yet.
   - The package does not provide any security against side channel or fault attacks.

## Package import

Cloning Flow repository and following the [installation steps](https://github.com/onflow/flow-go) builds the necessary tools to use Flow cryptography.

If you wish to only import the Flow cryptography package to your Go project, please follow the following steps:

- Get Flow cryptography package 
```
go get github.com/onflow/flow-go/crypto
```
or simply import the package to your Go project
 ```
import "github.com/onflow/flow-go/crypto"
```

This is enough to run the package code for many functionalities. However, this isn't enough if BLS signature related functionalities are used. The BLS features rely on an extrnal C library ([Relic](https://github.com/relic-toolkit/relic)) for lower level mathematical operations. Building your project at this stage including BLS functionalities would result in build errors related to missing "relic" files. For instance:
```
fatal error: 'relic.h' file not found
#include "relic.h"
         ^~~~~~~~~
```

 An extra step is required to compile the external dependency (Relic) locally. 

- Install [CMake](https://cmake.org/install/), which is used for building the package. The build also requires [Git](http://git-scm.com/) and bash scripting.  
- From the Go package directory in `$GOPATH/pkg/mod/github.com/onflow/flow-go/crypto@<version-tag>/`, build the package dependencies. `version-tag` is the imported package version. 
For instance:
```
cd $GOPATH/pkg/mod/github.com/onflow/flow-go/crypto@v0.18.0/
go generate
```

When building your project and including any BLS functionality, adding a build tag to include the BLS files in the build is necessary. 
```
go test -tags=relic
```


## Algorithms

### Hashing and Message Authentication Code:

`crypto/hash` provides the hashing and MAC algorithms required for Flow. All algorithm implement the generic interface `Hasher`. All digests are of the generic type `Hash`.

 * Sha3: 256 and 384 output sizes
 * Sha2: 256 and 384 output sizes
 * KMAC: 128 variant

### Signature schemes

All signature schemes use the generic interfaces of `PrivateKey` and `PublicKey`. All signatures are of the generic type `Signature`.

 * ECDSA
    * public keys are uncompressed.
    * ephemeral key is derived from the private key, hash and an external entropy using a CSPRNG (based on https://golang.org/pkg/crypto/ecdsa/).
    * supports NIST P-256 (secp256r1) and secp256k1 curves.

 * BLS
    * supports [BLS 12-381](https://electriccoin.co/blog/new-snark-curve/) curve.
    * is optimized for shorter signatures (on G1)
    * public keys are longer (on G2)
    * supports [compressed](https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-) and uncompressed serialization of G1/G2 points.
    * hash to curve is using the [optimized SWU map](https://eprint.iacr.org/2019/403.pdf).
    * expanding the message is using KMAC 128 with a domain separation tag.
    * signature verification includes the signature membership check in G1.
    * public key membership check in G2 is provided outside of the signature verification.
    * membership check in G1 is using [Bowe's fast check](https://eprint.iacr.org/2019/814.pdf), while membership check in G2 is using a simple scalar multiplication by the group order.
    * non-interactive aggregation of signatures, public keys and private keys.
    * multi-signature verification of an aggregated signature of a single message under multiple public keys.
    * multi-signature verification of an aggregated signature of multiple messages under multiple public keys.
    * batch verification of multiple signatures of a single message under multiple
    public keys: use a binary tree of aggregations to find the invalid signatures.
    * SPoCK scheme based on BLS: verifies two signatures have been generated from the same message that is unknown to the verifier.

 * Future features:
    * membership checks in G2 using [Bowe's method](https://eprint.iacr.org/2019/814.pdf).
    * support a G1/G2 swap (signatures on G2 and public keys on G1).

### PRNG

 * Xorshift128+

## Protocols

### Threshold Signature

 * BLS-based threshold signature
    * [non interactive](https://www.iacr.org/archive/pkc2003/25670031/25670031.pdf) threshold signature reconstruction.
    * supports only BLS 12-381 curve with the same features above.
    * (t+1) signatures are required to reconstruct the threshold signature.
    * key generation (single dealer) to provide the set of keys.
    * provides a stateless api and a stateful api.

 * Future features:
    * support a partial signature reconstruction in the stateful api to avoid a long final reconstruction.


### Discrete-Log based distributed key generation

All supported Distributed Key Generation protocols are [discrete log based](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.50.2737&rep=rep1&type=pdf) and are implemented for the same BLS setup on the BLS 12-381 curve. The protocols generate key sets for the BLS-based threshold signature.

 * Feldman VSS
    * simple verifiable secret sharing with a single dealer.
    * the library does not implement the communication channels between participants. The caller should implement the methods `PrivateSend` (1-to-1 messaging) and `Broadcast` (1-to-n messaging)
    * 1-to-1 messaging must be a private channel, the caller must make sure the channel preserves confidentialiy and authenticates the sender.
    * 1-to-n broadcasting assume all destination nodes receive the same copy of the message. The channel should also authenticate the broadcaster.
    * It is recommended that both communication channels are unique per protocol instance. This could be achieved by prepending the messages to send/broadcast by a unique protocol instance ID.
 * Feldman VSS Qual.
    * an extension of the simple Feldman VSS.
    * implements a complaint mechanism to qualify/disqualify the dealer.
 * Joint Feldman (Pedersen)
    * distributed generation.
    * based on multiple parallel instances of Feldman VSS Qual with multiple dealers.
    * same assumptions about the communication channels as in Feldman VSS.





