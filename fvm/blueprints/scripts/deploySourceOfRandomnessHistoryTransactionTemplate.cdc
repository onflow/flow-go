transaction(code: String) {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "SourceOfRandomnessHistory", code: code.decodeHex())
  }
}
