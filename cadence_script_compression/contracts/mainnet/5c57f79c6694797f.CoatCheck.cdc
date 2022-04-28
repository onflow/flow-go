import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448


// CoatCheck
//
// A Smart contract Meant to hold items for another address should they not be able to receive them.
// this contract is essentially an escrow that no one holds the keys to. It will support holding any Fungible and NonFungible Tokens
// with a specified address that is allowed to claim them. In this way, dapps which need to ensure that accounts are able to "Receive" assets
// have a way of putting them into a holding resource that the intended receiver can redeem.
pub contract CoatCheck {

    // | ---------------- Events ----------------- |
    pub event Initialized()

    pub event TicketCreated(ticketResourceID: UInt64, redeemer: Address, fungibleTokenTypes: [Type], nonFungibleTokenTypes: [Type])

    pub event TicketRedeemed(ticketResourceID: UInt64, redeemer: Address)

    pub event ValetDestroyed(resourceID: UInt64)

    access(account) let valet: @CoatCheck.Valet

    // A CoatCheck has been initialized. This resource can now be watched for deposited items that accounts can redeem
    pub event CoatCheckInitialized(resourceID: UInt64)

    pub resource interface TicketPublic {
        pub fun redeem(fungibleTokenReceiver: &{FungibleToken.Receiver}?, nonFungibleTokenReceiver: &{NonFungibleToken.CollectionPublic}?)
        pub fun getDetails(): CoatCheck.TicketDetails
    }

    pub struct TicketDetails {
        pub let nftTypes: [Type]
        pub let vaultTypes: [Type]
        pub let redeemer: Address

        init(
            redeemer: Address,
            nftTypes: [Type],
            vaultTypes: [Type]
        ) {
            self.redeemer = redeemer
            self.nftTypes = nftTypes
            self.vaultTypes = vaultTypes
        }
    }

    // Tickets are used to fungible tokens, non-fungible tokens, or both.
    // A ticket can be created by the CoatCheck valet, and can be redeemed only by
    // capabilities owned by the designated redeemer of a ticket
    pub resource Ticket: TicketPublic {
        // a ticket can have Fungible Tokens AND NonFungibleTokens
        access(self) var fungibleTokenVaults: @[FungibleToken.Vault]?
        access(self) var nonFungibleTokens: @[NonFungibleToken.NFT]?
        
        pub let details: TicketDetails

        // The following variables are maintained by the CoatCheck contract
        access(self) var redeemed: Bool // a ticket can only be redeemed once. It also cannot be destroyed unless redeemed is set to true
        access(self) var storageFee: UFix64 // the storage fee taken to hold this ticket in storage. It is returned when the ticket is redeemed.

        init(
            fungibleTokenVaults: @[FungibleToken.Vault]?,
            nonFungibleTokens: @[NonFungibleToken.NFT]?,
            redeemer: Address
        ) {
            assert(
                fungibleTokenVaults != nil || nonFungibleTokens != nil, 
                message: "must provide either FungibleToken vaults or NonFungibleToken NFTs"
            )

            let ftTypes: [Type] = []
            let nftTypes: [Type] = []

            let ticketVaults: @[FungibleToken.Vault] <- []
            let ticketTokens: @[NonFungibleToken.NFT] <- []

            if fungibleTokenVaults != nil {
                while fungibleTokenVaults?.length! > 0 {
                    let vault <- fungibleTokenVaults?.removeFirst()                  
                    ftTypes.append(vault?.getType()!)
                    ticketVaults.append(<-vault!)
                }
            }

            if nonFungibleTokens != nil {
                while nonFungibleTokens?.length! > 0 {
                    let token <- nonFungibleTokens?.removeFirst()
                    nftTypes.append(token?.getType()!)
                    ticketTokens.append(<-token!)
                }
            }

            if ftTypes.length > 0 {
                self.fungibleTokenVaults <- ticketVaults
            } else {
                self.fungibleTokenVaults <- nil
                destroy ticketVaults
            }

            if nftTypes.length > 0 {
                self.nonFungibleTokens <- ticketTokens
            } else {
                self.nonFungibleTokens <- nil
                destroy ticketTokens
            }

            self.details = TicketDetails(redeemer: redeemer, nftTypes: nftTypes, vaultTypes: ftTypes)
        
            self.redeemed = false
            self.storageFee = 0.0

            emit TicketCreated(ticketResourceID: self.uuid, redeemer: redeemer, fungibleTokenTypes: ftTypes, nonFungibleTokenTypes: nftTypes)

            destroy fungibleTokenVaults
            destroy nonFungibleTokens
        }

        // redeem the ticket using an optional receiver for fungible tokens and non-fungible tokens. The supplied receivers must be
        // owned by the redeemer of this ticket.
        pub fun redeem(fungibleTokenReceiver: &{FungibleToken.Receiver}?, nonFungibleTokenReceiver: &{NonFungibleToken.CollectionPublic}?) {
            pre {
                fungibleTokenReceiver == nil || (fungibleTokenReceiver!.owner!.address == self.details.redeemer) : "incorrect owner"
                nonFungibleTokenReceiver == nil || (nonFungibleTokenReceiver!.owner!.address == self.details.redeemer) : "incorrect owner"
                self.fungibleTokenVaults == nil || fungibleTokenReceiver != nil: "must provide fungibleTokenReceiver when there is a vault to claim"
                self.nonFungibleTokens == nil || nonFungibleTokenReceiver != nil: "must provide nonFungibleTokenReceiver when there is a vault to claim"
            }

            self.redeemed = true

            let vaults <- self.fungibleTokenVaults <- nil
            let tokens <- self.nonFungibleTokens <- nil

            // do we have vaults to distribute?
            if vaults != nil && vaults?.length! > 0 {
                while vaults?.length! > 0 {
                    // pop them off our list of vaults one by one and deposit them
                    let vault <- vaults?.remove(at: 0)!
                    fungibleTokenReceiver!.deposit(from: <-vault)
                }
            }

            // do we have nfts to distribute?
            if tokens != nil && tokens?.length! > 0 {
                while tokens?.length! > 0 {
                    // pop them off our list of tokens one by one and deposit them
                    let token <- tokens?.remove(at: 0)!
                    nonFungibleTokenReceiver!.deposit(token: <-token)
                }
            }

            emit TicketRedeemed(ticketResourceID: self.uuid, redeemer: self.details.redeemer)

            destroy vaults
            destroy tokens
        }

        pub fun getDetails(): CoatCheck.TicketDetails {
            return self.details
        }

        destroy () {
            pre {
                self.redeemed : "not redeemed"
            }
        
            destroy self.fungibleTokenVaults
            destroy self.nonFungibleTokens
        }
    }

    // ValetPublic contains our main entry-point methods that
    // anyone can use to make/redeem tickets.
    pub resource interface ValetPublic {
        // redeem a ticket, supplying an optional receiver to use for depositing
        // any fts or nfts in the ticket
        pub fun redeemTicket(
            ticketID: UInt64, 
            fungibleTokenReceiver: &{FungibleToken.Receiver}?,
            nonFungibleTokenReceiver: &{NonFungibleToken.CollectionPublic}?,
        )
        pub fun borrowTicket(ticketID: UInt64): &Ticket{TicketPublic}?
        
    }

    pub resource Valet: ValetPublic {
        access(self) var tickets: @{UInt64: Ticket}

        // we store the fees taken when a ticket is made so that the exact amount is withdrawn
        // when a ticket is redeemed
        access(self) var feesByTicketID: {UInt64: UFix64}

        init() {
            self.tickets <- {}
            self.feesByTicketID = {}
        }

        // Create a new ticket containing a list of vaults, nfts, or both.
        // The creator of a ticket must also include a vault to pay fees with,
        // and a receiver to refund the fee taken once a ticket is redeemed.
        // Any extra tokens sent for the storage fee are sent back when the ticket is made
        access(account) fun createTicket(
            redeemer: Address, 
            vaults: @[FungibleToken.Vault]?, 
            tokens: @[NonFungibleToken.NFT]?, 
        ) {
            let ticket <- create Ticket(
                fungibleTokenVaults: <-vaults,
                nonFungibleTokens: <-tokens,
                redeemer: redeemer,
            )

            let ticketID = ticket.uuid
            let oldTicket <- self.tickets[ticketID] <- ticket
            destroy oldTicket
        }

        pub fun borrowTicket(ticketID: UInt64): &Ticket{TicketPublic}? {
             if self.tickets[ticketID] != nil {
                return &self.tickets[ticketID] as! &Ticket{TicketPublic}
            } else {
                return nil
            }
        }

        // redeem the ticket using supplied receivers.
        // if a ticket has fungible tokens, the fungibleTokenReceiver is required.
        // if a ticket has nfts, the nonFungibleTokenReceiver is required.
        pub fun redeemTicket(
            ticketID: UInt64, 
            fungibleTokenReceiver: &{FungibleToken.Receiver}?,
            nonFungibleTokenReceiver: &{NonFungibleToken.CollectionPublic}?
        ) {
            pre {
                self.tickets[ticketID] != nil : "ticket does not exist"
            }
            // take the ticket out of storage and redeem it
            let ticket <-! self.tickets[ticketID] <- nil
            ticket?.redeem(fungibleTokenReceiver: fungibleTokenReceiver, nonFungibleTokenReceiver: nonFungibleTokenReceiver)
            destroy ticket
        }

        destroy () {
            emit ValetDestroyed(resourceID: self.uuid)
            destroy self.tickets
        }
    }

    pub fun getValetPublic(): &CoatCheck.Valet{CoatCheck.ValetPublic} {
        return &self.valet as &CoatCheck.Valet{CoatCheck.ValetPublic}
    }

    access(account) fun getValet(): &CoatCheck.Valet {
        return &self.valet as &CoatCheck.Valet
    }

    init() {
        self.valet <- create Valet()
    }
}
 