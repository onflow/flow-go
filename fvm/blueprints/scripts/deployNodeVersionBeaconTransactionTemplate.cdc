transaction(code: String, versionThreshold: UInt64) {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "NodeVersionBeacon", code: code.decodeHex(), versionUpdateBuffer: versionThreshold)
  }
}