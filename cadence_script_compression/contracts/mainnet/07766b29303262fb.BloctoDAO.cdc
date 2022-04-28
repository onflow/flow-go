import Crypto
import NonFungibleToken from 0x1d7e57aa55817448
import BloctoPass from 0x0f9df91c9121c460
import BloctoTokenStaking from 0x0f9df91c9121c460
import BloctoToken from 0x0f9df91c9121c460

pub contract BloctoDAO {
  access(contract) var topics: [Topic]

  pub let AdminStoragePath: StoragePath;

  pub enum CountStatus: UInt8 {
    pub case invalid
    pub case success
    pub case finished
  }

  // Admin resourse holder can create Proposers
  pub resource Admin {
    pub fun createProposer(): @BloctoDAO.Proposer {
      return <- create Proposer()
    }
  }

  // Proposer resource holder can propose new topics
  pub resource Proposer {
    pub fun addTopic(title: String, description: String, options: [String], startAt: UFix64?, endAt: UFix64?, minVoteStakingAmount: UFix64?) {
      BloctoDAO.topics.append(Topic(
        proposer: self.owner!.address,
        title: title, 
        description: description,
        options: options,
        startAt: startAt,
        endAt: endAt,
        minVoteStakingAmount: minVoteStakingAmount
      ))
    }
    pub fun updateTopic(id: Int, title: String?, description: String?, startAt: UFix64?, endAt: UFix64?, voided: Bool?) {
      pre {
        BloctoDAO.topics[id].proposer == self.owner!.address: "Only original proposer can update"
      }

      BloctoDAO.topics[id].update(
        title: title,
        description: description,
        startAt: startAt,
        endAt: endAt,
        voided: voided
      )
    }
  }

  // Voter resource holder can vote on topics 
  pub resource Voter {
    // voted topic id <-> options index mapping
    access(self) var records: { UInt64: Int }
    
    pub fun vote(topicId: UInt64, optionIndex: Int) {
      pre {
        self.records[topicId] == nil: "Already voted"
        optionIndex < BloctoDAO.topics[topicId].options.length: "Invalid option"
      }
      BloctoDAO.topics[topicId].vote(voterAddr: self.owner!.address, optionIndex: optionIndex)
      self.records[topicId] = optionIndex
    };

    pub fun getVotedOption(topicId: UInt64): Int? {
      return self.records[topicId]
    } 

    init() {
      self.records = {}
    }
  }

  pub struct VoteRecord {
    pub let address: Address
    pub let optionIndex: Int
    pub let amount: UFix64

    init(address: Address, optionIndex: Int, amount: UFix64) {
      self.address = address
      self.optionIndex = optionIndex
      self.amount = amount
    }
  }

  pub struct Topic {
    pub let proposer: Address
    pub var title: String
    pub var description: String
    pub let minVoteStakingAmount: UFix64

    pub var options: [String]
    // address <-> address selected options index mapping
    access(self) var voted: { Address: Int }
    // options index <-> result mapping
    pub var votesCountActual: [UFix64]

    pub let createdAt: UFix64
    pub var updatedAt: UFix64
    pub var startAt: UFix64
    pub var endAt: UFix64

    pub var sealed: Bool

    pub var countIndex: Int

    pub var voided: Bool

    init(proposer: Address, title: String, description: String, options: [String], startAt: UFix64?, endAt: UFix64?, minVoteStakingAmount: UFix64?) {
      pre {
        title.length <= 1000: "New title too long"
        description.length <= 1000: "New description too long"
      }

      self.proposer = proposer
      self.title = title
      self.options = options
      self.description = description
      self.minVoteStakingAmount = minVoteStakingAmount != nil ? minVoteStakingAmount! : 0.0
      self.votesCountActual = []
      for option in options {
        self.votesCountActual.append(0.0)
      }
      self.voted = {}

      self.sealed = false
      self.countIndex = 0

      self.createdAt = getCurrentBlock().timestamp
      self.updatedAt = getCurrentBlock().timestamp

      self.startAt = startAt != nil ? startAt! : getCurrentBlock().timestamp
      self.endAt = endAt != nil ? endAt! : self.createdAt + 86400.0 * 14.0

      self.voided = false
    }

    pub fun update(title: String?, description: String?, startAt: UFix64?, endAt: UFix64?, voided: Bool?) {
      pre {
        title?.length ?? 0 <= 1000: "Title too long"
        description?.length ?? 0 <= 1000: "Description too long"
        voided != true: "Can't update after started"
        getCurrentBlock().timestamp < self.startAt: "Can't update after started"
      }

      self.title = title != nil ? title! : self.title
      self.description = description != nil ? description! : self.description
      self.endAt = endAt != nil ? endAt! : self.endAt
      self.startAt = startAt != nil ? startAt! : self.startAt
      self.voided = voided != nil ? voided! : self.voided
      self.updatedAt = getCurrentBlock().timestamp
    }

    pub fun vote(voterAddr: Address, optionIndex: Int) {
      pre {
        self.isStarted(): "Vote not started"
        !self.isEnded(): "Vote ended"
        self.voted[voterAddr] == nil: "Already voted"
      }

      let voterStaked = BloctoDAO.getStakedBLT(address: voterAddr)

      assert(voterStaked >= self.minVoteStakingAmount, message: "Not eligible")

      self.voted[voterAddr] = optionIndex
    }

    // return if count ended
    pub fun count(size: Int): CountStatus {
      if self.isEnded() == false {
        return CountStatus.invalid
      }
      if self.sealed {
        return CountStatus.finished
      }

      let votedList = self.voted.keys
      var batchEnd = self.countIndex + size
      if batchEnd > votedList.length {
        batchEnd = votedList.length
      }
      while self.countIndex != batchEnd {
        let address = votedList[self.countIndex]
        let voterStaked = BloctoDAO.getStakedBLT(address: address)
        let votedOptionIndex = self.voted[address]!
        self.votesCountActual[votedOptionIndex] = self.votesCountActual[votedOptionIndex] + voterStaked

        self.countIndex = self.countIndex + 1
      }

      self.sealed = self.countIndex == votedList.length

      return CountStatus.success

    }

    pub fun isEnded(): Bool {
      return getCurrentBlock().timestamp >= self.endAt
    }

    pub fun isStarted(): Bool {
      return getCurrentBlock().timestamp >= self.startAt
    }

    pub fun getVotes(page: Int, pageSize: Int?): [VoteRecord] {
      var records: [VoteRecord] = []
      let size = pageSize != nil ? pageSize! : 100
      let addresses = self.voted.keys
      var pageStart = (page - 1) * size
      var pageEnd = pageStart + size
      if pageEnd > addresses.length {
        pageEnd = addresses.length
      }
      while pageStart < pageEnd {
        let address = addresses[pageStart]
        let optionIndex = self.voted[address]!
        let amount = BloctoDAO.getStakedBLT(address: address)
        records.append(VoteRecord(address: address, optionIndex: optionIndex, amount: amount))
        pageStart = pageStart + 1
      }
      return records
    }

    pub fun getTotalVoted(): Int {
      return self.voted.keys.length
    }
  }

  pub fun getStakedBLT(address: Address): UFix64 {
    let collectionRef = getAccount(address).getCapability(/public/bloctoPassCollection)
      .borrow<&{NonFungibleToken.CollectionPublic, BloctoPass.CollectionPublic}>()
      ?? panic("Could not borrow collection public reference")
    let ids = collectionRef.getIDs()
    var amount = 0.0

    for id in ids {
      let bloctoPassRef = collectionRef.borrowBloctoPassPublic(id: id)
      let stakerInfo = bloctoPassRef.getStakingInfo()
      amount = amount + stakerInfo.tokensStaked
    }

    return amount
  }

  pub fun getTopics(): [Topic] {
    return self.topics
  }

  pub fun getTopicsLength(): Int {
    return self.topics.length
  }

  pub fun getTopic(id: UInt64): Topic {
    return self.topics[id]
  }

  pub fun count(topicId: UInt64, maxSize: Int): CountStatus {
    return self.topics[topicId].count(size: maxSize)
  }

  pub fun initVoter(): @BloctoDAO.Voter {
    return <- create Voter()
  }

  init () {
    self.topics = []

    self.AdminStoragePath = /storage/bloctoDAOAdmin
    self.account.save(<-create Admin(), to: self.AdminStoragePath)
  }
}
