import FlowEpoch from "FlowEpoch"
import FlowIDTableStaking from "FlowIDTableStaking"
import FlowClusterQC from "FlowClusterQC"

// The recoverEpoch transaction creates and starts a new epoch in the FlowEpoch smart contract
// to which will force the network exit EFM. The recoverEpoch service event will be emitted
// and processed by all protocol participants and each participant will update their protocol
// state with the new Epoch data.
// This transaction should only be used with the output of the bootstrap utility:
//   util epoch efm-recover-tx-args
// Note: setting unsafeAllowOverwrite to true will force the FlowEpoch contract to overwrite the current
// epoch with the new configuration. If you need to instantiate a brand new epoch with the new configuration
// this should be set to false.
transaction(recoveryEpochCounter: UInt64,
            startView: UInt64,
            stakingEndView: UInt64,
            endView: UInt64,
            targetDuration: UInt64,
            targetEndTime: UInt64,
            clusterAssignments: [[String]],
            clusterQCVoteData: [FlowClusterQC.ClusterQCVoteData],
            dkgPubKeys: [String],
            dkgGroupKey: String,
            dkgIdMapping: {String: Int},
            nodeIDs: [String],
            unsafeAllowOverwrite: Bool) {

    prepare(signer: auth(BorrowValue) &Account) {
        let epochAdmin = signer.storage.borrow<&FlowEpoch.Admin>(from: FlowEpoch.adminStoragePath)
            ?? panic("Could not borrow epoch admin from storage path")

        let proposedEpochCounter = FlowEpoch.proposedEpochCounter()
        if recoveryEpochCounter == proposedEpochCounter {
            // Typical path: RecoveryEpoch uses proposed epoch counter (+1 from current)
            epochAdmin.recoverNewEpoch(
                recoveryEpochCounter: recoveryEpochCounter,
                startView: startView,
                stakingEndView: stakingEndView,
                endView: endView,
                targetDuration: targetDuration,
                targetEndTime: targetEndTime,
                clusterAssignments: clusterAssignments,
                clusterQCVoteData: clusterQCVoteData,
                dkgPubKeys: dkgPubKeys,
                dkgGroupKey: dkgGroupKey,
                dkgIdMapping: dkgIdMapping,
                nodeIDs: nodeIDs
            )
        } else {
            // Atypical path: RecoveryEpoch is overwriting existing epoch.
            if !unsafeAllowOverwrite {
                panic("cannot overwrite existing epoch with safety flag specified")
            }
            epochAdmin.recoverCurrentEpoch(
                recoveryEpochCounter: recoveryEpochCounter,
                startView: startView,
                stakingEndView: stakingEndView,
                endView: endView,
                targetDuration: targetDuration,
                targetEndTime: targetEndTime,
                clusterAssignments: clusterAssignments,
                clusterQCVoteData: clusterQCVoteData,
                dkgPubKeys: dkgPubKeys,
                dkgGroupKey: dkgGroupKey,
                dkgIdMapping: dkgIdMapping,
                nodeIDs: nodeIDs
            )
        }
    }
}
