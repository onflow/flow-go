
import NonFungibleToken from 0x1d7e57aa55817448

pub contract MINTNFTINCBalmainV1NonFungibleToken: NonFungibleToken {

pub var totalSupply: UInt64

pub event ContractInitialized()
pub event Withdraw(id: UInt64, from: Address?)
pub event Deposit(id: UInt64, to: Address?)
pub event NFTMinted(id: UInt64, name: String, symbol: String, description: String, collectionName: String, tokenURI: String, mintNFTId: String, mintNFTStandard: String, mintNFTVideoURI: String, thirdPartyId: String , terms: String)
pub event NFTDestroyed(id: UInt64)

pub resource NFT: NonFungibleToken.INFT {
    pub let id: UInt64

    pub let name: String 
    pub let symbol: String
    pub let description: String
    pub let collectionName: String
    pub let tokenURI: String
    pub let mintNFTId: String
    pub let mintNFTStandard: String
    pub let mintNFTVideoURI: String
    pub let thirdPartyId: String
    pub let terms: String
    pub let burnable: Bool


    init(initID: UInt64, _name: String, _symbol: String, _description: String, _collectionName: String, _tokenURI: String, _mintNFTId: String,_mintNFTStandard: String, _mintNFTVideoURI: String, _thirdPartyId: String , _terms: String) {
      
      self.id = initID
      self.name = _name
      self.symbol = _symbol
      self.description = _description
      self.collectionName = _collectionName
      self.tokenURI = _tokenURI
      self.mintNFTId = _mintNFTId
      self.mintNFTStandard = _mintNFTStandard
      self.mintNFTVideoURI = _mintNFTVideoURI
      self.thirdPartyId = _thirdPartyId
      self.terms = _terms
      self.burnable = false;

      emit NFTMinted(id: self.id, name: self.name, symbol: self.symbol, description: self.description, collectionName: self.collectionName, tokenURI: self.tokenURI, mintNFTId: self.mintNFTId, mintNFTStandard: self.mintNFTStandard, mintNFTVideoURI: self.mintNFTVideoURI, thirdPartyId: self.thirdPartyId , terms: self.terms)

    }

     destroy() {
      emit NFTDestroyed(id: self.id)
    }
    }


   pub resource interface MINTNFTINCBalmainV1CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMINTNFT(id: UInt64): &MINTNFTINCBalmainV1NonFungibleToken.NFT? {

        post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow NFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: MINTNFTINCBalmainV1CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("Cannot withdraw: NFT does not exist in the collection")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @MINTNFTINCBalmainV1NonFungibleToken.NFT
            let id: UInt64 = token.id
            let oldToken <- self.ownedNFTs[id] <- token
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldToken
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()
            
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            return <-batchCollection
        }

        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            let keys = tokens.getIDs()

            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }


        pub fun borrowMINTNFT(id: UInt64): &MINTNFTINCBalmainV1NonFungibleToken.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &MINTNFTINCBalmainV1NonFungibleToken.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

  
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }


    pub resource MINTNFTINCBalmainV1Minter {

        pub fun mintNFT(recipient: &{MINTNFTINCBalmainV1CollectionPublic}, name: String, symbol: String, description: String, collectionName: String, tokenURI: String, mintNFTId: String, mintNFTStandard: String, mintNFTVideoURI: String, thirdPartyId: String , terms: String) : UInt64 {
            
            let newNFT <- create NFT(initID: MINTNFTINCBalmainV1NonFungibleToken.totalSupply, _name: name, _symbol: symbol, _description: description, _collectionName: collectionName, _tokenURI: tokenURI, _mintNFTId: mintNFTId, _mintNFTStandard: mintNFTStandard, _mintNFTVideoURI: mintNFTVideoURI, _thirdPartyId: thirdPartyId , _terms: terms)           
            var tempId: UInt64 = newNFT.id
            recipient.deposit(token: <-newNFT)
            MINTNFTINCBalmainV1NonFungibleToken.totalSupply = MINTNFTINCBalmainV1NonFungibleToken.totalSupply + UInt64(1)

            return tempId

        }

        pub fun mintNFTBatch(count: UInt64, recipients: [&{MINTNFTINCBalmainV1CollectionPublic}], names: [String], symbols: [String], descriptions: [String], collectionNames: [String], tokenURIs: [String], mintNFTIds: [String], mintNFTStandards: [String], mintNFTVideoURIs: [String], thirdPartyIds: [String] , terms: [String]): [UInt64] {
        var nftIDs: [UInt64] = []
     

        var idx: UInt64 = 0
        var len: UInt64 = count - 1
        
        while idx < len {
          let nftId = self.mintNFT(recipient: recipients[idx], name: names[idx], symbol: symbols[idx], description: descriptions[idx], collectionName: collectionNames[idx], tokenURI: tokenURIs[idx], mintNFTId: mintNFTIds[idx], mintNFTStandard: mintNFTStandards[idx], mintNFTVideoURI: mintNFTVideoURIs[idx], thirdPartyId: thirdPartyIds[idx] , terms: terms[idx])
          nftIDs.append(nftId)
          idx = idx + 1
        }

        return nftIDs

        }

    }

    init() {
       
        self.totalSupply = 0

        self.account.save(<- create Collection(), to: /storage/MINTNFTINCBalmainV1Collection)
     
        self.account.link<&{MINTNFTINCBalmainV1CollectionPublic}>(/public/MINTNFTINCBalmainV1Collection, target: /storage/MINTNFTINCBalmainV1Collection)
   
        self.account.save(<- create MINTNFTINCBalmainV1Minter(), to: /storage/MINTNFTINCBalmainV1Minter)

        emit ContractInitialized()
    }



}
