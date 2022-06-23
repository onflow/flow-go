transaction {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "FlowIDTableStaking", code: "%s".decodeHex(), epochTokenPayout: UFix64(%d), rewardCut: UFix64(%d))
  }
}
