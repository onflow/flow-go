import FlowStorageFees from 0xFLOWSTORAGEFEESADDRESS
import FlowServiceAccount from 0xFLOWSERVICEADDRESS

transaction(accountCreationFee: UFix64, minimumStorageReservation: UFix64, storageMegaBytesPerReservedFLOW: UFix64, restrictedAccountCreationEnabled: Bool) {

    prepare(service: auth(BorrowValue) &Account) {
        let serviceAdmin = service.storage.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
            ?? panic("Could not borrow reference to the flow service admin!");

        let storageAdmin = service.storage.borrow<&FlowStorageFees.Administrator>(from: /storage/storageFeesAdmin)
            ?? panic("Could not borrow reference to the flow storage fees admin!");

        serviceAdmin.setAccountCreationFee(accountCreationFee)
        serviceAdmin.setIsAccountCreationRestricted(restrictedAccountCreationEnabled)
        storageAdmin.setMinimumStorageReservation(minimumStorageReservation)
        storageAdmin.setStorageMegaBytesPerReservedFLOW(storageMegaBytesPerReservedFLOW)
    }
}
