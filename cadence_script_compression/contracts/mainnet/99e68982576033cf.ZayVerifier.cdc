//  ____   ____ _   _ __  __  ___  _____ ____  
// / ___| / ___| | | |  \/  |/ _ \| ____/ ___| 
// \___ \| |   | |_| | |\/| | | | |  _| \___ \ 
//  ___) | |___|  _  | |  | | |_| | |___ ___) |
// |____/ \____|_| |_|_|  |_|\___/|_____|____/ 
// 
// Made by amit @ zay.codes
//

import NonFungibleToken from 0x1d7e57aa55817448
import Crypto

pub contract ZayVerifier {

    // Returns timestamp of the last sealed block for when this signature was created
    // If the signature is invalid, returns nil.
    pub fun verifySignature(acctAddress: Address, message: String, keyIds: [Int], signatures: [String], signatureBlock: UInt64): UFix64? {

        let keyList = Crypto.KeyList()

        let rawPublicKeys: [String] = []
        let weights: [UFix64] = []
        let signAlgos: [UInt] = []

        let uniqueKeys: {Int: Bool} = {}
        let account = getAccount(acctAddress)
        
        for id in keyIds {
            uniqueKeys[id] = true
        }

        assert(uniqueKeys.keys.length == keyIds.length, message: "Invalid duplicates of the same keyID provided for signature")

        var counter = 0
        while (counter < keyIds.length) {
            let accountKey = account.keys.get(keyIndex: keyIds[counter]) ?? panic("Provided key signature does not exist")
            rawPublicKeys.append(String.encodeHex(accountKey.publicKey.publicKey))
            weights.append(accountKey.weight)
            signAlgos.append(UInt(accountKey.publicKey.signatureAlgorithm.rawValue))
            counter = counter + 1
        }

        var totalWeight = 0.0
        var weightIndex = 0
        while (weightIndex < weights.length) {
            totalWeight = totalWeight + weights[weightIndex]
            weightIndex = weightIndex + 1
        }
        assert(totalWeight >= 1000.0, message: "Total weight of combined signatures did not satisfy 1000 requirement.")

        var i = 0
        for rawPublicKey in rawPublicKeys {
            keyList.add(
                PublicKey(
                    publicKey: rawPublicKey.decodeHex(),
                    signatureAlgorithm: signAlgos[i] == 2 ? SignatureAlgorithm.ECDSA_secp256k1  : SignatureAlgorithm.ECDSA_P256
                ),
                hashAlgorithm: HashAlgorithm.SHA3_256,
                weight: weights[i]
            )
            i = i + 1
        }

        let signatureSet: [Crypto.KeyListSignature] = []
        var j = 0
        for signature in signatures {
            signatureSet.append(
                Crypto.KeyListSignature(
                    keyIndex: j,
                    signature: signature.decodeHex()
                )
            )
            j = j + 1
        }

        var signingBlockHashStr = ""
        counter = 0
        let signingBlock = getBlock(at: signatureBlock)!
        let id = signingBlock.id
        let ids: [UInt8] = []
        while (counter < id.length) {
            ids.append(id[counter])
            counter = counter + 1
        }
        let hexStr = String.encodeHex(ids)

        assert(message == hexStr, message: "Unable to validate signature provided contained a valid block id.")

        let signedData = message.decodeHex()
        let signatureValid = keyList.verify(
            signatureSet: signatureSet,
            signedData: signedData
        )
        if (signatureValid) {
            return signingBlock.timestamp
        } else {
            return nil
        }
    }

    pub fun checkOwnership(
        address: Address,
        collectionPath: PublicPath,
        nftType: Type
    ): Bool {
        let collectionRef = getAccount(address).getCapability<&{NonFungibleToken.CollectionPublic}>(collectionPath).borrow()
        if collectionRef == nil {
            return false
        }
        let ids = collectionRef!.getIDs()
        var index = 0
        while (index < ids.length) {
            // Borrow the NFT, and ensure it is the proper type
            let id = ids[index]
            let nft = collectionRef!.borrowNFT(id: id)
            if (nft.isInstance(nftType)) {
                return true
            }
            index = index + 1
        }
        return false
    }
}
