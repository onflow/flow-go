import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61
import BarterYardPackNFT from 0xa95b021cf8a30d80
import BarterYardClubWerewolf from 0x28abb9f291cadaf2

/*
    BarterYardClubWerewolfSale
    This contract will be used for the minting of your membership NFTs
    The sale will have 3 stages :
        - Alpha private -> allows minting for 19Flow per NFT limited to 5 NFT per pass
        - Beta private -> allows minting for 25Flow per NFT limited to 5 NFT per pass
        - Public private -> allows minting for 31Flow per NFT with no limit
    For Alpha Pass, if they are not fully used, they can be used to mint in Beta sale with Beta price.
*/

pub contract BarterYardClubWerewolfSale {
    pub let AdminStoragePath: StoragePath
    pub let MaxMintPerPass: UInt8

    access(self) let stages: {StageName: Stage}
    access(self) let nftsForSale: {UInt16: WerewolfInfo}
    access(self) let usedMintPass: {UInt64: UInt8}

    /// saleWallet is the vault where funds will be deposited when an NFT is bought
    access(self) let saleWallet: Capability<&FlowToken.Vault{FungibleToken.Receiver}>
    // minter is a capability to allow users to mint using smart contract minting function
    access(self) let minter: Capability<&BarterYardClubWerewolf.Admin>

    pub event ContractInitialized()
    pub event BarterYardClubWerewolfPurchased(id: UInt64, stage: UInt8, remaining: UInt16, to: Address)
    pub event SaleOpened(_ stage: UInt8)
    pub event SaleClosed(_ stage: UInt8)

    pub enum StageName: UInt8 {
        pub case Alpha
        pub case Beta
        pub case Public
    }

    pub struct Stage {
        pub let name: StageName
        pub let price: UInt8
        pub let maxSupply: UInt16
        pub var remaining: UInt16
        pub var opened: Bool

        pub fun openSale() {
            self.opened = true
            emit SaleOpened(self.name.rawValue)
        }

        pub fun closeSale() {
            self.opened = false
            emit SaleClosed(self.name.rawValue)
        }

        pub fun sold(number: UInt16) {
            self.remaining = self.remaining - number
        }

        init(
            name: StageName,
            price: UInt8,
            maxSupply: UInt16,
        ) {
            self.name = name
            self.price = price
            self.maxSupply = maxSupply
            self.remaining = maxSupply
            self.opened = false
        }
    }

    pub struct WerewolfInfo {
        pub let name: String
        access(contract) let attributes: [BarterYardClubWerewolf.NftTrait; 8]
        pub let description: String
        pub let dayImage: MetadataViews.IPFSFile
        pub let nightImage: MetadataViews.IPFSFile
        pub let animation: MetadataViews.IPFSFile

        init(
            name: String,
            description: String,
            attributes: [BarterYardClubWerewolf.NftTrait; 8],
            dayImage: MetadataViews.IPFSFile,
            nightImage: MetadataViews.IPFSFile,
            animation: MetadataViews.IPFSFile,
        ) {
            self.name = name
            self.description = description
            self.attributes = attributes
            self.dayImage = dayImage
            self.nightImage = nightImage
            self.animation = animation
        }
    }

    pub fun mint(
        recipient: &BarterYardClubWerewolf.Collection{NonFungibleToken.CollectionPublic},
        mintingNumber: UInt8,
        saleStage: StageName,
        buyTokens: @FungibleToken.Vault,
        mintPassCollection: &BarterYardPackNFT.Collection?
    ): @FungibleToken.Vault? {
        pre {
            self.stages[saleStage] != nil:
                "Stage not found"
            self.stages[saleStage]!.opened:
                "[BarterYardClubWerewolfSale](mint): Couldn't mint in given sale stage because it is closed."
            UFix64(UInt16(self.stages[saleStage]!.price) * UInt16(mintingNumber)) <= buyTokens.balance:
                "[BarterYardClubWerewolfSale](mint): Couldn't proceed to mint because not enough tokens were given."
            self.stages[saleStage]!.remaining > 0:
                "[BarterYardClubWerewolfSale](mint): Sale stage is sold out."
            self.nftsForSale.length > 0:
                "[BarterYardClubWerewolfSale](mint): Sale is sold out."
        }

        // Get min (mintingNumber, remaining) to mint maximum remaining NFTs in sale
        let maxMintingNumber = UInt16(mintingNumber) < self.stages[saleStage]!.remaining
                ? mintingNumber
                : UInt8(self.stages[saleStage]!.remaining);
        var numberToMint: UInt8 = maxMintingNumber

        if saleStage != StageName.Public {
            numberToMint = 0

            assert(mintPassCollection != nil, message: "[BarterYardClubWerewolfSale](mint): Missing mint pass collection. Access to private sell is restricted to Mint Pass owner.")

            let passesIDs = mintPassCollection!.getIDs()
            var remainingPassToUse = maxMintingNumber

            for passID in passesIDs {
                let pass = mintPassCollection!.borrowBarterYardPackNFT(id: passID)!

                // Only Alpha mint pass can be used for alpha sale, but both can be used for beta sale.
                if (saleStage == StageName.Alpha && pass.packPartId == 0) || saleStage == StageName.Beta {
                    var mintPassFullyUsed = self.usedMintPass[passID] == self.MaxMintPerPass

                    if mintPassFullyUsed {
                        continue
                    }

                    while remainingPassToUse > 0 && !mintPassFullyUsed {
                        self.usedMintPass[passID] = (self.usedMintPass[passID] ?? 0) + 1
                        remainingPassToUse = remainingPassToUse - 1
                        numberToMint = numberToMint + 1
                        if self.usedMintPass[passID] == self.MaxMintPerPass {
                            mintPassFullyUsed = true
                        }
                    }
                }
            }

            assert(numberToMint != 0, message: "[BarterYardClubWerewolfSale](mint): No available minting pass were found.")
        }

        var minted: UInt8 = 0
        while minted < numberToMint {
            let availableNfts = self.nftsForSale.keys
            let random = (UInt16)(
            (Fix64(unsafeRandom() % 100000)
                .saturatingDivide(99999.0))
            .saturatingMultiply(Fix64(availableNfts.length)))

            let nftInfo = self.nftsForSale.remove(key: availableNfts[random])!
            let minter = self.minter.borrow() ?? panic ("[BarterYardClubWerewolfSale](mint): Couldn't borrow minter.")

            let nft <- minter.mintNFT(
                name: nftInfo.name,
                description: nftInfo.description,
                attributes: nftInfo.attributes,
                dayImage: nftInfo.dayImage,
                nightImage: nftInfo.nightImage,
                animation: nftInfo.animation
            )

            // deposit the NFT into the buyers collection
            recipient.deposit(token: <-nft)

            self.stages[saleStage]!.sold(number: 1)

            emit BarterYardClubWerewolfPurchased(
                id: BarterYardClubWerewolf.totalSupply,
                stage: saleStage.rawValue,
                remaining: self.stages[saleStage]!.remaining,
                to: recipient.owner!.address
            )
            minted = minted + 1
        }

        let refund = buyTokens.balance - UFix64(UInt16(numberToMint) * UInt16(self.stages[saleStage]!.price))
        var refundVault: @FungibleToken.Vault? <- nil

        if refund > 0.0 {
            let tmp <- refundVault <- buyTokens.withdraw(amount: refund)
            destroy tmp
        }

        let saleWallet = self.saleWallet.borrow()!
        saleWallet.deposit(from: <-buyTokens)

        return <- refundVault
    }

    pub fun getStages(): [Stage] {
        return self.stages.values
    }

    /// Resource that an admin would own to manage the sale
    pub resource Admin {
        pub fun startStage(_ stageName: StageName) {
            let stage = BarterYardClubWerewolfSale.stages[stageName]
                ?? panic("[Admin](startStage): couldn't retrieve stage")
            stage.openSale()
            BarterYardClubWerewolfSale.stages[stageName] = stage
        }

        pub fun closeStage(_ stageName: StageName) {
            let stage = BarterYardClubWerewolfSale.stages[stageName]
                ?? panic("[Admin](closeStage): couldn't retrieve stage")
            stage.closeSale()
            BarterYardClubWerewolfSale.stages[stageName] = stage
        }

        pub fun putWerewolfForSale(werewolfInfo: WerewolfInfo) {
            let barterYardTraits = BarterYardClubWerewolf.getNftTraits()

            for trait in werewolfInfo.attributes {
                assert(barterYardTraits.containsKey(trait.id), message: "[Admin](putWerewolfForSale): Trait is not supported.")
            }

            BarterYardClubWerewolfSale.nftsForSale.insert(
                key: (UInt16)(BarterYardClubWerewolfSale.nftsForSale.length),
                werewolfInfo,
            )
        }

        pub fun removeWerewolfFromSale(key: UInt16) {
            BarterYardClubWerewolfSale.nftsForSale.remove(key: key)
        }
    }

    pub fun getNftsForSale(): [BarterYardClubWerewolfSale.WerewolfInfo] {
        return BarterYardClubWerewolfSale.nftsForSale.values
    }

    init() {
        self.AdminStoragePath = /storage/BarterYardClubWerewolfSaleAdmin
        self.MaxMintPerPass = 5

        // Seting stages in stone
        self.stages = {
            StageName.Alpha: Stage(
                name: StageName.Alpha,
                price: 19,
                maxSupply: 1000,
            ),
            StageName.Beta: Stage(
                name: StageName.Beta,
                price: 25,
                maxSupply: 2300,
            ),
            StageName.Public: Stage(
                name: StageName.Public,
                price: 31,
                maxSupply: 6500,
            )
        }
        self.nftsForSale = {}
        self.usedMintPass = {}

        // Create an Admin resource and save it to storage
        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)

        self.saleWallet = self.account.getCapability<&FlowToken.Vault{FungibleToken.Receiver}>(/public/flowTokenReceiver)
        self.minter = self.account.getCapability<&BarterYardClubWerewolf.Admin>(BarterYardClubWerewolf.AdminPrivatePath)

        emit ContractInitialized()
    }
}
