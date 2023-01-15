import FlowEpoch from 0xEPOCHADDRESS
import NodeVersionBeacon from 0xNODEVERSIONBEACONADDRESS

transaction {
  prepare(serviceAccount: AuthAccount) {
    let heartbeat = serviceAccount.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
        ?? panic("Could not borrow heartbeat from storage path")
    heartbeat.advanceBlock()

    let versionBeacon = serviceAccount.borrow<&NodeVersionBeacon.NodeVersionAdmin>(from: NodeVersionBeacon.NodeVersionAdminStoragePath)
        ?? panic("Could not borrow version admin from storage path")
    versionBeacon.checkVersionTableChanges()
  }
}
