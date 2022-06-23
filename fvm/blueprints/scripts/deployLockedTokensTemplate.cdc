transaction(publicKeys: [[UInt8]]) {

    prepare(admin: AuthAccount) {
        admin.contracts.add(name: "LockedTokens", code: "%s".decodeHex(), admin)

    }
}
