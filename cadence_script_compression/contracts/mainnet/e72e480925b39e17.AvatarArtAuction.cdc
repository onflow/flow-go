import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import AvatarArtNFT from 0xe72e480925b39e17
import AvatarArtTransactionInfo from 0xe72e480925b39e17

// An fork of Versus Auction with some features addiontional
pub contract AvatarArtAuction {
  pub var totalAuctions: UInt64;

  pub let AuctionStoreStoragePath: StoragePath;
  pub let AdminStoragePath: StoragePath;
  pub let AuctionStorePublicPath: PublicPath;

  access(self) var priceSteps: {UFix64: UFix64};
  access(self) var feeReference: Capability<&{AvatarArtTransactionInfo.PublicFeeInfo}>?;
  access(self) var feeRecepientReference: Capability<&{AvatarArtTransactionInfo.PublicTransactionAddress}>?;

  pub event Bid(storeID: UInt64, nftID: UInt64, auctionID: UInt64, bidderAddress: Address, bidPrice: UFix64, time: UFix64);
  pub event CuttedFee(storeID: UInt64, auctionID: UInt64, fee: CutFee);
  pub event TokenPurchased(storeID: UInt64, auctionID: UInt64, nftID: UInt64, price: UFix64, from: Address, to: Address?);
  pub event AuctionCompleted(storeID: UInt64, auctionID: UInt64, tokenPurchased: Bool);
  pub event AuctionAvailable(
    storeAddress: Address,
    auctionID: UInt64,
    nftType: Type,
    nftID: UInt64,
    ftVaultType: Type,
    startPrice: UFix64,
    startTime: UFix64,
    endTime: UFix64,
    );
  pub event StoreInitialized(storeID: UInt64);
  pub event StoreDestroyed(storeID: UInt64);

  pub struct AuctionDetail{
      pub let id: UInt64;
      pub let lastPrice : UFix64;
      pub let mimimumBidIncrement : UFix64;
      pub let bids : UInt64;
      pub let timeRemaining : Fix64;
      pub let endTime : UFix64;
      pub let startTime : UFix64;
      pub let nftID: UInt64?;
      pub let owner: Address;
      pub let minNextBid: UFix64;
      pub let completed: Bool;
      pub let expired: Bool;
      pub let leader: Address?;
  
      init(id:UInt64, 
          currentPrice: UFix64, 
          bids:UInt64, 
          active: Bool, 
          timeRemaining:Fix64, 
          nftID: UInt64?,
          minimumBidIncrement: UFix64,
          owner: Address, 
          startTime: UFix64,
          endTime: UFix64,
          minNextBid:UFix64,
          completed: Bool,
          expired:Bool,
          leader: Address?
      ) {
          self.id = id;
          self.lastPrice = currentPrice;
          self.bids = bids;
          self.timeRemaining = timeRemaining;
          self.nftID = nftID;
          self.mimimumBidIncrement = minimumBidIncrement;
          self.owner = owner;
          self.startTime = startTime;
          self.endTime = endTime;
          self.minNextBid = minNextBid;
          self.completed = completed;
          self.expired = expired;
          self.leader = leader;
      }
  }

  pub struct CutFee {
      pub(set) var affiliate: UFix64;
      pub(set) var storing: UFix64;
      pub(set) var insurance: UFix64;
      pub(set) var contractor: UFix64;
      pub(set) var platform: UFix64;
      pub(set) var author: UFix64;

      init() {
        self.affiliate = 0.0;
        self.storing = 0.0;
        self.insurance = 0.0;
        self.contractor = 0.0;
        self.platform = 0.0;
        self.author = 0.0;
      }
  }


  pub resource interface AuctionPublic {
    pub fun timeRemaining() : Fix64;
    pub fun isAuctionExpired(): Bool;
    pub fun getDetails(): AuctionDetail;
    pub fun bidder() : Address?;
    pub fun currentBidForUser(address: Address): UFix64;
    pub fun placeBid(price: UFix64,
            affiliateVaultCap: Capability<&{FungibleToken.Receiver}>?,
            vaultCap: Capability<&{FungibleToken.Receiver}>,
            collectionCap: Capability<&{AvatarArtNFT.CollectionPublic}>,
            bidTokens: @FungibleToken.Vault);
  }

  pub resource AuctionItem: AuctionPublic {
    //Number of bids made, that is aggregated to the status struct
    access(self) var numberOfBids: UInt64;

    //The Item that is sold at this auction
    //It would be really easy to extend this auction with using a NFTCollection here to be able to auction of several NFTs as a single
    access(self) var nft: @AvatarArtNFT.NFT?;

    //This is the escrow vault that holds the tokens for the current largest bid
    access(self) let bidVault: @FungibleToken.Vault;

    //The start time of auction
    access(self) var startTime: UFix64;

    //The end time of auction
    access(self) var endTime: UFix64;

    access(contract) var auctionCompleted: Bool;

    // The start price to bid
    access(self) var startPrice: UFix64;

    //The minimum increment for a bid. This is an english auction style system where bids increase
    access(self) let minimumBidIncrement: UFix64

    // The last price of auction (current price)
    access(self) var lastPrice: UFix64;

    // The capablity to send the escrow bidVault affiliate 
    access(self) var affiliateVaultCap: Capability<&{FungibleToken.Receiver}>?;

    // The capability that points to the resource where you want the NFT transfered to if you win this bid. 
    access(self) var recipientCollectionCap: Capability<&{AvatarArtNFT.CollectionPublic}>?;

    // The capablity to send the escrow bidVault to if you are outbid
    access(self) var recipientVaultCap: Capability<&{FungibleToken.Receiver}>?;

    // The capability for the owner of the NFT to return the item to if the auction is cancelled
    access(self) let ownerCollectionCap: Capability<&{AvatarArtNFT.CollectionPublic}>;

    // The capability to pay the owner of the item when the auction is done
    access(self) let ownerVaultCap: Capability<&{FungibleToken.Receiver}>;

    access(self) let storeID: UInt64;

    init(
        nft: @AvatarArtNFT.NFT,
        bidVault: @FungibleToken.Vault,
        startTime: UFix64,
        endTime: UFix64,
        startPrice: UFix64, 
        minimumBidIncrement: UFix64, 
        ownerCollectionCap: Capability<&{AvatarArtNFT.CollectionPublic}>,
        ownerVaultCap: Capability<&{FungibleToken.Receiver}>,
        storeID: UInt64
    ) {
        pre {
          bidVault.balance == 0.0: "AuctionItem init: Bid Vault should be empty (balance = 0.0)"
          endTime > startTime: "Endtime should be greater than start time"
          endTime > getCurrentBlock().timestamp: "End time should be greater than current time"
        }

        AvatarArtAuction.totalAuctions = AvatarArtAuction.totalAuctions + 1
        self.nft <- nft;
        self.bidVault <- bidVault;
        self.startPrice = startPrice;
        self.minimumBidIncrement = minimumBidIncrement;
        self.lastPrice = 0.0;
        self.startTime = startTime;
        self.endTime = endTime;
        self.auctionCompleted = false;

        self.recipientCollectionCap = nil;
        self.recipientVaultCap = nil;
        self.affiliateVaultCap = nil;

        self.ownerCollectionCap = ownerCollectionCap;
        self.ownerVaultCap = ownerVaultCap;
        self.numberOfBids = 0;
        self.storeID = storeID;
    }

    // sendNFT sends the NFT to the Collection belonging to the provided Capability
    access(self) fun sendNFT(_ capability: Capability<&{AvatarArtNFT.CollectionPublic}>) {
        // Try send the NFT to provided cap
        let collectionRef = capability.borrow()!;
        let nft <- self.nft <- nil
        collectionRef.deposit(token: <-nft!)
    }

    // send the bid tokens to the Vault Receiver belonging to the provided Capability
    access(self) fun sendBidTokens(_ capability: Capability<&{FungibleToken.Receiver}>) {
        // Send bid tokens to the given capability
        let vaultRef = capability.borrow();
        let bidVaultRef = &self.bidVault as &FungibleToken.Vault

        if bidVaultRef.balance > 0.0 {
            vaultRef!.deposit(from: <-bidVaultRef.withdraw(amount: bidVaultRef.balance))
        }
    }
    
    access(self) fun releasePreviousBid() {
        if let vaultCap = self.recipientVaultCap {
            self.sendBidTokens(self.recipientVaultCap!);

            self.recipientCollectionCap = nil;
            self.recipientVaultCap = nil;
            self.affiliateVaultCap = nil;

            return
        } 
    }


    access(contract) fun returnAuctionItemToOwner() {
      // release the bidder's tokens
      self.releasePreviousBid()

      // deposit the NFT into the owner's collection
      self.sendNFT(self.ownerCollectionCap)
    }

    //this can be negative if is expired
    pub fun timeRemaining() : Fix64 {
        let currentTime = getCurrentBlock().timestamp
        return Fix64(self.endTime) - Fix64(currentTime)
    }

  
    pub fun isAuctionExpired(): Bool {
        let timeRemaining= self.timeRemaining()
        return timeRemaining < Fix64(0.0)
    }

    pub fun minNextBid() :UFix64{
        //If there are bids then the next min bid is the current price plus the increment
        if self.lastPrice != 0.0 {
            return self.lastPrice + self.minimumBidIncrement
        }
        //else start price
        return self.startPrice
    }

    pub fun bidder() : Address? {
        if let vaultCap = self.recipientVaultCap {
            return vaultCap.address
        }
        return nil
    }

    pub fun currentBidForUser(address: Address): UFix64 {
          if self.bidder() == address {
            return self.bidVault.balance
        }
        return 0.0
    }

    pub fun placeBid(
            price: UFix64,
            affiliateVaultCap: Capability<&{FungibleToken.Receiver}>?,
            vaultCap: Capability<&{FungibleToken.Receiver}>,
            collectionCap: Capability<&{AvatarArtNFT.CollectionPublic}>,
            bidTokens: @FungibleToken.Vault){
      pre {
        self.nft != nil: "The selling NFT doesnot exist"
      }

      assert(bidTokens.isInstance(self.bidVault.getType()), message: "Payment token is not allow")


      let currentTime = getCurrentBlock().timestamp;
      if currentTime < self.startTime || currentTime > self.endTime {
        panic("Invalid time to place");
      }

      let bidderAddress = vaultCap.address;
      let collectionAddress = collectionCap.address;
      if bidderAddress != collectionAddress {
        panic("you cannot make a bid and send the art to sombody elses collection")
      }

      let biddingAmount = bidTokens.balance + self.currentBidForUser(address: bidderAddress);
      let minNextBid = self.minNextBid();
      if biddingAmount < minNextBid {
          panic(
            "bid amount + (your current bid) must be larger or equal to the current price + minimum bid increment "
            .concat(biddingAmount.toString())
            .concat(" < ")
            .concat(minNextBid.toString()))
      }


      // Check incomming bidder and last bidder are same?
      if self.bidder() != bidderAddress {
        if self.bidVault.balance != 0.0 {

          // IF not, Refund to previous bidder
          self.sendBidTokens(self.recipientVaultCap!);
        }
      }

      // Update the auction item
      self.bidVault.deposit(from: <-bidTokens);

      //update the capability of the wallet for the address with the current highest bid
      self.recipientVaultCap = vaultCap;

      // Add the bidder's Vault and NFT receiver references
      self.recipientCollectionCap = collectionCap;

      // Update the current price of the token
      self.lastPrice = self.bidVault.balance;

      self.numberOfBids = self.numberOfBids + 1;

      emit Bid(
        storeID: self.storeID,
        nftID: self.nft?.id!, auctionID: self.uuid,
        bidderAddress: bidderAddress, bidPrice: self.lastPrice,
        time: getCurrentBlock().timestamp
      );
    }

    access(self) fun settleFee(nftID: UInt64) {
        if AvatarArtAuction.feeReference == nil || AvatarArtAuction.feeRecepientReference == nil {
          return
        }

        let feeOp = AvatarArtAuction.feeReference!.borrow()!.getFee(tokenId: nftID);
        let feeRecepientOp = AvatarArtAuction.feeRecepientReference!.borrow()!.getAddress(
              tokenId: nftID,
              payType: self.bidVault.getType()
        )

        if feeOp == nil || feeRecepientOp == nil {
          return 
        }

        let fee = feeOp!;
        let feeRecepient = feeRecepientOp!;


        let cutFee = CutFee()

        // Affiliate
        if fee.affiliate != nil && fee.affiliate > 0.0 && self.affiliateVaultCap != nil
            && self.affiliateVaultCap!.check() {
            let fee = self.lastPrice * fee.affiliate / 100.0;
            let feeVault <- self.bidVault.withdraw(amount: fee);
            self.affiliateVaultCap!.borrow()!.deposit(from: <- feeVault);
            cutFee.affiliate = fee;
        }

        // Storage fee
        if fee.storing != nil && fee.storing > 0.0 && feeRecepient.storing != nil
            && feeRecepient.storing!.check() {
            let fee = self.lastPrice * fee.storing / 100.0;
            let feeVault <- self.bidVault.withdraw(amount: fee);
            feeRecepient.storing!.borrow()!.deposit(from: <- feeVault);
            cutFee.storing = fee;
        }

        // Insurrance Fee
        if fee.insurance != nil && fee.insurance > 0.0 && feeRecepient.insurance != nil
          && feeRecepient.insurance!.check() {
            let fee = self.lastPrice * fee.insurance / 100.0;
            let feeVault <- self.bidVault.withdraw(amount: fee);
            feeRecepient.insurance!.borrow()!.deposit(from: <- feeVault);
            cutFee.insurance = fee;
        }

        // Dev
        if fee.contractor != nil && fee.contractor > 0.0 && feeRecepient.contractor != nil
            && feeRecepient.contractor!.check() {
            let fee = self.lastPrice * fee.contractor / 100.0;
            let feeVault <- self.bidVault.withdraw(amount: fee);
            feeRecepient.contractor!.borrow()!.deposit(from: <- feeVault);
            cutFee.contractor = fee;
        }

        // The Platform
        if fee.platform != nil && fee.platform > 0.0 && feeRecepient.platform != nil
          && feeRecepient.platform!.check() {
            let fee = self.lastPrice * fee.platform / 100.0;
            let feeVault <- self.bidVault.withdraw(amount: fee);
            feeRecepient.platform!.borrow()!.deposit(from: <- feeVault);
            cutFee.platform = fee;
        }

        // Author
        if fee.author != nil && fee.author > 0.0 && feeRecepient.author != nil
          &&  feeRecepient.author!.check()  {
            let fee = self.lastPrice * fee.author / 100.0;
            let feeVault <- self.bidVault.withdraw(amount: fee);
            feeRecepient.author!.borrow()!.deposit(from: <- feeVault);
            cutFee.author = fee;
        }

        emit CuttedFee(storeID: self.storeID, auctionID: self.uuid, fee: cutFee);
    }

    access(contract) fun settleAuction() {
        pre {
          getCurrentBlock().timestamp > self.endTime: "Auction has not ended";
          !self.auctionCompleted : "The auction is already settled"
          self.nft != nil: "The selling NFT doesnot exist";
          self.isAuctionExpired() : "Auction has not completed yet";
        }

       // return if there are no bids to settle
        if self.lastPrice == 0.0 {
            self.returnAuctionItemToOwner();
            return
        }

        let nftID = self.nft?.id!;

        self.settleFee(nftID: nftID);
        self.sendNFT(self.recipientCollectionCap!);
        self.sendBidTokens(self.ownerVaultCap);

        self.auctionCompleted = true;


        emit AuctionCompleted(storeID: self.storeID, auctionID: self.uuid, tokenPurchased: true);
        emit TokenPurchased(
            storeID: self.storeID,
            auctionID: self.uuid, 
            nftID: nftID, 
            price: self.lastPrice, 
            from: self.ownerVaultCap.address, 
            to: self.recipientCollectionCap?.address);
    } 

    pub fun getDetails(): AuctionDetail {
        var leader:Address? = nil;
        if let recipient = self.recipientVaultCap {
            leader = recipient.address
        }

        return AuctionDetail(
            id: self.uuid,
            currentPrice: self.lastPrice, 
            bids: self.numberOfBids,
            active: !self.auctionCompleted  && !self.isAuctionExpired(),
            timeRemaining: self.timeRemaining(),
            nftID: self.nft?.id,
            minimumBidIncrement: self.minimumBidIncrement,
            owner: self.ownerVaultCap.address,
            startTime: self.startTime,
            endTime: self.endTime,
            minNextBid: self.minNextBid(),
            completed: self.auctionCompleted,
            expired: self.isAuctionExpired(),
            leader: leader,
        )
    }

    destroy() {
      // send the NFT back to auction owner
      if self.nft != nil {
          self.sendNFT(self.ownerCollectionCap);
      }
      
      // if there's a bidder...
      if let vaultCap = self.recipientVaultCap {
          // ...send the bid tokens back to the bidder
          self.sendBidTokens(vaultCap);
      }

      if !self.auctionCompleted {
        emit AuctionCompleted(storeID: self.storeID, auctionID: self.uuid, tokenPurchased: false);
      }

      destroy self.nft;
      destroy self.bidVault;
    }

  }



  pub resource interface AuctionStorePublic {
    pub fun createStandaloneAuction(
        token: @AvatarArtNFT.NFT, 
        bidVault: @FungibleToken.Vault,
        startTime: UFix64,
        endTime: UFix64,
        startPrice: UFix64, 
        collectionCap: Capability<&{AvatarArtNFT.CollectionPublic}>, 
        vaultCap: Capability<&{FungibleToken.Receiver}>
    ) : UInt64;

    pub fun getAuctionIDs(): [UInt64];
    pub fun borrowAuction(auctionID: UInt64): &AuctionItem{AuctionPublic}?;
  }

  pub resource interface AuctionStoreManager {
    pub fun cancelAuction(auctionID: UInt64);
    pub fun settleAuction(auctionID: UInt64);
  }

  pub resource AuctionStore: AuctionStorePublic, AuctionStoreManager {
    access(self) let auctions: @{UInt64: AuctionItem};

    pub fun createStandaloneAuction(
        token: @AvatarArtNFT.NFT, 
        bidVault: @FungibleToken.Vault,
        startTime: UFix64,
        endTime: UFix64,
        startPrice: UFix64, 
        collectionCap: Capability<&{AvatarArtNFT.CollectionPublic}>, 
        vaultCap: Capability<&{FungibleToken.Receiver}>,
    ) : UInt64 {
        pre {
          AvatarArtTransactionInfo.isCurrencyAccepted(type: bidVault.getType()): "Currency not allow"
        }

        let nftID = token.id;
        let nftType = token.getType();
        let ftType = bidVault.getType();

        // create a new auction items resource container
        let auctionItem  <- create AuctionItem(
            nft: <-token,
            bidVault: <- bidVault,
            startTime: startTime,
            endTime: endTime,
            startPrice: startPrice,
            minimumBidIncrement: AvatarArtAuction.getPriceStep(price: startPrice),
            ownerCollectionCap: collectionCap,
            ownerVaultCap: vaultCap,
            storeID: self.uuid
        )

        let auctionId = auctionItem.uuid;
        let oldAuc <- self.auctions[auctionId] <- auctionItem;
        destroy oldAuc;

        emit AuctionAvailable(
              storeAddress: self.owner?.address!,
              auctionID: auctionId,
              nftType: nftType,
              nftID: nftID,
              ftVaultType: ftType,
              startPrice: startPrice,
              startTime: startTime,
              endTime: endTime
          )

        return auctionId;
    }

    pub fun getAuctionIDs(): [UInt64] {
      return self.auctions.keys;
    }

    pub fun borrowAuction(auctionID: UInt64): &AuctionItem{AuctionPublic}? {
      if self.auctions[auctionID] != nil {
        return  &self.auctions[auctionID] as! &AuctionItem{AuctionPublic};
      } else {
        return nil;
      }
    }

    pub fun cancelAuction(auctionID: UInt64) {
      let auction <- self.auctions.remove(key: auctionID) ?? panic("could not find auction with given id")
      auction.returnAuctionItemToOwner();

      destroy auction;
    }

    pub fun settleAuction(auctionID: UInt64) {
      let auction <- self.auctions.remove(key: auctionID) ?? panic("could not find auction with given id")
      auction.settleAuction();

      destroy auction;
    }

    destroy() {
      destroy  self.auctions;

      emit StoreDestroyed(storeID: self.uuid);
    }

    init() {
      self.auctions <- {};

      emit StoreInitialized(storeID: self.uuid);
    }

}

  pub resource Administrator {
    pub fun setPriceStep(price: UFix64, priceStep: UFix64){
       AvatarArtAuction.priceSteps[price] = priceStep;
    }

    pub fun setFeePreference(
      feeReference: Capability<&AvatarArtTransactionInfo.FeeInfo{AvatarArtTransactionInfo.PublicFeeInfo}>,
      feeRecepientReference: Capability<&AvatarArtTransactionInfo.TransactionAddress{AvatarArtTransactionInfo.PublicTransactionAddress}>
    ) {
      AvatarArtAuction.feeRecepientReference = feeRecepientReference;
      AvatarArtAuction.feeReference = feeReference;
    }

  }

  access(self) fun getPriceStep(price: UFix64): UFix64 {
      var priceStep: UFix64 = 0.0;
      var prevPrice: UFix64 = 0.0;

      for key in AvatarArtAuction.priceSteps.keys{
          if key == price {
              return AvatarArtAuction.priceSteps[key]!;
          } else if key < price && prevPrice < key {
              priceStep = AvatarArtAuction.priceSteps[key]!;
              prevPrice = key;
          }
      }

      return priceStep;
  }

  pub fun createAuctionStore(): @AuctionStore {
      return <-create AuctionStore()
  }

  init() {
    self.totalAuctions = 0;

    self.priceSteps = {};
    self.feeRecepientReference = nil;
    self.feeReference = nil;

    // TODO: Remove suffix
    self.AuctionStoreStoragePath = /storage/avatarArtAuctionStore04;
    self.AuctionStorePublicPath = /public/avatarArtAuctionStore04;
    self.AdminStoragePath = /storage/avatarArtAuctionAdmin04;

    self.account.save(<- create Administrator(), to: self.AdminStoragePath);
  }
}
