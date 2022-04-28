import FungibleToken from 0xf233dcee88fe0abe
import AACommon from 0x39eeb4ee6f30fc3f
import AAReferralManager from 0x39eeb4ee6f30fc3f

import AAPhysical from 0x39eeb4ee6f30fc3f
import AACurrencyManager from 0x39eeb4ee6f30fc3f

pub contract AAFeeManager {
    access(contract) let itemPhysicalCuts: { UInt64: [AACommon.PaymentCut]}

    access(contract) var baseNonPhysicalCuts: [AACommon.PaymentCut]
    access(contract) let basePhysicalCuts: {PhysicalCutFor: [AACommon.PaymentCut]}

    access(contract) let extradata: { String: NFTExtradata }

    access(contract) var platformCut: AACommon.PaymentCut?

    pub var referralRate: UFix64
    pub var affiliateRate: UFix64

    pub let AdminStoragePath: StoragePath

    pub event DigitalFeeSetting(nftID: UInt64, cuts: [AACommon.PaymentCut]);
    pub event PhysicalFeeSetting(nftID: UInt64, cuts: [AACommon.PaymentCut]);
    pub event PlatformCutSetting(recipient: Address, rate: UFix64);
    pub event BaseFeeDigitalSetting(cuts: [AACommon.PaymentCut]);
    pub event BaseFeePhysicalSetting(firstPurchased: Bool, cuts: [AACommon.PaymentCut]);
    pub event ReferralRateSetting(newRate: UFix64)
    pub event AffiliateRateSetting(newRate: UFix64)

    pub enum NFTType: UInt8 {
      pub case Digital
      pub case Physical
    }

    pub enum PhysicalCutFor: UInt8 {
      pub case firstOrder
      pub case Purchased
    }

    pub struct NFTExtradata {
      pub var purchased: Bool
      pub var royalty: AACommon.PaymentCut?
      pub var insurance: Bool

      init() {
          self.purchased = false
          self.royalty = nil
          self.insurance = false
      }

      pub fun markAsPurchased() {
          self.purchased = true
      }

      pub fun setRoyalty(_ r: AACommon.PaymentCut) {
          self.royalty = r
      }

      pub fun setInsurrance(_ b: Bool) {
          self.insurance = b
      }
    }

    pub resource Administrator {
        priv fun assert(cuts: [AACommon.PaymentCut]) {
          var totalRate = 0.0;
          for cut in cuts {
            assert(cut.rate < 1.0, message: "Cut rate must be in range [0..1)")
            totalRate = totalRate + cut.rate;
          }

          assert(totalRate <= 1.0 , message: "Total rate exceed 1.0")
        }

        pub fun setCutForPhysicalItem(nftID: UInt64, cuts: [AACommon.PaymentCut]) {
          self.assert(cuts: cuts)

          AAFeeManager.itemPhysicalCuts[nftID] = cuts;
          emit PhysicalFeeSetting(nftID: nftID, cuts: cuts);
        }

        pub fun setPlatformCut(recipient: Address, rate: UFix64) {
            assert(rate <= 1.0, message: "Cut rate must be in range [0..1)")

            AAFeeManager.platformCut = AACommon.PaymentCut(
              type: "Platform",
              recipient: recipient,
              rate: rate
            )

            self.validatePlatformFee()

            emit PlatformCutSetting(recipient: recipient, rate: rate)
        }

        pub fun setBaseNonPhysicalsCuts(cuts: [AACommon.PaymentCut]) {
            self.assert(cuts: cuts)
            
            AAFeeManager.baseNonPhysicalCuts = cuts;

            emit BaseFeeDigitalSetting(cuts: cuts)
        }

        pub fun setBasePhysicalCuts(forPurchased: Bool, cuts: [AACommon.PaymentCut]) {
          self.assert(cuts: cuts);

          if forPurchased {
            AAFeeManager.basePhysicalCuts[PhysicalCutFor.Purchased] = cuts
          } else {
            AAFeeManager.basePhysicalCuts[PhysicalCutFor.firstOrder] = cuts
          }

          emit BaseFeePhysicalSetting(firstPurchased: forPurchased, cuts: cuts)
        }

        pub fun setAffiliateRate(rate: UFix64) {
            pre {
              rate < 1.0: "Cut rate must be in range [0..1)"
            }

            AAFeeManager.affiliateRate = rate
            self.validatePlatformFee()

            emit AffiliateRateSetting(newRate: rate)
        }

        pub fun setReferralRate(newRate: UFix64) {
            pre {
              newRate < 1.0: "Cut rate must be in range [0..1)"
            }

            AAFeeManager.referralRate = newRate

            self.validatePlatformFee()

            emit ReferralRateSetting(newRate: newRate)
        }

        pub fun configFee(plarformRecipient: Address, platformRate: UFix64, affiliateRate: UFix64, referralRate: UFix64) {
            pre {
              platformRate < 1.0: "Cut rate must be in range [0..1]"
              referralRate < 1.0: "Cut rate must be in range [0..1]"
              affiliateRate < 1.0: "Cut rate must be in range [0..1]"
            }


            AAFeeManager.referralRate = referralRate
            AAFeeManager.affiliateRate = affiliateRate
            AAFeeManager.platformCut = AACommon.PaymentCut(
              type: "Platform",
              recipient: plarformRecipient,
              rate: platformRate
            )

            self.validatePlatformFee()

            emit PlatformCutSetting(recipient: plarformRecipient, rate: platformRate)
            emit ReferralRateSetting(newRate: referralRate)
            emit AffiliateRateSetting(newRate: affiliateRate)
        }

        pub fun validatePlatformFee() {
            if AAFeeManager.platformCut == nil {
              return
            }

            assert(
              AAFeeManager.affiliateRate + AAFeeManager.referralRate < AAFeeManager.platformCut!.rate,
              message:  "Affilate rate + referral rate exceed platform rate"
            )
        }

        pub fun setRoyalty(type: Type, nftID: UInt64, recipient: Address, rate: UFix64) {
            let id = AACommon.itemIdentifier(type: type, id: nftID) 

            if AAFeeManager.extradata[id] == nil {
                AAFeeManager.extradata[id] = NFTExtradata()
            }

            let extradata = AAFeeManager.extradata[id]!

            extradata.setRoyalty(AACommon.PaymentCut(
              type: "Royalty",
              recipient: recipient,
              rate: rate 
            ))

            AAFeeManager.extradata[id] = extradata
        }


        pub fun setInsurrance(type: Type, nftID: UInt64, b: Bool) {
            let id = AACommon.itemIdentifier(type: type, id: nftID) 

            if AAFeeManager.extradata[id] == nil {
                AAFeeManager.extradata[id] = NFTExtradata()
            }

            let extradata = AAFeeManager.extradata[id]!

            extradata.setInsurrance(b)

            AAFeeManager.extradata[id] = extradata
        }
    }

    pub fun getPlatformCut(): AACommon.PaymentCut? {
      return self.platformCut
    }

    pub fun getNonPhysicalsPaymentCuts(type: Type, nftID: UInt64): [AACommon.PaymentCut] {
      let cuts: [AACommon.PaymentCut] = []

      let id = AACommon.itemIdentifier(type: type, id: nftID) 
      let data = AAFeeManager.extradata[id]

      if data?.royalty != nil {
        cuts.append(data!.royalty!)
      }

      cuts.appendAll(AAFeeManager.baseNonPhysicalCuts);
      return cuts;
    }

    pub fun getPhysicalPaymentCuts(nftID: UInt64): [AACommon.PaymentCut] {
      let cuts: [AACommon.PaymentCut] = []

      let id = AACommon.itemIdentifier(type: Type<@AAPhysical.NFT>(), id: nftID)
      let data = AAFeeManager.extradata[id]

      if data?.royalty != nil {
        cuts.append(data!.royalty!)
      }

      // Try get cuts from Setting per item
      if let otherCuts = AAFeeManager.itemPhysicalCuts[nftID] {
        cuts.appendAll(otherCuts);
        return cuts
      }

      // Get from base fee
      let purchased = data?.purchased ?? false
      let cutFor = purchased ? PhysicalCutFor.Purchased : PhysicalCutFor.firstOrder

      for cut in  AAFeeManager.basePhysicalCuts[cutFor] ?? []{
          if data?.insurance ?? false {
              if cut.type == "Insurrance" {
                  continue
              }
          }
          cuts.append(cut) 
      }

      return cuts;
    }

    pub fun getPaymentCuts(type: Type, nftID: UInt64): [AACommon.PaymentCut] {
      if Type<@AAPhysical.NFT>() == type {
        return self.getPhysicalPaymentCuts(nftID: nftID)
      }

      return self.getNonPhysicalsPaymentCuts(type: type, nftID: nftID)
    }

    pub fun getPlatformCuts(referralReceiver: Address?, affiliate: Address?): [AACommon.PaymentCut]? {
        if let platformCut = self.platformCut {
            let cuts: [AACommon.PaymentCut] = []
            var baseRate = platformCut.rate

            if self.affiliateRate > 0.0 && affiliate != nil {
                baseRate = baseRate - self.affiliateRate
                cuts.append(
                  AACommon.PaymentCut(type: "Platform - Affiliate", recipient: affiliate!, rate: self.affiliateRate)
                )    
            }

            if let referralReceiver = referralReceiver {
                baseRate = baseRate - self.referralRate
                cuts.append(AACommon.PaymentCut(type: "Platform - Referral", recipient: referralReceiver, rate: self.referralRate))
            }

            cuts.append(AACommon.PaymentCut(type: "System", recipient: platformCut.recipient, rate: baseRate))
            return cuts
        }

        return nil
    }

    pub fun paidAffiliateFee(paymentType: Type, affiliate: Address?, amount: UFix64): AACommon.Payment? {
        let path = AACurrencyManager.getPath(type: paymentType)
        assert(path != nil, message: "Currency invalid")

        let vaultRef = self.account.borrow<&FungibleToken.Vault>(from: path!.storagePath)
            ?? panic("Can not borrow payment type")
        let recipientAddress = affiliate ?? self.platformCut?.recipient

        if recipientAddress == nil {
          return nil
        }

        if let recipient = getAccount(recipientAddress!).getCapability<&{FungibleToken.Receiver}>(path!.publicPath).borrow() {
            let vault <- vaultRef.withdraw(amount: amount)
            recipient.deposit(from: <- vault)

            return AACommon.Payment(
              type: "Affiliate",
              recipient: recipientAddress!,
              rate: 0.0,
              amount: amount 
            )
        }

        return nil
    }

    access(account) fun markAsPurchased(type: Type, nftID: UInt64) {
        let id = AACommon.itemIdentifier(type: type, id: nftID) 

        if self.extradata[id] == nil {
            self.extradata[id] = NFTExtradata()
        }

        let extradata = self.extradata[id]!
        extradata.markAsPurchased()

        self.extradata[id] = extradata
    }

    init() {
        self.referralRate = 0.002
        self.affiliateRate = 0.028

        self.extradata = {}
        self.itemPhysicalCuts = {}
        self.baseNonPhysicalCuts = []
        self.basePhysicalCuts = {}
        self.platformCut = nil

        self.AdminStoragePath = /storage/AAFeeManagerAdmin
        self.account.save(<- create Administrator(), to: self.AdminStoragePath)
    }
}
