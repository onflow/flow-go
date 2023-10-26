pub fun main(addr: Address): [String] {
    let acc = getAccount(addr)
    var stored: [String] = []

    acc.forEachPublic(fun (path: PublicPath, type: Type): Bool {
        stored.append(path.toString())
        return true
    })

    acc.keys.forEach(fun (key: AccountKey): Bool {
        let no = key.publicKey.verify(signature: [1], signedData: [2], domainSeparationTag: "d", hashAlgorithm: HashAlgorithm.SHA2_256)
        log(no)
        return true
    })

    let balance = acc.balance
    let availableBalance = acc.availableBalance
    let sum = balance + availableBalance
    log(sum)

    let ran = unsafeRandom()
    let block = getCurrentBlock()
    let foo = ran + block.height
    log(foo)

    let data = "666f6f".decodeHex()
    log(data)

    return stored
}
