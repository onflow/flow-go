import FUSD from 0x3c5959b568896393
import NonFungibleToken from 0x1d7e57aa55817448
import FungibleToken from 0xf233dcee88fe0abe
import CoatCheck from 0x5c57f79c6694797f

pub contract FlowtyUtils {
    access(contract) var Attributes: {String: AnyStruct}

    pub let FlowtyUtilsStoragePath: StoragePath

    pub resource FlowtyUtilsAdmin {
        // addSupportedTokenType
        // add a supported token type that can be used in Flowty loans
        pub fun addSupportedTokenType(type: Type) {
            var supportedTokens = FlowtyUtils.Attributes["supportedTokens"]
            if supportedTokens == nil {
                supportedTokens = [Type<@FUSD.Vault>()] as! [Type] 
            }

            let tokens = supportedTokens! as! [Type]

            if !FlowtyUtils.isTokenSupported(type: type) {
                tokens.append(type)
            }

            FlowtyUtils.Attributes["supportedTokens"] = tokens
        }
    }

    pub fun getSupportedTokens(): AnyStruct {
        return self.Attributes["supportedTokens"]!
    }

    // getAllowedTokens
    // return an array of types that are able to be used as the payment type
    // for loans
    pub fun getAllowedTokens(): [Type] {
        var supportedTokens = self.Attributes["supportedTokens"]
        return supportedTokens != nil ? supportedTokens! as! [Type] : [Type<@FUSD.Vault>()]
    }

    // isTokenSupported
    // check if the given type is able to be used as payment
    pub fun isTokenSupported(type: Type): Bool {
        for t in FlowtyUtils.getAllowedTokens() {
            if t == type {
                return true
            }
        }

        return false
    }

    access(account) fun trySendFungibleTokenVault(vault: @FungibleToken.Vault, receiver: Capability<&{FungibleToken.Receiver}>){
        let redeemer = receiver.address
        if receiver.borrow() == nil {
            let valet = CoatCheck.getValet()
            let vaults: @[FungibleToken.Vault] <- []
            vaults.append(<-vault)
            valet.createTicket(redeemer: redeemer, vaults: <-vaults, tokens: nil)
        } else {
            receiver.borrow()!.deposit(from: <-vault)
        }
    }

    access(account) fun trySendNFT(nft: @NonFungibleToken.NFT, receiver: Capability<&{NonFungibleToken.CollectionPublic}>) {
        let redeemer = receiver.address
        if receiver.borrow() == nil {
            let valet = CoatCheck.getValet()
            let nfts: @[NonFungibleToken.NFT] <- []
            nfts.append(<-nft)
            valet.createTicket(redeemer: redeemer, vaults: nil, tokens: <-nfts)
        } else {
            receiver.borrow()!.deposit(token: <-nft)
        }
    }


    init() {
        self.Attributes = {}

        self.FlowtyUtilsStoragePath = /storage/FlowtyUtils

        let utilsAdmin <- create FlowtyUtilsAdmin()
        self.account.save(<-utilsAdmin, to: self.FlowtyUtilsStoragePath)
    }
}