transaction(code: String, versionThreshold: UInt64) {
  prepare(serviceAccount: auth(AddContract) &Account) {
	serviceAccount.contracts.add(name: "NodeVersionBeacon", code: code.decodeHex(), versionUpdateBuffer: versionThreshold)
  }
}
