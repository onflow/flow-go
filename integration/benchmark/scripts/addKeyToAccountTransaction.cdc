transaction(keys: [[UInt8]]) {
  prepare(signer: auth(AddContract) &Account) {
    for key in keys {
      let publicKey = PublicKey(
        publicKey: key,
        signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
      )
      signer.keys.add(
        publicKey: publicKey,
        hashAlgorithm: HashAlgorithm.SHA2_256,
        weight: 1000.0
      )
    }
  }
}
