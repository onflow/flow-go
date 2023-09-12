import FlowEpoch from 0xEPOCHADDRESS
import NodeVersionBeacon from 0xNODEVERSIONBEACONADDRESS

transaction {
    prepare(serviceAccount: auth(BorrowValue) &Account) {
        let epochHeartbeat = serviceAccount.storage.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
            ?? panic("Could not borrow heartbeat from storage path")
        epochHeartbeat.advanceBlock()

        let versionBeaconHeartbeat = serviceAccount.borrow<&NodeVersionBeacon.Heartbeat>(
            from: NodeVersionBeacon.HeartbeatStoragePath)
                ?? panic("Couldn't borrow NodeVersionBeacon.Heartbeat Resource")
        versionBeaconHeartbeat.heartbeat()
    }
}
