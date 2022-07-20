transaction(code: String, epochTokenPayout: UFix64, rewardCut: UFix64) {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "FlowIDTableStaking", code: code.decodeHex(), epochTokenPayout: epochTokenPayout, rewardCut: rewardCut)
  }
}
