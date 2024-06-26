transaction(name: String, code: String) {
  prepare(signer: auth(AddContract) &Account) {
    signer.contracts.add(name: name, code: code.utf8)
  }
}
