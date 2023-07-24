import FlowEpoch from 0xEPOCHADDRESS
import NodeVersionBeacon from 0xNODEVERSIONBEACONADDRESS
import SourceOfRandomness from 0xSORADDRESS

transaction {
    prepare(serviceAccount: AuthAccount) {
        let epochHeartbeat = serviceAccount.borrow<&FlowEpoch.Heartbeat>(from: FlowEpoch.heartbeatStoragePath)
            ?? panic("Could not borrow heartbeat from storage path")
        epochHeartbeat.advanceBlock()

        let versionBeaconHeartbeat = serviceAccount.borrow<&NodeVersionBeacon.Heartbeat>(
            from: NodeVersionBeacon.HeartbeatStoragePath)
                ?? panic("Couldn't borrow NodeVersionBeacon.Heartbeat Resource")
        versionBeaconHeartbeat.heartbeat()

        let sourceOfRandomnessHeartbeat = serviceAccount.borrow<&SourceOfRandomness.Heartbeat>(
            from: SourceOfRandomness.HeartbeatStoragePath)
                ?? panic("Couldn't borrow SourceOfRandomness.Heartbeat Resource")
        sourceOfRandomnessHeartbeat.heartbeat()
    }
}
