import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe

pub contract NFTAirDrop {

    pub event Claimed(
        nftType: Type,
        nftID: UInt64,
        recipient: Address,
    )

    pub let DropStoragePath: StoragePath
    pub let DropPublicPath: PublicPath

    /* Custom public interface for our drop capability. */
    pub resource interface DropPublic {
        pub fun claim(
            id: UInt64,
            signature: [UInt8],
            receiver: &{NonFungibleToken.CollectionPublic}
        )

        pub fun getClaims(): {UInt64: String}
    }

    /* Drop
    Resource that will store pre-minted claimable NFTs.
    Upon minting, a public/private key pair is generated,
    users will be able to claim the NFT with the private key
    and NFTs are stored along with the public key to verify it matches.
    The project using this smart contract will only have to pre-mint,
    and share the private keys with users ahead of time.
    This smart contract is generic and can be used to store any NFT type.
    But the DropStoragePath still limits to 1 drop (hence 1 type) per account.
    */
    pub resource Drop: DropPublic {

        access(self) let nftType: Type
        access(self) let collection: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>
        // map of nft ids to decoded public claim keys
        access(self) let claims: {UInt64: [UInt8]}

        init(
            nftType: Type,
            collection: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>
        ) {
            self.nftType = nftType
            self.collection = collection
            self.claims = {}
        }

        // list yet-to-be-claimed NFTs along with the public claim keys
        pub fun getClaims(): {UInt64: String} {
            let encodedClaims: {UInt64: String} = {}
            for nftID in self.claims.keys {
                encodedClaims[nftID] = String.encodeHex(self.claims[nftID]!)
            }
            return encodedClaims
        }

        /* A Drop acts as a proxy for a Collection:
        when a token is deposited, add the (token.id, claimKey) pair to the Drop resource,
        and forward the token to the Collection that the Drop was initialised with.
        */
        pub fun deposit(token: @NonFungibleToken.NFT, publicKey: [UInt8]) {
            let collection = self.collection.borrow()!

            self.claims[token.id] = publicKey

            collection.deposit(token: <- token)
        }

        // claim a claimable NFT part of the Drop using a cryptographic signature
        pub fun claim(
            id: UInt64,
            signature: [UInt8],
            receiver: &{NonFungibleToken.CollectionPublic}
        ) {
            let collection = self.collection.borrow()!

            let rawPublicKey = self.claims.remove(key: id) ?? panic("no claim exists for NFT")

            let publicKey = PublicKey(
                publicKey: rawPublicKey,
                signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
            )

            let address = receiver.owner!.address

            let message = self.makeClaimMessage(receiverAddress: address, nftID: id)

            let isValidClaim = publicKey.verify(
                signature: signature,
                signedData: message,
                domainSeparationTag: "FLOW-V0.0-user",
                hashAlgorithm: HashAlgorithm.SHA3_256
            )

            assert(isValidClaim, message: "invalid claim signature")

            receiver.deposit(token: <- collection.withdraw(withdrawID: id))

            emit Claimed(nftType: self.nftType, nftID: id, recipient: address)
        }

        pub fun makeClaimMessage(receiverAddress: Address, nftID: UInt64): [UInt8] {
            let addressBytes = receiverAddress.toBytes()
            let idBytes = nftID.toBigEndianBytes()
            return addressBytes.concat(idBytes)
        }
    }

    pub fun createDrop(
        nftType: Type,
        collection: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>
    ): @Drop {
        return <- create Drop(nftType: nftType, collection: collection)
    }

    init() {
        self.DropStoragePath = /storage/NFTAirDrop
        self.DropPublicPath = /public/NFTAirDrop
    }
}
