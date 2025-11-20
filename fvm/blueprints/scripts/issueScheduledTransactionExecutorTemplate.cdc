import "FlowTransactionScheduler"

transaction {
	prepare(serviceAccount: auth(Capabilities) &Account, capabilityAccount: auth(Storage) &Account) {
        let capability = serviceAccount.capabilities.storage.issue<auth(FlowTransactionScheduler.Execute) &FlowTransactionScheduler.SharedScheduler>(/storage/sharedScheduler)
        capabilityAccount.storage.save(capability, to: /storage/executeScheduledTransactionsCapability)
	}
}
