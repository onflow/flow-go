import FlowEpoch from "FlowEpoch"
import FlowIDTableStaking from "FlowIDTableStaking"
import FlowClusterQC from "FlowClusterQC"

// The recoverEpoch transaction creates and starts a new epoch in the FlowEpoch smart contract
// to which will force the network exit EFM. The recoverEpoch service event will be emitted
// and processed by all protocol participants and each participant will update their protocol
// state with the new Epoch data.
// This transaction should only be used with the output of the bootstrap utility:
//   util epoch efm-recover-tx-args
transaction(startView: UInt64,
            stakingEndView: UInt64,
            endView: UInt64,
            targetDuration: UInt64,
            targetEndTime: UInt64,
            clusterAssignments: [[String]],
            clusterQCVoteData: [FlowClusterQC.ClusterQCVoteData],
            dkgPubKeys: [String],
            nodeIDs: [String],
            initNewEpoch: Bool) {

    prepare(signer: auth(BorrowValue) &Account) {
        let epochAdmin = signer.storage.borrow<&FlowEpoch.Admin>(from: FlowEpoch.adminStoragePath)
            ?? panic("Could not borrow epoch admin from storage path")

        if initNewEpoch {
            epochAdmin.recoverNewEpoch(startView: startView,
                    stakingEndView: stakingEndView,
                    endView: endView,
                    targetDuration: targetDuration,
                    targetEndTime: targetEndTime,
                    clusterAssignments: clusterAssignments  ,
                    clusterQCVoteData: clusterQCVoteData,
                    dkgPubKeys: dkgPubKeys,
                    nodeIDs: nodeIDs)
        } else {
            epochAdmin.recoverCurrentEpoch(startView: startView,
                    stakingEndView: stakingEndView,
                    endView: endView,
                    targetDuration: targetDuration,
                    targetEndTime: targetEndTime,
                    clusterAssignments: clusterAssignments  ,
                    clusterQCVoteData: clusterQCVoteData,
                    dkgPubKeys: dkgPubKeys,
                    nodeIDs: nodeIDs)
        }
    }
}
