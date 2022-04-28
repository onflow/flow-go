import Ashes from 0xe2a89587100c6096
import TopShot from 0x0b2a3299cc857e29

pub contract AshesMessageBoard {

    pub event messagePosted(boardID: UInt64, ashSerial: UInt64, payload: String, encoding: String)

    pub resource interface PublicMessageBoard {
        pub fun getMessages(ashSerials: [UInt64]): {UInt64: Message}
        pub fun getConfig(): BoardConfig
        pub fun publishMesasge(ash: @Ashes.Ash, payload: String, encoding: String): @Ashes.Ash
    }

    pub struct Message {
        pub(set) var ashSerial: UInt64?
        pub(set) var momentID: UInt64?
        pub(set) var momentData: TopShot.MomentData?

        pub(set) var payload: String?
        pub(set) var encoding: String?

        pub(set) var paused: Bool
        pub(set) var sizeLimit: UInt32
        pub(set) var pristine: Bool
        pub(set) var meta: {String:String}

        pub fun checkCanPost(payload: String, encoding: String) {
            // message exists
            if self.paused {
                panic("message paused")
            }

            if (!self.pristine) {
                panic("message not pristine")
            }

            if payload.length + encoding.length > Int(self.sizeLimit) {
                panic("message size too big")
            }
        }

        init(sizeLimit: UInt32) {
            self.sizeLimit = sizeLimit
            self.paused = false
            self.pristine = true
            self.ashSerial = nil
            self.payload = nil
            self.encoding = nil
            self.momentData = nil
            self.momentID = nil
            self.meta = {}
        }
    }


    pub struct BoardConfig {
        pub(set) var defaultMessageSizeLimit: UInt32
        pub(set) var canPostMessage: Bool

        init(defaultMessageSizeLimit: UInt32) {
            self.defaultMessageSizeLimit = defaultMessageSizeLimit
            self.canPostMessage = false
        }
    }

    pub resource Board: PublicMessageBoard {
        priv var messages: {UInt64:Message}
        priv var conf: BoardConfig

        pub fun getMessages(ashSerials: [UInt64]) : {UInt64: Message} {
            let res:{UInt64: Message} = {}
            for ashSerial in ashSerials {
                res[ashSerial] = self.messages[ashSerial]!
            }
            return res
        }

        pub fun getConfig(): BoardConfig {
            return self.conf
        }

        pub fun setConfig(conf: BoardConfig) {
            self.conf = conf
        }

        pub fun publishMesasge(ash: @Ashes.Ash, payload: String, encoding: String): @Ashes.Ash{
            let ashSerial = ash.ashSerial

            let message = self.getOrCreateMessage(ashSerial: ashSerial)

            if !self.conf.canPostMessage {
                panic("posting is closed")
            }

            message.checkCanPost(payload: payload, encoding: encoding)

            message.payload = payload
            message.encoding = encoding
            message.ashSerial = ashSerial
            message.momentID = ash.id
            message.momentData = ash.momentData
            message.pristine = false

            self.messages[ashSerial] = message

            emit messagePosted(boardID: self.uuid, ashSerial: ashSerial, payload: payload, encoding: encoding)
            return <- ash
        }

        pub fun setMessage(ashSerial: UInt64, message: Message) {
            self.messages[ashSerial] = message
        }

        pub fun getOrCreateMessage(ashSerial: UInt64): Message {
           var message = self.messages[ashSerial]
           if message == nil {
               message = Message(sizeLimit: self.conf.defaultMessageSizeLimit)
           }

           return message!
        }

        init() {
            self.messages = {}
            self.conf = BoardConfig(defaultMessageSizeLimit:120)
        }

    }

    init() {
        self.account.save<@Board>(<- create Board(), to: /storage/MessageBoard1)
        self.account.link<&{PublicMessageBoard}>(/public/MessageBoard1, target: /storage/MessageBoard1)
    }

}

