transaction(code: String) {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "RandomBeaconHistory", code: code.utf8)
  }
}
