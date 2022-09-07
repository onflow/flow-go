transaction(publicKeys: [[UInt8]], code: String) {

    prepare(admin: AuthAccount) {
        admin.contracts.add(name: "LockedTokens", code: code.decodeHex(), admin)

    }
}
