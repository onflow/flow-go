import FlowDKG from "FlowDKG"

// This transaction submits the DKG result using the contract API compatible with Protocol Version 1.
// It is included as a backward compatibility measure during the transition to Protocol Version 2.
// TODO(mainnet27, #6792): remove this file
transaction(submission: [String?]) {

    let dkgParticipant: &FlowDKG.Participant

    prepare(signer: auth(BorrowValue) &Account) {
        self.dkgParticipant = signer.storage.borrow<&FlowDKG.Participant>(from: FlowDKG.ParticipantStoragePath)
            ?? panic("Cannot borrow dkg participant reference")
    }

    execute {
        self.dkgParticipant.sendFinalSubmission(submission)
    }
}
