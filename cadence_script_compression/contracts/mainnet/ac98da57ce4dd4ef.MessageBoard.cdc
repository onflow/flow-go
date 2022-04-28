
pub contract MessageBoard {
  // The path to the Admin object in this contract's storage
  pub let adminStoragePath: StoragePath

  pub struct Post {
    pub let timestamp: UFix64
    pub let message: String
    pub let from: Address

    init(timestamp: UFix64, message: String, from: Address) {
      self.timestamp = timestamp
      self.message = message
      self.from = from
    }
  }

  // Records 100 latest messages
  pub var posts: [Post]

  // Emitted when a post is made
  pub event Posted(timestamp: UFix64, message: String, from: Address)

  pub fun post(message: String, from: Address) {
    pre {
      message.length <= 140: "Message too long"
    }

    let post = Post(timestamp: getCurrentBlock().timestamp, message: message, from: from)
    self.posts.append(post)

    // Keeps only the latest 100 messages
    if (self.posts.length > 100) {
      self.posts.removeFirst()
    }

    emit Posted(timestamp: getCurrentBlock().timestamp, message: message, from: from)
  }

  // Check current messages
  pub fun getPosts(): [Post] {
    return self.posts
  }

  pub resource Admin {
      pub fun deletePost(index: UInt64) {
          MessageBoard.posts.remove(at: index)
      }
  }

  init() {
    self.adminStoragePath = /storage/admin
    self.posts = []
    self.account.save(<-create Admin(), to: self.adminStoragePath)
  }
}
