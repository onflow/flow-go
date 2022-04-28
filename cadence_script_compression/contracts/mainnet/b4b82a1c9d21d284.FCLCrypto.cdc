pub contract FCLCrypto {
    
    pub fun verifyUserSignatures(
        address: Address,
        message: String,
        keyIndices: [Int],
        signatures: [String]
    ): Bool {
        return self.verifySignatures(
            address: address,
            message: message,
            keyIndices: keyIndices,
            signatures: signatures,
            domainSeparationTag: self.domainSeparationTagFlowUser,
        )
    }

    pub fun verifyAccountProofSignatures(
        address: Address,
        message: String,
        keyIndices: [Int],
        signatures: [String]
    ): Bool {
        return self.verifySignatures(
            address: address,
            message: message,
            keyIndices: keyIndices,
            signatures: signatures,
            domainSeparationTag: self.domainSeparationTagFlowUser,
        )
    }

    priv fun verifySignatures(
        address: Address,
        message: String,
        keyIndices: [Int],
        signatures: [String],
        domainSeparationTag: String,
    ): Bool {
        pre {
            keyIndices.length == signatures.length : "Key index list length does not match signature list length"
        }

        let account = getAccount(address)
        let messageBytes = message.decodeHex()

        var totalWeight: UFix64 = 0.0
        let seenKeyIndices: {Int: Bool} = {}

        var i = 0

        for keyIndex in keyIndices {

            let accountKey = account.keys.get(keyIndex: keyIndex) ?? panic("Key provided does not exist on account")
            let signature = signatures[i].decodeHex()

            // Ensure this key index has not already been seen

            if seenKeyIndices[accountKey.keyIndex] ?? false {
                return false
            }

            // Record the key index was seen

            seenKeyIndices[accountKey.keyIndex] = true

            // Ensure the key is not revoked

            if accountKey.isRevoked {
                return false
            }

            // Ensure the signature is valid

            if !accountKey.publicKey.verify(
                signature: signature,
                signedData: messageBytes,
                domainSeparationTag: domainSeparationTag,
                hashAlgorithm: accountKey.hashAlgorithm
            ) {
                return false
            }

            totalWeight = totalWeight + accountKey.weight

            i = i + 1
        }
        
        return totalWeight >= 1000.0
    }

    priv let domainSeparationTagFlowUser: String
    priv let domainSeparationTagFCLUser: String
    priv let domainSeparationTagAccountProof: String

    init() {
        self.domainSeparationTagFlowUser = "FLOW-V0.0-user"
        self.domainSeparationTagFCLUser = "FCL-USER-V0.0"
        self.domainSeparationTagAccountProof = "FCL-ACCOUNT-PROOF-V0.0"
    }
}
