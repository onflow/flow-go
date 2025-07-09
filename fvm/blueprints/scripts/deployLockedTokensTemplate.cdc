transaction(publicKeys: [[UInt8]], code: String) {

    prepare(admin: auth(AddContract, Storage) &Account) {
        admin.contracts.add(name: "LockedTokens", code: code.decodeHex(), admin)

    }
}
