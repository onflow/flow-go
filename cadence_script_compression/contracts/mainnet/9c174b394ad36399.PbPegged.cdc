import Pb from 0x9c174b394ad36399
pub contract PbPegged {
  pub struct Withdraw {
    pub var token: [UInt8]
    pub var receiver: Address
    pub var amount: UFix64
    pub var burnAccount: String
    pub var refChainId: UInt64
    pub var refId: String
    init(_ raw: [UInt8]) {
      self.token = []
      self.receiver = Address(0x0)
      self.amount = 0.0
      self.burnAccount = ""
      self.refChainId = 0
      self.refId = ""
      let buf = Pb.Buffer(raw)
      // todo: write gen-cdc tool
      // what about token? we could save utf8 string but in cadence there is no way to use [UInt8] as string?
      // so we have to just keep [UInt8] and compare to tokenStr.utf8 in withdraw
      while buf.hasMore() {
        let tagType = buf.decKey()
        switch Int(tagType.tag) {
          case 1:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.token = buf.decBytes()
          case 2:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.receiver = Pb.toAddress(buf.decBytes())
          case 3:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.amount = Pb.toUFix64(buf.decBytes())
          case 4:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.burnAccount = Pb.toString(buf.decBytes())
          case 5:
            assert(tagType.wt==Pb.WireType.Varint, message: "wrong wire type")
            self.refChainId = buf.decVarint()
          case 6:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.refId = Pb.toString(buf.decBytes())
          default:
            assert(false, message: "unsupported tag")
        }
      }
    }
    // compare tkStr.utf8 equals self.token
    pub fun eqToken(tkStr:String): Bool {
      let tk = tkStr.utf8
      if tk.length == self.token.length {
        var i = 0;
        while i < tk.length {
          if tk[i] != self.token[i] {
            return false
          }
          i = i + 1
        }
        return true
      }
      return false
    }
  }

  pub struct Mint {
    pub var token: [UInt8]
    pub var receiver: Address
    pub var amount: UFix64
    pub var depositor: String
    pub var refChainId: UInt64
    pub var refId: String
    init(_ raw: [UInt8]) {
      self.token = []
      self.receiver = Address(0x0)
      self.amount = 0.0
      self.depositor = ""
      self.refChainId = 0
      self.refId = ""
      let buf = Pb.Buffer(raw)
      // todo: write gen-cdc tool
      // what about token? we could save utf8 string but in cadence there is no way to use [UInt8] as string?
      // so we have to just keep [UInt8] and compare to tokenStr.utf8 in withdraw
      while buf.hasMore() {
        let tagType = buf.decKey()
        switch Int(tagType.tag) {
          case 1:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.token = buf.decBytes()
          case 2:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.receiver = Pb.toAddress(buf.decBytes())
          case 3:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.amount = Pb.toUFix64(buf.decBytes())
          case 4:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.depositor = Pb.toString(buf.decBytes())
          case 5:
            assert(tagType.wt==Pb.WireType.Varint, message: "wrong wire type")
            self.refChainId = buf.decVarint()
          case 6:
            assert(tagType.wt==Pb.WireType.LengthDelim, message: "wrong wire type")
            self.refId = Pb.toString(buf.decBytes())
          default:
            assert(false, message: "unsupported tag")
        }
      }
    }

    // TODO add a common func, share with withdraw msg
    // compare tkStr.utf8 equals self.token
    pub fun eqToken(tkStr:String): Bool {
      let tk = tkStr.utf8
      if tk.length == self.token.length {
        var i = 0;
        while i < tk.length {
          if tk[i] != self.token[i] {
            return false
          }
          i = i + 1
        }
        return true
      }
      return false
    }
  }
}