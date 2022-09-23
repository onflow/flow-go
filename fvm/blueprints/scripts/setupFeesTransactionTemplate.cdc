import FlowFees from 0xFLOWFEESADDRESS

transaction(surgeFactor: UFix64, inclusionEffortCost: UFix64, executionEffortCost: UFix64) {
    prepare(service: AuthAccount) {

        let flowFeesAdmin = service.borrow<&FlowFees.Administrator>(from: /storage/flowFeesAdmin)
            ?? panic("Could not borrow reference to the flow fees admin!");

        flowFeesAdmin.setFeeParameters(surgeFactor: surgeFactor, inclusionEffortCost: inclusionEffortCost, executionEffortCost: executionEffortCost)
    }
}
