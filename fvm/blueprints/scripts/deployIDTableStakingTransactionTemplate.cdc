// Note: uses a default large candidate limit
transaction(code: String, epochTokenPayout: UFix64, rewardCut: UFix64) {
  prepare(serviceAccount: auth(AddContract) &Account) {
    let candidateNodeLimits: {UInt8: UInt64} = {1: 10000, 2: 10000, 3: 10000, 4: 10000, 5: 10000}
	serviceAccount.contracts.add(name: "FlowIDTableStaking", code: code.decodeHex(), epochTokenPayout: epochTokenPayout, rewardCut: rewardCut, candidateNodeLimits: candidateNodeLimits)
  }
}
