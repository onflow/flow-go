import FlowEpoch from 0x%s

transaction {
  prepare(serviceAccount: AuthAccount) {
	let heartbeat = serviceAccount.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
      ?? panic("Could not borrow heartbeat from storage path")
    heartbeat.advanceBlock()
  }
}
