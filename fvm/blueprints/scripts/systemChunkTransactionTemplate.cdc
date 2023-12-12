import FlowEpoch from 0xEPOCHADDRESS
import NodeVersionBeacon from 0xNODEVERSIONBEACONADDRESS
import RandomBeaconHistory from 0xRANDOMBEACONHISTORYADDRESS

transaction {
    prepare(serviceAccount: auth(BorrowValue) &Account) {
        let epochHeartbeat = serviceAccount.storage.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
            ?? panic("Could not borrow heartbeat from storage path")
        epochHeartbeat.advanceBlock()

        let versionBeaconHeartbeat = serviceAccount.storage
            .borrow<&NodeVersionBeacon.Heartbeat>(from: NodeVersionBeacon.HeartbeatStoragePath)
            ?? panic("Couldn't borrow NodeVersionBeacon.Heartbeat Resource")
        versionBeaconHeartbeat.heartbeat()

        let randomBeaconHistoryHeartbeat = serviceAccount.storage
            .borrow<&RandomBeaconHistory.Heartbeat>(from: RandomBeaconHistory.HeartbeatStoragePath)
            ?? panic("Couldn't borrow RandomBeaconHistory.Heartbeat Resource")
        randomBeaconHistoryHeartbeat.heartbeat(randomSourceHistory: randomSourceHistory())
    }
}
