import FlowToken from 0xFLOWTOKENADDRESS

transaction {
	prepare(serviceAccount: auth(Storage) &Account) {
    /// Borrow a reference to the Flow Token Admin in the account storage
    let flowTokenAdmin = serviceAccount.storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
        ?? panic("Could not borrow a reference to the Flow Token Admin resource")

    /// Create a flowTokenMinterResource
    let flowTokenMinter <- flowTokenAdmin.createNewMinter(allowedAmount: 1000000000.0)

    serviceAccount.storage.save(<-flowTokenMinter, to: /storage/flowTokenMinter)
	}
}
