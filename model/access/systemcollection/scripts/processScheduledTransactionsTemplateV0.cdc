import "FlowTransactionScheduler"

// Process scheduled transactions by the FlowTransactionScheduler contract.
// This will be called by the FVM and all scheduled transactions that should be 
// executed will be processed. An event for each will be emitted.
transaction {
    prepare(serviceAccount: auth(BorrowValue) &Account) {
        let scheduler = serviceAccount.storage.borrow<auth(FlowTransactionScheduler.Process) &FlowTransactionScheduler.SharedScheduler>(from: FlowTransactionScheduler.storagePath)
            ?? panic("Could not borrow FlowTransactionScheduler")

        scheduler.process()
    }
}