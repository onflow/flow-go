import FlowEpoch from 0xEPOCHADDRESS

transaction {
  prepare(serviceAccount: AuthAccount) {
	let heartbeat = serviceAccount.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
      ?? panic("Could not borrow heartbeat from storage path")
    heartbeat.advanceBlock()
  }
}
