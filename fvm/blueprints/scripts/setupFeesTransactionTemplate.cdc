import FlowFees from "FlowFees"

transaction(surgeFactor: UFix64, inclusionEffortCost: UFix64, executionEffortCost: UFix64) {
    prepare(service: auth(BorrowValue) &Account) {

        let flowFeesAdmin = service.storage.borrow<&FlowFees.Administrator>(from: /storage/flowFeesAdmin)
            ?? panic("Could not borrow reference to the flow fees admin!");

        flowFeesAdmin.setFeeParameters(surgeFactor: surgeFactor, inclusionEffortCost: inclusionEffortCost, executionEffortCost: executionEffortCost)
    }
}
