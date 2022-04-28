/**
# ArtistRegistery contract

This contract, owned by the platform admin, allows to define the artists metadata and vaults to receive their cut. 

 */

import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393

access(all) contract ArtistRegistery {

    // -----------------------------------------------------------------------
    // Variables 
    // -----------------------------------------------------------------------
    // Manager public path, allowing an Admin to initialize it
    pub let ManagerPublicPath: PublicPath
    // Manager storage path, for a manager to manager an artist
    pub let ManagerStoragePath: StoragePath
    // Admin storage path
    pub let AdminStoragePath: StoragePath
    // Admin private path, allowing initialized Manager to cmanage an artist
    pub let AdminPrivatePath: PrivatePath

    access(self) var artists: @{UInt64: Artist}
    access(self) var artistTempVaults: @{UInt64: FUSD.Vault} // temporary vaults when the artists' ones are not available
    pub var numberOfArtists: UInt64 

    // -----------------------------------------------------------------------
    // Events 
    // -----------------------------------------------------------------------
    
    // An artist has been created
    pub event Created(id: UInt64, name: String)
    // An artist has received FUSD
    pub event FUSDReceived(id: UInt64, amount: UFix64)
    // An artist vault has been updated
    pub event VaultUpdated(id: UInt64)
    // An artist name has been updated
    pub event NameUpdated(id: UInt64, name: String)

    // -----------------------------------------------------------------------
    // Resources 
    // -----------------------------------------------------------------------

    // Resource representing a single artist
    pub resource Artist {
        pub let id: UInt64
        pub var name: String
        access(self) var vault: Capability<&FUSD.Vault{FungibleToken.Receiver}>?

        init(name: String) {
            ArtistRegistery.numberOfArtists = ArtistRegistery.numberOfArtists + (1 as UInt64)
            self.id = ArtistRegistery.numberOfArtists

            self.name = name
            self.vault = nil

            emit Created(id: self.id, name: name)
        }

        // Update the artist name
        access(contract) fun updateName(name: String) {
            self.name = name
            emit NameUpdated(id: self.id, name: name)
        }

        // Update the artist vault to receive their cut from sales and auctions
        access(contract) fun updateVault(vault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            self.vault = vault
            emit VaultUpdated(id: self.id)
        }

        // Send the artist share. If the artist vault is not available, it is put in a temporary vault
        pub fun sendShare(deposit: @FungibleToken.Vault) {
            let balance = deposit.balance 
            if(self.vault != nil && self.vault!.check()) {
                let vaultRef = self.vault!.borrow()!
                vaultRef.deposit(from: <-deposit)
            }
            else {
                // If the artist vault is anavailable, put the tokens in a temporary vault
                let artistTempVault = &ArtistRegistery.artistTempVaults[self.id] as &FUSD.Vault
                artistTempVault.deposit(from: <-deposit) // will fail if it is not a @FUSD.Vault
            }

            emit FUSDReceived(id: self.id, amount: balance)
        }

        // When an artist vault is not available, the tokens are put in a temporary vault
        // This function allows to release the funds to the artist's newly set vault
        pub fun unlockArtistShare() {
            let artistTempVault = &ArtistRegistery.artistTempVaults[self.id] as &FUSD.Vault
            if(self.vault != nil && self.vault!.check()) {
                let vaultRef = self.vault!.borrow()!
                vaultRef.deposit(from: <-artistTempVault.withdraw(amount: artistTempVault.balance))
            } 
            else {
                panic("Cannot borrow artist's vault")
            }
        }
    }

    // ArtistModifier
    //
    // An artist modifier can update the artist info
    //
    pub resource interface ArtistModifier {
        // Update an artist's name
        pub fun updateName(id: UInt64, name: String) 

        // Update an artist's vault
        pub fun updateVault(id: UInt64, vault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) 

        // When an artist vault is not available, the tokens are put in a temporary vault
        // This function allows to release the funds to the artist's newly set vault
        pub fun unlockArtistShare(id: UInt64) 

    }

    // An admin creates artists, allowing Sale and Auction contract to send the artist share
    pub resource Admin: ArtistModifier {
        // Create an artist
        pub fun createArtist(name: String) {
            let artist <- create Artist(name: name)

            ArtistRegistery.artistTempVaults[artist.id] <-! FUSD.createEmptyVault() as! @FUSD.Vault
            ArtistRegistery.artists[artist.id] <-! artist
        }

        // Update an artist's name
        // If artist doesn't exist, will not fail, nothing will happen
        pub fun updateName(id: UInt64, name: String) {
            ArtistRegistery.artists[id]?.updateName(name: name)
        }

        // Update an artist's vault
        // If artist doesn't exist, will not fail, nothing will happen
        pub fun updateVault(id: UInt64, vault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                vault.check(): "The artist vault should be available"
            }

           ArtistRegistery.artists[id]?.updateVault(vault: vault)
        }

        // When an artist vault is not available, the tokens are put in a temporary vault
        // This function allows to release the funds to the artist's newly set vault
        // If artist doesn't exist, will not fail, nothing will happen
        pub fun unlockArtistShare(id: UInt64) {
            ArtistRegistery.artists[id]?.unlockArtistShare()
        }
    }
    
    pub resource interface ManagerClient {
        pub fun setArtist(artistID: UInt64, server: Capability<&Admin{ArtistModifier}>)
    }

    // Manager
    //
    // A manager can change the artist vault and name
    //
    pub resource Manager: ManagerClient {
        access(self) var artistID: UInt64?
        access(self) var server: Capability<&Admin{ArtistModifier}>?

        init() {
            self.artistID = nil 
            self.server = nil
        }

        pub fun setArtist(artistID: UInt64, server: Capability<&Admin{ArtistModifier}>) {
            pre {
                server.check() : "Invalid server capablity"
                self.server == nil : "Server already set"
                self.artistID == nil : "Artist already set"
            }
            self.server = server
            self.artistID = artistID
        }

        // Update an artist's name
        pub fun updateName(name: String) {
            pre {
                self.server != nil : "Server not set"
                self.artistID != nil : "Artist not set"
            }
            if let artistModifier = self.server!.borrow() {
                artistModifier.updateName(id: self.artistID!, name: name)
                return 
            }
            panic("Could not borrow the artist modifier")
        }

        // Update an artist's vault
        pub fun updateVault(vault: Capability<&FUSD.Vault{FungibleToken.Receiver}>) {
            pre {
                self.server != nil : "Server not set"
                self.artistID != nil : "Artist not set"
            }
            if let artistModifier = self.server!.borrow() {
                artistModifier.updateVault(id: self.artistID!, vault: vault)
                return
            }
            panic("Could not borrow the artist modifier")
        }

        // When an artist vault is not available, the tokens are put in a temporary vault
        // This function allows to release the funds to the artist's newly set vault
        pub fun unlockArtistShare() {
            pre {
                self.server != nil : "Server not set"
                self.artistID != nil : "Artist not set"
            }
            if let artistModifier = self.server!.borrow() {
                artistModifier.unlockArtistShare(id: self.artistID!)
                return
            }
            panic("Could not borrow the artist modifier")
        }
    }
    

    // -----------------------------------------------------------------------
    // Contract public functions
    // -----------------------------------------------------------------------

    pub fun createManager(): @Manager {
        return <- create Manager()
    }

    // Send the artist share. If the artist vault is not available, it is put in a temporary vault
    pub fun sendArtistShare(id: UInt64, deposit: @FungibleToken.Vault) {
        pre {
            ArtistRegistery.artists[id] != nil: "No such artist"
        }
        let artist = &ArtistRegistery.artists[id] as &Artist
        artist.sendShare(deposit: <-deposit)
    }

    // Get an artist's data (name, vault capability)
    pub fun getArtistName(id: UInt64): String {
        pre {
            ArtistRegistery.artists[id] != nil: "No such artist"
        }
        let artist = &ArtistRegistery.artists[id] as &Artist
        return artist.name
    }


            
    // -----------------------------------------------------------------------
    // Initialization function
    // -----------------------------------------------------------------------

    init() {
        self.ManagerPublicPath = /public/boulangerieV1artistRegisteryManager
        self.ManagerStoragePath = /storage/boulangerieV1artistRegisteryManager
        self.AdminStoragePath = /storage/boulangerieV1artistRegisteryAdmin
        self.AdminPrivatePath = /private/boulangerieV1artistRegisteryAdmin

        self.artists <- {}
        self.artistTempVaults <- {}
        self.numberOfArtists = 0

        let admin <- create Admin()
        self.account.save(<-admin, to: self.AdminStoragePath)
        self.account.link<&Admin{ArtistModifier}>(self.AdminPrivatePath, target: self.AdminStoragePath)
    }
}
